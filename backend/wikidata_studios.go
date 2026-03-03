package main

// Wikidata QID enrichment scraper for animation studios.
//
// Reads studio rows from stdin as CSV (exported from PostgreSQL) and resolves
// each entry to a Wikidata QID by name search:
//
//  1. wbsearchentities + MediaWiki full-text search (merged)
//  2. P31 property gate (company/studio types only)
//  3. Exact label match for disambiguation
//
// After QID resolution, Phase 2 fetches extra properties for all resolved
// studios: English/Japanese Wikipedia links, official website, and Twitter handle.
//
// Writes results CSV to stdout:
//   studio_id, wikidata_qid, method,
//   wikipedia_en, wikipedia_ja, website_url, twitter_handle, youtube_channel_id
//
// Writes ambiguous candidates to -ambiguous file for manual review.
//
// Usage:
//
//	psql "$DATABASE_URL" -t -A -F',' \
//	  -c "SELECT id, COALESCE(name,'') FROM studio WHERE wikidata_qid IS NULL ORDER BY id" \
//	  | ./wikidata_studios [flags] > results.csv

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
	wikidataSPARQL = "https://query.wikidata.org/sparql"
	wikidataAPI    = "https://www.wikidata.org/w/api.php"
	userAgent      = "AniGraph-WikidataStudioEnricher/1.0 (anigraph.xyz)"
	sparqlBatch    = 50

	// Decodo proxy pool — same credentials as other scrapers.
	defaultProxyHost  = "dc.decodo.com"
	defaultProxyUser  = "spawee4ylf"
	defaultProxyPass  = "yIzsp7~aeb7Yrz87RQ"
	defaultPortStart  = 10001
	defaultPortEnd    = 10100
	defaultMinSeconds = 3.8
)

// studioP31Set is the set of Wikidata P31 (instance-of) QIDs that qualify a
// Wikidata item as a company, studio, or organisation. Used to filter search
// hits so we don't accidentally match anime titles or characters with the same
// name as a studio.
var studioP31Set = map[string]bool{
	"Q495996":   true, // animation studio (old/variant QID)
	"Q1107679":  true, // animation studio (primary — 790 items)
	"Q2085381":  true, // publishing house
	"Q11396960": true, // production company
	"Q1137655":  true, // company
	"Q4830453":  true, // business
	"Q891723":   true, // public company
	"Q783794":   true, // company (UK QID variant)
	"Q2736883":  true, // studio (film/media)
	"Q375336":   true, // film studio
	"Q17149669": true, // audiovisual production company
	"Q10296194": true, // film and television production company
	"Q1616075":  true, // television studio
	"Q10689397": true, // television production company
	"Q43229":    true, // organization (broad fallback)
	"Q1762059":  true, // film production company (old QID)
	"Q18388277": true, // film production company
	"Q161524":   true, // subsidiary
	"Q658255":   true, // subsidiary company
	"Q167037":   true, // corporation
	"Q15265344": true, // broadcaster
	"Q1194970":  true, // video game developer (old QID)
	"Q210167":   true, // video game developer
	"Q6881511":  true, // enterprise
	"Q1480166":  true, // kabushiki gaisha
}

// ---------------------------------------------------------------------------
// Stats tracker
// ---------------------------------------------------------------------------

type Stats struct {
	mu              sync.Mutex
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	startTime       time.Time
	lastReportTime  time.Time
	lastReportCount int64
}

func newStats() *Stats {
	now := time.Now()
	return &Stats{startTime: now, lastReportTime: now}
}

func (s *Stats) recordSuccess() {
	s.mu.Lock()
	s.totalRequests++
	s.successRequests++
	s.mu.Unlock()
}

func (s *Stats) recordFailure() {
	s.mu.Lock()
	s.totalRequests++
	s.failedRequests++
	s.mu.Unlock()
}

