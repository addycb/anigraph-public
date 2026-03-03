package main

// Wikidata extra-properties backfill scraper.
//
// Reads anime rows from stdin as CSV with columns: anilist_id, wikidata_qid
// and fetches extra properties (Wikipedia links, livechart, notify, TheTVDB,
// TMDB, TVmaze, MyWaifuList, Unconsenting Media) for each QID via batched
// SPARQL queries.
//
// This is a lightweight alternative to re-running the full wikidata_qid
// pipeline — it skips QID resolution entirely and only does the Phase 4
// extra-props fetch.
//
// Writes results CSV to stdout:
//   anilist_id, wikidata_qid, wikipedia_en, wikipedia_ja,
//   livechart_id, notify_id, tvdb_id, tmdb_movie_id, tmdb_tv_id,
//   tvmaze_id, mywaifulist_id, unconsenting_media_id
//
// Usage:
//
//	psql "$DATABASE_URL" -t -A -F',' \
//	  -c "SELECT anilist_id, wikidata_qid FROM anime
//	      WHERE wikidata_qid IS NOT NULL AND wikipedia_en IS NULL
//	      ORDER BY anilist_id" \
//	  | ./wikidata_backfill_props [flags] > results.csv

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	sparqlEndpoint = "https://query.wikidata.org/sparql"
	bpUserAgent    = "AniGraph-WikidataBackfill/1.0 (anigraph.xyz)"
	bpSparqlBatch  = 50

	// Decodo proxy pool — same credentials as scrape.go / wikidata_qid.go.
	bpDefaultProxyHost = "dc.decodo.com"
	bpDefaultProxyUser = "spawee4ylf"
	bpDefaultProxyPass = "yIzsp7~aeb7Yrz87RQ"
	bpDefaultPortStart = 10001
	bpDefaultPortEnd   = 10100
	bpDefaultMinSecs   = 3.8
)

// ---------------------------------------------------------------------------
// Stats tracker
// ---------------------------------------------------------------------------

type bpStats struct {
	mu              sync.Mutex
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	startTime       time.Time
	lastReportTime  time.Time
	lastReportCount int64
}

func newBpStats() *bpStats {
	now := time.Now()
	return &bpStats{startTime: now, lastReportTime: now}
}

func (s *bpStats) recordSuccess() {
	s.mu.Lock()
	s.totalRequests++
	s.successRequests++
	s.mu.Unlock()
}

func (s *bpStats) recordFailure() {
	s.mu.Lock()
	s.totalRequests++
	s.failedRequests++
	s.mu.Unlock()
}

func (s *bpStats) report() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(s.startTime).Seconds()
	intervalReqs := s.totalRequests - s.lastReportCount
	intervalSecs := now.Sub(s.lastReportTime).Seconds()
	rps := 0.0
	if intervalSecs > 0 {
		rps = float64(intervalReqs) / intervalSecs
	}
	s.lastReportTime = now
	s.lastReportCount = s.totalRequests
	fmt.Fprintf(os.Stderr, "[stats] total=%d ok=%d fail=%d elapsed=%.0fs rps=%.1f\n",
		s.totalRequests, s.successRequests, s.failedRequests, elapsed, rps)
}

var stats = newBpStats()

// ---------------------------------------------------------------------------
// Proxy pool
// ---------------------------------------------------------------------------

