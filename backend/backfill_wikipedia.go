package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
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
	bfSPARQL    = "https://query.wikidata.org/sparql"
	bfUserAgent = "AniGraph-WikidataEnricher/1.0 (anigraph.xyz)"
	bfBatch     = 50

	bfProxyHost  = "dc.decodo.com"
	bfProxyUser  = "spawee4ylf"
	bfProxyPass  = "yIzsp7~aeb7Yrz87RQ"
	bfPortStart  = 10001
	bfPortEnd    = 10100
	bfMinSeconds = 3.8
)

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

type bfStats struct {
	mu      sync.Mutex
	total   int64
	success int64
	failed  int64
	start   time.Time
}

func newBFStats() *bfStats { return &bfStats{start: time.Now()} }

func (s *bfStats) ok()   { s.mu.Lock(); s.total++; s.success++; s.mu.Unlock() }
func (s *bfStats) fail() { s.mu.Lock(); s.total++; s.failed++; s.mu.Unlock() }
func (s *bfStats) report() {
	s.mu.Lock()
	defer s.mu.Unlock()
	elapsed := time.Since(s.start).Seconds()
	rps := 0.0
	if elapsed > 0 {
		rps = float64(s.total) / elapsed
	}
	fmt.Fprintf(os.Stderr, "[stats] total=%d ok=%d fail=%d elapsed=%.0fs rps=%.1f\n",
		s.total, s.success, s.failed, elapsed, rps)
}

var stats = newBFStats()

// ---------------------------------------------------------------------------
// Proxy worker
// ---------------------------------------------------------------------------

type worker struct {
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func newWorker(proxyURL string, interval time.Duration) *worker {
	proxy, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxy),
		MaxIdleConnsPerHost: 10,
	}
	return &worker{
		client:      &http.Client{Transport: transport, Timeout: 60 * time.Second},
		minInterval: interval,
	}
}

func (w *worker) Do(req *http.Request) (*http.Response, error) {
	w.mu.Lock()
	if elapsed := time.Since(w.lastUsed); elapsed < w.minInterval {
		time.Sleep(w.minInterval - elapsed)
	}
	w.lastUsed = time.Now()
	w.mu.Unlock()
	return w.client.Do(req)
}

func buildWorkers(portStart, portEnd int, interval time.Duration) []*worker {
	var workers []*worker
	for port := portStart; port <= portEnd; port++ {
		proxyURL := fmt.Sprintf("http://%s:%s@%s:%d", bfProxyUser, bfProxyPass, bfProxyHost, port)
		workers = append(workers, newWorker(proxyURL, interval))
	}
	return workers
}

// ---------------------------------------------------------------------------
// SPARQL helper
// ---------------------------------------------------------------------------

func sparql(w *worker, query string) ([]map[string]string, error) {
	const maxAttempts = 6
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			base := time.Duration(1<<uint(attempt)) * time.Second
			jitter := time.Duration(rand.Int63n(int64(base / 2)))
			time.Sleep(base + jitter)
		}

		body := url.Values{"query": {query}, "format": {"json"}}.Encode()
		req, err := http.NewRequest("POST", bfSPARQL, strings.NewReader(body))
		if err != nil {
			stats.fail()
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", bfUserAgent)
		req.Header.Set("Accept", "application/sparql-results+json")

		resp, err := w.Do(req)
		if err != nil {
			stats.fail()
			fmt.Fprintf(os.Stderr, "  SPARQL error (attempt %d): %v\n", attempt+1, err)
			continue
		}

		if resp.StatusCode == 429 || resp.StatusCode == 503 {
			wait := 120
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if n, _ := fmt.Sscan(ra, &wait); n == 0 {
					wait = 120
				}
			}
			resp.Body.Close()
			stats.fail()
			fmt.Fprintf(os.Stderr, "  Rate limited (%d), waiting %ds...\n", resp.StatusCode, wait)
			time.Sleep(time.Duration(wait) * time.Second)
			continue
		}
		if resp.StatusCode == 502 || resp.StatusCode == 504 {
			resp.Body.Close()
			stats.fail()
			fmt.Fprintf(os.Stderr, "  HTTP %d (attempt %d), retrying...\n", resp.StatusCode, attempt+1)
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			stats.fail()
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
			stats.fail()
			return nil, err
		}
		resp.Body.Close()
		stats.ok()

		rows := make([]map[string]string, len(data.Results.Bindings))
		for i, b := range data.Results.Bindings {
			rows[i] = make(map[string]string)
			for k, v := range b {
				rows[i][k] = v.Value
			}
		}
		return rows, nil
	}
	stats.fail()
	return nil, fmt.Errorf("SPARQL failed after %d attempts", maxAttempts)
}