func (s *Stats) report() {
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

var globalStats = newStats()

// ---------------------------------------------------------------------------
// Proxy pool
// ---------------------------------------------------------------------------

type ProxyWorker struct {
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func newProxyWorker(proxyURL string, minInterval time.Duration) *ProxyWorker {
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
	return &ProxyWorker{
		client:      &http.Client{Transport: transport, Timeout: 60 * time.Second},
		minInterval: minInterval,
	}
}

func (pw *ProxyWorker) Do(req *http.Request) (*http.Response, error) {
	pw.mu.Lock()
	if elapsed := time.Since(pw.lastUsed); elapsed < pw.minInterval {
		time.Sleep(pw.minInterval - elapsed)
	}
	pw.lastUsed = time.Now()
	pw.mu.Unlock()
	return pw.client.Do(req)
}

func buildProxies(template string, portStart, portEnd int, minInterval time.Duration) []*ProxyWorker {
	parts := strings.SplitN(template, ":", 4)
	if len(parts) < 4 {
		fmt.Fprintln(os.Stderr, "Proxy format must be host:port:username:password — running without proxies")
		return []*ProxyWorker{newProxyWorker("", minInterval)}
	}
	host, user, pass := parts[0], parts[2], parts[3]
	var workers []*ProxyWorker
	for port := portStart; port <= portEnd; port++ {
		proxyURL := fmt.Sprintf("http://%s:%s@%s:%d", user, pass, host, port)
		workers = append(workers, newProxyWorker(proxyURL, minInterval))
	}
	return workers
}

// ---------------------------------------------------------------------------
// Data types
// ---------------------------------------------------------------------------

type studioRow struct {
	id   string
	name string
}

type studioResult struct {
	id          string
	wikidataQID string
	method      string // "name" | "not_found"
}

type searchHit struct {
	qid   string
	label string
}

type ambiguousEntry struct {
	studio     studioRow
	candidates []searchHit
}

// extraStudioProps holds additional Wikidata-sourced fields fetched in Phase 2.
type extraStudioProps struct {
	wikipediaEn      string // full URL e.g. https://en.wikipedia.org/wiki/Kyoto_Animation
	wikipediaJa      string // full URL e.g. https://ja.wikipedia.org/wiki/…
	websiteURL       string // P856 official website URL
	twitterHandle    string // P2002 Twitter username (without @)
	youtubeChannelID string // P2397 YouTube channel ID
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func sparqlQuery(worker *ProxyWorker, query string) ([]map[string]string, error) {
	const maxAttempts = 6
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			base := time.Duration(1<<uint(attempt)) * time.Second
			jitter := time.Duration(rand.Int63n(int64(base / 2)))
			backoff := base + jitter
			fmt.Fprintf(os.Stderr, "  SPARQL retry %d/%d in %.0fs...\n", attempt+1, maxAttempts, backoff.Seconds())
			time.Sleep(backoff)
		}

		// POST the query as form-encoded body (avoids GET URL length limits / 403s).
		body := url.Values{"query": {query}, "format": {"json"}}.Encode()
		req, err := http.NewRequest("POST", wikidataSPARQL, strings.NewReader(body))
		if err != nil {
			globalStats.recordFailure()
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/sparql-results+json")

		resp, err := worker.Do(req)
		if err != nil {
			globalStats.recordFailure()
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
			globalStats.recordFailure()
			fmt.Fprintf(os.Stderr, "  Rate limited (%d), waiting %ds...\n", resp.StatusCode, wait)
			time.Sleep(time.Duration(wait) * time.Second)
			continue
		}
		if resp.StatusCode == 502 || resp.StatusCode == 504 {
			resp.Body.Close()
			globalStats.recordFailure()
			fmt.Fprintf(os.Stderr, "  HTTP %d (attempt %d), retrying...\n", resp.StatusCode, attempt+1)
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			globalStats.recordFailure()
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
			globalStats.recordFailure()
			return nil, err
		}
		resp.Body.Close()
		globalStats.recordSuccess()

		rows := make([]map[string]string, len(data.Results.Bindings))
		for i, b := range data.Results.Bindings {
			rows[i] = make(map[string]string)
			for k, v := range b {
				rows[i][k] = v.Value
			}
		}
		return rows, nil
	}
	globalStats.recordFailure()
	return nil, fmt.Errorf("SPARQL failed after %d attempts", maxAttempts)
}

// wikidataSearchWB calls wbsearchentities and returns all hits with labels.
func wikidataSearchWB(worker *ProxyWorker, title string) ([]searchHit, error) {
	const maxAttempts = 4

	params := url.Values{
		"action":   {"wbsearchentities"},
		"search":   {title},
		"language": {"en"},
		"limit":    {"10"},
		"format":   {"json"},
		"type":     {"item"},
	}
	apiURL := wikidataAPI + "?" + params.Encode()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
			time.Sleep(backoff + jitter)
		}

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := worker.Do(req)
		if err != nil {
			globalStats.recordFailure()
			continue
		}

		if resp.StatusCode == 429 {
			wait := 120
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if n, _ := fmt.Sscan(ra, &wait); n == 0 {
					wait = 120
				}
			}
			resp.Body.Close()
			globalStats.recordFailure()
			fmt.Fprintf(os.Stderr, "  wbsearch rate limited (429), waiting %ds...\n", wait)
			time.Sleep(time.Duration(wait) * time.Second)
			continue
		}
		if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			resp.Body.Close()
			globalStats.recordFailure()
			fmt.Fprintf(os.Stderr, "  wbsearch HTTP %d (attempt %d/%d), retrying...\n", resp.StatusCode, attempt+1, maxAttempts)
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			globalStats.recordFailure()
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		var data struct {
			Search []struct {
				ID    string `json:"id"`
				Label string `json:"label"`
			} `json:"search"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			globalStats.recordFailure()
			return nil, err
		}
		resp.Body.Close()
		globalStats.recordSuccess()

		var hits []searchHit
		for _, item := range data.Search {
			hits = append(hits, searchHit{item.ID, item.Label})
		}
		return hits, nil
	}
	globalStats.recordFailure()
	return nil, fmt.Errorf("wbsearch failed after %d attempts", maxAttempts)
}

// mediawikiSearch calls action=query&list=search (MediaWiki full-text search).
// Returns hits with QID only — no label (those come from wbsearchentities).
func mediawikiSearch(worker *ProxyWorker, title string) ([]searchHit, error) {
	const maxAttempts = 4

	params := url.Values{
		"action":      {"query"},
		"list":        {"search"},
		"srsearch":    {title},
		"srnamespace": {"0"},
		"srlimit":     {"10"},
		"srprop":      {""},
		"format":      {"json"},
	}
	apiURL := wikidataAPI + "?" + params.Encode()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
			time.Sleep(backoff + jitter)
		}

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", userAgent)

		resp, err := worker.Do(req)
		if err != nil {
			globalStats.recordFailure()
			continue
		}

		if resp.StatusCode == 429 {
			wait := 120
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if n, _ := fmt.Sscan(ra, &wait); n == 0 {
					wait = 120
				}
			}
			resp.Body.Close()
			globalStats.recordFailure()
			fmt.Fprintf(os.Stderr, "  mwsearch rate limited (429), waiting %ds...\n", wait)
			time.Sleep(time.Duration(wait) * time.Second)
			continue
		}
		if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			resp.Body.Close()
			globalStats.recordFailure()
			fmt.Fprintf(os.Stderr, "  mwsearch HTTP %d (attempt %d/%d), retrying...\n", resp.StatusCode, attempt+1, maxAttempts)
			continue
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			globalStats.recordFailure()
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		var data struct {
			Query struct {
				Search []struct {
					Title string `json:"title"`
				} `json:"search"`
			} `json:"query"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			globalStats.recordFailure()
			return nil, err
		}
		resp.Body.Close()
		globalStats.recordSuccess()

		var hits []searchHit
		for _, item := range data.Query.Search {
			if strings.HasPrefix(item.Title, "Q") {
				hits = append(hits, searchHit{qid: item.Title})
			}
		}
		return hits, nil
	}
	globalStats.recordFailure()
	return nil, fmt.Errorf("mwsearch failed after %d attempts", maxAttempts)
}