type proxyWorker struct {
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func newProxy(proxyURL string, minInterval time.Duration) *proxyWorker {
	var transport *http.Transport
	if proxyURL != "" {
		proxy, _ := url.Parse(proxyURL)
		transport = &http.Transport{
			Proxy:               http.ProxyURL(proxy),
			MaxIdleConnsPerHost: 10,
		}
	} else {
		transport = &http.Transport{MaxIdleConnsPerHost: 10}
	}
	return &proxyWorker{
		client:      &http.Client{Transport: transport, Timeout: 60 * time.Second},
		minInterval: minInterval,
	}
}

func (pw *proxyWorker) Do(req *http.Request) (*http.Response, error) {
	pw.mu.Lock()
	if elapsed := time.Since(pw.lastUsed); elapsed < pw.minInterval {
		time.Sleep(pw.minInterval - elapsed)
	}
	pw.lastUsed = time.Now()
	pw.mu.Unlock()
	return pw.client.Do(req)
}

func buildProxyPool(template string, portStart, portEnd int, minInterval time.Duration) []*proxyWorker {
	parts := strings.SplitN(template, ":", 4)
	if len(parts) < 4 {
		fmt.Fprintln(os.Stderr, "Proxy format must be host:port:username:password — running without proxies")
		return []*proxyWorker{newProxy("", minInterval)}
	}
	host, user, pass := parts[0], parts[2], parts[3]
	var workers []*proxyWorker
	for port := portStart; port <= portEnd; port++ {
		proxyURL := fmt.Sprintf("http://%s:%s@%s:%d", user, pass, host, port)
		workers = append(workers, newProxy(proxyURL, minInterval))
	}
	return workers
}

// ---------------------------------------------------------------------------
// SPARQL helper
// ---------------------------------------------------------------------------

func bpSparqlQuery(worker *proxyWorker, query string) ([]map[string]string, error) {
	const maxAttempts = 6
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			base := time.Duration(1<<uint(attempt)) * time.Second
			jitter := time.Duration(rand.Int63n(int64(base / 2)))
			backoff := base + jitter
			fmt.Fprintf(os.Stderr, "  SPARQL retry %d/%d in %.0fs...\n", attempt+1, maxAttempts, backoff.Seconds())
			time.Sleep(backoff)
		}

		body := url.Values{"query": {query}, "format": {"json"}}.Encode()
		req, err := http.NewRequest("POST", sparqlEndpoint, strings.NewReader(body))
		if err != nil {
			stats.recordFailure()
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", bpUserAgent)
		req.Header.Set("Accept", "application/sparql-results+json")

		resp, err := worker.Do(req)
		if err != nil {
			stats.recordFailure()
			fmt.Fprintf(os.Stderr, "  SPARQL request error (attempt %d): %v\n", attempt+1, err)
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode == 503 {
			wait := 120
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if n, err := fmt.Sscan(ra, &wait); n == 0 || err != nil {
					wait = 120
				}
			}
			resp.Body.Close()
			stats.recordFailure()
			fmt.Fprintf(os.Stderr, "  Rate limited (%d), waiting %ds...\n", resp.StatusCode, wait)
			time.Sleep(time.Duration(wait) * time.Second)
			continue
		}
		if resp.StatusCode == 502 || resp.StatusCode == 504 {
			resp.Body.Close()
			stats.recordFailure()
			fmt.Fprintf(os.Stderr, "  HTTP %d (attempt %d), retrying...\n", resp.StatusCode, attempt+1)
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			stats.recordFailure()
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		var data struct {
			Results struct {
				Bindings []map[string]struct {
					Value string `json:"value"`
				} `json:"bindings"`
			} `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			stats.recordFailure()
			return nil, err
		}
		resp.Body.Close()
		stats.recordSuccess()

		rows := make([]map[string]string, len(data.Results.Bindings))
		for i, b := range data.Results.Bindings {
			rows[i] = make(map[string]string)
			for k, v := range b {
				rows[i][k] = v.Value
			}
		}
		return rows, nil
	}
	stats.recordFailure()
	return nil, fmt.Errorf("SPARQL failed after %d attempts", maxAttempts)
}

// ---------------------------------------------------------------------------
// QID helper
// ---------------------------------------------------------------------------

func bpExtractQID(uri string) string {
	if idx := strings.LastIndex(uri, "/"); idx >= 0 {
		s := uri[idx+1:]
		if strings.HasPrefix(s, "Q") {
			return s
		}
	}
	if strings.HasPrefix(uri, "Q") {
		return uri
	}
	return ""
}

// ---------------------------------------------------------------------------
// Extra props types
// ---------------------------------------------------------------------------

type bpExtraProps struct {
	wikipediaEn    string
	wikipediaJa    string
	livechartID    string
	notifyID       string
	tvdbID         string
	tmdbMovieID    string
	tmdbTvID       string
	tvmazeID       string
	mywaifulistID  string
	unconsentingID string
}

// ---------------------------------------------------------------------------
// Fetch extra properties via batched SPARQL
// ---------------------------------------------------------------------------

func fetchAllExtraProps(pairs []inputRow, workers []*proxyWorker) map[string]bpExtraProps {
	if len(pairs) == 0 {
		return map[string]bpExtraProps{}
	}

	// Build reverse lookup: QID → anilistID
	qidToAnilist := make(map[string]string)
	for _, p := range pairs {
		qidToAnilist[p.qid] = p.anilistID
	}

	type batch struct {
		items []inputRow
		index int
	}
	var batches []batch
	for i := 0; i < len(pairs); i += bpSparqlBatch {
		end := i + bpSparqlBatch
		if end > len(pairs) {
			end = len(pairs)
		}
		batches = append(batches, batch{pairs[i:end], i / bpSparqlBatch})
	}

	batchCh := make(chan batch, len(batches))
	for _, b := range batches {
		batchCh <- b
	}
	close(batchCh)

	var mu sync.Mutex
	out := make(map[string]bpExtraProps)
	var wg sync.WaitGroup

	for _, w := range workers {
		wg.Add(1)
		go func(worker *proxyWorker) {
			defer wg.Done()
			for b := range batchCh {
				vals := make([]string, len(b.items))
				for i, p := range b.items {
					vals[i] = "wd:" + p.qid
				}
				query := `SELECT ?item
  ?enwiki ?jawiki
  ?livechart ?notify ?tvdb ?tmdb_movie ?tmdb_tv ?tvmaze ?mwl ?unconsenting
WHERE {
  VALUES ?item { ` + strings.Join(vals, " ") + ` }
  OPTIONAL { ?enwiki schema:about ?item ; schema:inLanguage "en" ;
                     schema:isPartOf <https://en.wikipedia.org/> . }
  OPTIONAL { ?jawiki schema:about ?item ; schema:inLanguage "ja" ;
                     schema:isPartOf <https://ja.wikipedia.org/> . }
  OPTIONAL { ?item wdt:P12489 ?livechart . }
  OPTIONAL { ?item wdt:P12427 ?notify . }
  OPTIONAL { ?item wdt:P4835  ?tvdb . }
  OPTIONAL { ?item wdt:P4947  ?tmdb_movie . }
  OPTIONAL { ?item wdt:P4983  ?tmdb_tv . }
  OPTIONAL { ?item wdt:P8600  ?tvmaze . }
  OPTIONAL { ?item wdt:P14051 ?mwl . }
  OPTIONAL { ?item wdt:P9821  ?unconsenting . }
}`

				rows, err := bpSparqlQuery(worker, query)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  extra-props batch %d/%d SPARQL error: %v\n", b.index+1, len(batches), err)
					continue
				}

				mu.Lock()
				for _, row := range rows {
					qid := bpExtractQID(row["item"])
					anilistID, ok := qidToAnilist[qid]
					if !ok {
						continue
					}
					ep := out[anilistID]
					if v := row["enwiki"]; v != "" {
						ep.wikipediaEn = v
					}
					if v := row["jawiki"]; v != "" {
						ep.wikipediaJa = v
					}
					if v := row["livechart"]; v != "" {
						ep.livechartID = v
					}
					if v := row["notify"]; v != "" {
						ep.notifyID = v
					}
					if v := row["tvdb"]; v != "" {
						ep.tvdbID = v
					}
					if v := row["tmdb_movie"]; v != "" {
						ep.tmdbMovieID = v
					}
					if v := row["tmdb_tv"]; v != "" {
						ep.tmdbTvID = v
					}
					if v := row["tvmaze"]; v != "" {
						ep.tvmazeID = v
					}
					if v := row["mwl"]; v != "" {
						ep.mywaifulistID = v
					}
					if v := row["unconsenting"]; v != "" {
						ep.unconsentingID = v
					}
					out[anilistID] = ep
				}
				mu.Unlock()

				fmt.Fprintf(os.Stderr, "  extra-props: batch %d/%d done (%d results)\n", b.index+1, len(batches), len(rows))
			}
		}(w)
	}
	wg.Wait()
	return out
}

// ---------------------------------------------------------------------------
// Input row
// ---------------------------------------------------------------------------

type inputRow struct {
	anilistID string
	qid       string
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	sparqlWorkerCount := flag.Int("sparql-workers", 8, "Concurrent workers for SPARQL batch queries")
	minInterval := flag.Float64("min-interval", bpDefaultMinSecs, "Minimum seconds between requests per proxy")
	noProxy := flag.Bool("no-proxy", false, "Run without proxies (direct connections)")
	flag.Parse()

	interval := time.Duration(*minInterval * float64(time.Second))

	var workers []*proxyWorker
	if *noProxy {
		fmt.Fprintln(os.Stderr, "Running without proxies (direct connection)")
		workers = []*proxyWorker{newProxy("", interval)}
	} else {
		hardcoded := fmt.Sprintf("%s:%d:%s:%s", bpDefaultProxyHost, bpDefaultPortStart, bpDefaultProxyUser, bpDefaultProxyPass)
		workers = buildProxyPool(hardcoded, bpDefaultPortStart, bpDefaultPortEnd, interval)
	}
	fmt.Fprintf(os.Stderr, "Using %d proxies (min interval %.1fs each)\n", len(workers), *minInterval)

	// Limit to requested worker count.
	if *sparqlWorkerCount < len(workers) {
		workers = workers[:*sparqlWorkerCount]
	}
	fmt.Fprintf(os.Stderr, "Using %d SPARQL workers\n", len(workers))

	// ---- Read stdin CSV ----
	// Columns: anilist_id, wikidata_qid
	r := csv.NewReader(bufio.NewReader(os.Stdin))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var all []inputRow
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "CSV error: %v\n", err)
			continue
		}
		if len(row) < 2 {
			continue
		}
		anilistID := strings.TrimSpace(row[0])
		qid := strings.TrimSpace(row[1])
		if anilistID == "" || qid == "" {
			continue
		}
		all = append(all, inputRow{anilistID: anilistID, qid: qid})
	}
	fmt.Fprintf(os.Stderr, "Loaded %d anime with QIDs to backfill\n\n", len(all))

	if len(all) == 0 {
		fmt.Fprintln(os.Stderr, "Nothing to do.")
		os.Exit(0)
	}

	// Periodic stats ticker.
	statsDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats.report()
			case <-statsDone:
				return
			}
		}
	}()
	defer close(statsDone)

	// ---- Fetch extra properties ----
	fmt.Fprintf(os.Stderr, "Fetching extra properties for %d QIDs in batches of %d...\n", len(all), bpSparqlBatch)
	extraMap := fetchAllExtraProps(all, workers)
	fmt.Fprintf(os.Stderr, "\nDone: fetched extra props for %d / %d entries\n", len(extraMap), len(all))
	stats.report()

	// ---- Write output CSV ----
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{
		"anilist_id", "wikidata_qid",
		"wikipedia_en", "wikipedia_ja",
		"livechart_id", "notify_id", "tvdb_id", "tmdb_movie_id", "tmdb_tv_id",
		"tvmaze_id", "mywaifulist_id", "unconsenting_media_id",
	})
	for _, row := range all {
		ep := extraMap[row.anilistID]
		w.Write([]string{
			row.anilistID, row.qid,
			ep.wikipediaEn, ep.wikipediaJa,
			ep.livechartID, ep.notifyID, ep.tvdbID, ep.tmdbMovieID, ep.tmdbTvID,
			ep.tvmazeID, ep.mywaifulistID, ep.unconsentingID,
		})
	}
	w.Flush()
}