func extractQID(uri string) string {
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
// Main
// ---------------------------------------------------------------------------

func main() {
	interval := time.Duration(bfMinSeconds * float64(time.Second))
	workers := buildWorkers(bfPortStart, bfPortEnd, interval)
	fmt.Fprintf(os.Stderr, "Using %d proxies (min interval %.1fs each)\n", len(workers), bfMinSeconds)

	// Read stdin CSV: anilist_id,wikidata_qid
	r := csv.NewReader(bufio.NewReader(os.Stdin))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	type inputRow struct {
		anilistID string
		qid       string
	}
	var all []inputRow
	// QID → []anilistIDs (many-to-one)
	qidToAnilist := make(map[string][]string)

	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "CSV error: %v\n", err)
			continue
		}
		if len(row) < 2 || strings.TrimSpace(row[0]) == "" || strings.TrimSpace(row[1]) == "" {
			continue
		}
		aid := strings.TrimSpace(row[0])
		qid := strings.TrimSpace(row[1])
		all = append(all, inputRow{aid, qid})
		qidToAnilist[qid] = append(qidToAnilist[qid], aid)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d anime (%d unique QIDs) to backfill\n", len(all), len(qidToAnilist))

	if len(qidToAnilist) == 0 {
		fmt.Fprintf(os.Stderr, "Nothing to do.\n")
		return
	}

	// Collect unique QIDs
	uniqueQIDs := make([]string, 0, len(qidToAnilist))
	for qid := range qidToAnilist {
		uniqueQIDs = append(uniqueQIDs, qid)
	}

	// Batch SPARQL for Wikipedia sitelinks
	type batch struct {
		qids  []string
		index int
	}
	var batches []batch
	for i := 0; i < len(uniqueQIDs); i += bfBatch {
		end := i + bfBatch
		if end > len(uniqueQIDs) {
			end = len(uniqueQIDs)
		}
		batches = append(batches, batch{uniqueQIDs[i:end], i / bfBatch})
	}

	batchCh := make(chan batch, len(batches))
	for _, b := range batches {
		batchCh <- b
	}
	close(batchCh)

	// Results: QID → {en, ja}
	type wikiURLs struct{ en, ja string }
	var mu sync.Mutex
	results := make(map[string]wikiURLs)
	var wg sync.WaitGroup

	// Stats ticker
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

	workerCount := len(workers)
	if workerCount > 100 {
		workerCount = 100
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(w *worker) {
			defer wg.Done()
			for b := range batchCh {
				vals := make([]string, len(b.qids))
				for i, q := range b.qids {
					vals[i] = "wd:" + q
				}
				query := `SELECT ?item ?enwiki ?jawiki WHERE {
  VALUES ?item { ` + strings.Join(vals, " ") + ` }
  OPTIONAL { ?enwiki schema:about ?item ; schema:inLanguage "en" ;
                     schema:isPartOf <https://en.wikipedia.org/> . }
  OPTIONAL { ?jawiki schema:about ?item ; schema:inLanguage "ja" ;
                     schema:isPartOf <https://ja.wikipedia.org/> . }
}`
				rows, err := sparql(w, query)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  batch %d SPARQL error: %v\n", b.index, err)
					continue
				}

				mu.Lock()
				for _, row := range rows {
					qid := extractQID(row["item"])
					if qid == "" {
						continue
					}
					cur := results[qid]
					if v := row["enwiki"]; v != "" {
						cur.en = v
					}
					if v := row["jawiki"]; v != "" {
						cur.ja = v
					}
					results[qid] = cur
				}
				mu.Unlock()

				if (b.index+1)%50 == 0 || b.index+1 == len(batches) {
					fmt.Fprintf(os.Stderr, "  batch %d/%d done\n", b.index+1, len(batches))
				}
			}
		}(workers[i%len(workers)])
	}
	wg.Wait()
	close(statsDone)
	stats.report()

	// Write results CSV to stdout: anilist_id,wikipedia_en,wikipedia_ja
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"anilist_id", "wikipedia_en", "wikipedia_ja"})
	written := 0
	for _, row := range all {
		urls, ok := results[row.qid]
		if !ok || (urls.en == "" && urls.ja == "") {
			continue
		}
		w.Write([]string{row.anilistID, urls.en, urls.ja})
		written++
	}
	w.Flush()
	fmt.Fprintf(os.Stderr, "Done: %d anime with Wikipedia URLs (out of %d)\n", written, len(all))
}