// wikidataSearchDual fires both search APIs sequentially on the same proxy worker
// and merges results. wbsearchentities results take priority since they carry labels.
func wikidataSearchDual(worker *ProxyWorker, title string) []searchHit {
	hits1, err1 := wikidataSearchWB(worker, title)
	if err1 != nil {
		fmt.Fprintf(os.Stderr, "  wbsearch error for %q: %v\n", title, err1)
	}
	hits2, err2 := mediawikiSearch(worker, title)
	if err2 != nil {
		fmt.Fprintf(os.Stderr, "  mwsearch error for %q: %v\n", title, err2)
	}

	seen := make(map[string]bool)
	var merged []searchHit
	for _, h := range hits1 {
		if !seen[h.qid] {
			seen[h.qid] = true
			merged = append(merged, h)
		}
	}
	for _, h := range hits2 {
		if !seen[h.qid] {
			seen[h.qid] = true
			merged = append(merged, h)
		}
	}
	return merged
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
// Phase 1 — name search pipeline (1a→1b→1c)
// ---------------------------------------------------------------------------

// pendingSearch holds search results for one studio before SPARQL filtering.
type pendingSearch struct {
	studio studioRow
	hits   []searchHit // from wikidataSearchDual
}

// batchFetchP31 fetches P31 types for all given QIDs using batched SPARQL.
// Returns map[QID]→[]P31_QIDs.
func batchFetchP31(qids []string, workers []*ProxyWorker) map[string][]string {
	if len(qids) == 0 {
		return map[string][]string{}
	}

	type batch struct {
		qids  []string
		index int
	}
	var batches []batch
	for i := 0; i < len(qids); i += sparqlBatch {
		end := i + sparqlBatch
		if end > len(qids) {
			end = len(qids)
		}
		batches = append(batches, batch{qids[i:end], i / sparqlBatch})
	}

	batchCh := make(chan batch, len(batches))
	for _, b := range batches {
		batchCh <- b
	}
	close(batchCh)

	var mu sync.Mutex
	out := make(map[string][]string)
	var wg sync.WaitGroup

	for _, w := range workers {
		wg.Add(1)
		go func(worker *ProxyWorker) {
			defer wg.Done()
			for b := range batchCh {
				vals := make([]string, len(b.qids))
				for i, q := range b.qids {
					vals[i] = "wd:" + q
				}
				query := `SELECT ?item ?type WHERE {
  VALUES ?item { ` + strings.Join(vals, " ") + ` }
  ?item wdt:P31 ?type .
}`
				rows, err := sparqlQuery(worker, query)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  P31 batch %d SPARQL error: %v\n", b.index, err)
					continue
				}
				mu.Lock()
				for _, row := range rows {
					item := extractQID(row["item"])
					typ := extractQID(row["type"])
					if item != "" && typ != "" {
						out[item] = append(out[item], typ)
					}
				}
				mu.Unlock()

				if (b.index+1)%50 == 0 {
					fmt.Fprintf(os.Stderr, "  P31-batch: %d/%d done\n", b.index+1, len(batches))
				}
			}
		}(w)
	}
	wg.Wait()
	fmt.Fprintf(os.Stderr, "  P31-batch: all %d batches done (%d unique QIDs)\n", len(batches), len(qids))
	return out
}

// p31FilterHits applies P31 type filtering locally using a pre-fetched p31Map.
// Returns only hits whose P31 types match studioP31Set.
func p31FilterHits(hits []searchHit, p31Map map[string][]string) []searchHit {
	var matched []searchHit
	for _, h := range hits {
		for _, t := range p31Map[h.qid] {
			if studioP31Set[t] {
				matched = append(matched, h)
				break
			}
		}
	}
	return matched
}

// ---------------------------------------------------------------------------
// Phase 2 — fetch extra properties for all resolved QIDs
// ---------------------------------------------------------------------------

func fetchExtraStudioProps(resolved map[string]studioResult, workers []*ProxyWorker) map[string]extraStudioProps {
	type pair struct{ studioID, qid string }
	var pairs []pair
	qidToStudio := make(map[string]string)
	for studioID, res := range resolved {
		if res.wikidataQID == "" {
			continue
		}
		pairs = append(pairs, pair{studioID, res.wikidataQID})
		qidToStudio[res.wikidataQID] = studioID
	}
	if len(pairs) == 0 {
		return map[string]extraStudioProps{}
	}

	type batch struct {
		pairs []pair
		index int
	}
	var batches []batch
	for i := 0; i < len(pairs); i += sparqlBatch {
		end := i + sparqlBatch
		if end > len(pairs) {
			end = len(pairs)
		}
		batches = append(batches, batch{pairs[i:end], i / sparqlBatch})
	}

	batchCh := make(chan batch, len(batches))
	for _, b := range batches {
		batchCh <- b
	}
	close(batchCh)

	var mu sync.Mutex
	out := make(map[string]extraStudioProps)
	var wg sync.WaitGroup

	for _, w := range workers {
		wg.Add(1)
		go func(worker *ProxyWorker) {
			defer wg.Done()
			for b := range batchCh {
				vals := make([]string, len(b.pairs))
				for i, p := range b.pairs {
					vals[i] = "wd:" + p.qid
				}
				query := `SELECT ?item ?enwiki ?jawiki ?website ?twitter ?youtube WHERE {
  VALUES ?item { ` + strings.Join(vals, " ") + ` }
  OPTIONAL { ?enwiki schema:about ?item ; schema:inLanguage "en" ;
                     schema:isPartOf <https://en.wikipedia.org/> . }
  OPTIONAL { ?jawiki schema:about ?item ; schema:inLanguage "ja" ;
                     schema:isPartOf <https://ja.wikipedia.org/> . }
  OPTIONAL { ?item wdt:P856  ?website . }
  OPTIONAL { ?item wdt:P2002 ?twitter . }
  OPTIONAL { ?item wdt:P2397 ?youtube . }
}`

				rows, err := sparqlQuery(worker, query)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  extra-props batch %d SPARQL error: %v\n", b.index, err)
					continue
				}

				mu.Lock()
				for _, row := range rows {
					qid := extractQID(row["item"])
					studioID, ok := qidToStudio[qid]
					if !ok {
						continue
					}
					ep := out[studioID]
					if v := row["enwiki"]; v != "" {
						ep.wikipediaEn = v
					}
					if v := row["jawiki"]; v != "" {
						ep.wikipediaJa = v
					}
					if v := row["website"]; v != "" {
						ep.websiteURL = v
					}
					if v := row["twitter"]; v != "" {
						ep.twitterHandle = v
					}
					if v := row["youtube"]; v != "" {
						ep.youtubeChannelID = v
					}
					out[studioID] = ep
				}
				mu.Unlock()

				if (b.index+1)%10 == 0 {
					fmt.Fprintf(os.Stderr, "  extra-props: batch %d/%d done\n", b.index+1, len(batches))
				}
			}
		}(w)
	}
	wg.Wait()
	return out
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	proxyStr          := flag.String("proxy", "", "Proxy template host:port:user:pass (overrides hardcoded Decodo creds)")
	proxyStart        := flag.Int("proxy-start", defaultPortStart, "First proxy port")
	proxyEnd          := flag.Int("proxy-end", defaultPortEnd, "Last proxy port")
	minInterval       := flag.Float64("min-interval", defaultMinSeconds, "Minimum seconds between requests per proxy")
	workerCount       := flag.Int("workers", defaultPortEnd-defaultPortStart+1, "Concurrent workers for name search phase")
	sparqlWorkerCount := flag.Int("sparql-workers", 8, "Concurrent workers for extra-props SPARQL phase")
	ambigFile         := flag.String("ambiguous", "studio_ambiguous.csv", "File for ambiguous candidates")
	flag.Parse()

	interval := time.Duration(*minInterval * float64(time.Second))

	var allWorkers []*ProxyWorker
	if *proxyStr != "" {
		allWorkers = buildProxies(*proxyStr, *proxyStart, *proxyEnd, interval)
	} else {
		hardcoded := fmt.Sprintf("%s:%d:%s:%s", defaultProxyHost, defaultPortStart, defaultProxyUser, defaultProxyPass)
		allWorkers = buildProxies(hardcoded, *proxyStart, *proxyEnd, interval)
	}
	fmt.Fprintf(os.Stderr, "Using %d proxies (min interval %.1fs each)\n", len(allWorkers), *minInterval)

	nameWorkers := allWorkers
	if *workerCount < len(allWorkers) {
		nameWorkers = allWorkers[:*workerCount]
	}
	sparqlWorkers := allWorkers
	if *sparqlWorkerCount < len(allWorkers) {
		sparqlWorkers = allWorkers[:*sparqlWorkerCount]
	}

	// ---- Read stdin CSV ----
	// Columns: id, name
	r := csv.NewReader(bufio.NewReader(os.Stdin))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var all []studioRow
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "CSV error: %v\n", err)
			continue
		}
		if len(row) < 2 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		s := studioRow{
			id:   strings.TrimSpace(row[0]),
			name: strings.TrimSpace(row[1]),
		}
		if s.name == "" {
			continue
		}
		all = append(all, s)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d studios to resolve\n\n", len(all))

	// Periodic stats ticker.
	statsDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				globalStats.report()
			case <-statsDone:
				return
			}
		}
	}()
	defer close(statsDone)

	resolved := make(map[string]studioResult)

	// ---- Phase 1a: Name search (API only, no SPARQL) ----
	fmt.Fprintf(os.Stderr, "Phase 1a: Name search (dual-API) — %d workers, %d studios...\n", len(nameWorkers), len(all))

	type indexedPending struct {
		idx int
		ps  pendingSearch
	}
	jobs := make(chan indexedPending, len(all))
	for i, s := range all {
		jobs <- indexedPending{i, pendingSearch{studio: s}}
	}
	close(jobs)

	pending := make([]pendingSearch, len(all))
	var wg sync.WaitGroup

	for i, w := range nameWorkers {
		if i >= len(all) {
			break
		}
		wg.Add(1)
		go func(worker *ProxyWorker) {
			defer wg.Done()
			for job := range jobs {
				s := job.ps.studio
				if s.name != "" {
					job.ps.hits = wikidataSearchDual(worker, s.name)
				}
				pending[job.idx] = job.ps
			}
		}(w)
	}
	wg.Wait()
	fmt.Fprintf(os.Stderr, "  -> Phase 1a done: %d studios searched\n\n", len(all))

	// ---- Phase 1b: Batch P31 filter ----
	// Collect all unique candidate QIDs.
	uniqueQIDs := make(map[string]bool)
	for _, ps := range pending {
		for _, h := range ps.hits {
			uniqueQIDs[h.qid] = true
		}
	}
	allQIDs := make([]string, 0, len(uniqueQIDs))
	for q := range uniqueQIDs {
		allQIDs = append(allQIDs, q)
	}
	fmt.Fprintf(os.Stderr, "Phase 1b: Batch P31 filter — %d unique QIDs, %d workers...\n", len(allQIDs), len(sparqlWorkers))

	p31Map := batchFetchP31(allQIDs, sparqlWorkers)

	// Apply P31 filter locally per studio + exact label disambiguation.
	var mu sync.Mutex
	var ambiguous []ambiguousEntry
	phase1found, phase1ambiguous := 0, 0

	for _, ps := range pending {
		s := ps.studio
		filtered := p31FilterHits(ps.hits, p31Map)
		if len(filtered) == 0 {
			continue // not found
		}
		if len(filtered) == 1 {
			resolved[s.id] = studioResult{s.id, filtered[0].qid, "name"}
			phase1found++
			continue
		}

		// ---- Phase 1c: Exact label match tiebreaker (pure local, no network) ----
		nameLower := strings.ToLower(strings.TrimSpace(s.name))
		var exact []searchHit
		for _, h := range filtered {
			if strings.ToLower(strings.TrimSpace(h.label)) == nameLower {
				exact = append(exact, h)
			}
		}
		if len(exact) == 1 {
			resolved[s.id] = studioResult{s.id, exact[0].qid, "name"}
			phase1found++
		} else if len(exact) > 1 {
			// Multiple items share the exact name — return only those for review.
			mu.Lock()
			ambiguous = append(ambiguous, ambiguousEntry{s, exact})
			phase1ambiguous++
			mu.Unlock()
		} else {
			// No exact label match — return all P31-valid candidates for review.
			mu.Lock()
			ambiguous = append(ambiguous, ambiguousEntry{s, filtered})
			phase1ambiguous++
			mu.Unlock()
		}
	}
	fmt.Fprintf(os.Stderr, "  -> Phase 1: %d found, %d ambiguous (-> %s)\n\n", phase1found, phase1ambiguous, *ambigFile)

	// ---- Phase 2: Extra properties ----
	fmt.Fprintf(os.Stderr, "Phase 2: Extra properties — %d resolved studios, %d workers...\n", len(resolved), len(sparqlWorkers))
	extraMap := fetchExtraStudioProps(resolved, sparqlWorkers)
	fmt.Fprintf(os.Stderr, "  -> extra-props fetched for %d studios\n\n", len(extraMap))

	// ---- Summary ----
	notFound := len(all) - len(resolved) - len(ambiguous)
	fmt.Fprintf(os.Stderr, "Done: %d resolved, %d ambiguous, %d not found (out of %d)\n",
		len(resolved), len(ambiguous), notFound, len(all))

	// ---- Write ambiguous review CSV ----
	if len(ambiguous) > 0 {
		af, err := os.Create(*ambigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create %s: %v\n", *ambigFile, err)
		} else {
			aw := csv.NewWriter(af)
			aw.Write([]string{"studio_id", "studio_name", "candidate_qid", "candidate_label"})
			totalRows := 0
			for _, e := range ambiguous {
				for _, c := range e.candidates {
					aw.Write([]string{e.studio.id, e.studio.name, c.qid, c.label})
					totalRows++
				}
			}
			aw.Flush()
			af.Close()
			fmt.Fprintf(os.Stderr, "Wrote %d ambiguous studios (%d candidate rows) to %s\n",
				len(ambiguous), totalRows, *ambigFile)
		}
	}

	// ---- Write main results CSV ----
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{
		"studio_id", "wikidata_qid", "method",
		"wikipedia_en", "wikipedia_ja", "website_url", "twitter_handle", "youtube_channel_id",
	})
	for _, s := range all {
		ep := extraMap[s.id]
		if res, ok := resolved[s.id]; ok {
			w.Write([]string{
				res.id, res.wikidataQID, res.method,
				ep.wikipediaEn, ep.wikipediaJa, ep.websiteURL, ep.twitterHandle, ep.youtubeChannelID,
			})
		} else {
			w.Write([]string{s.id, "", "not_found", "", "", "", "", ""})
		}
	}
	w.Flush()
}
