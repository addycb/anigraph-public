package main

// Wikidata QID enrichment scraper.
//
// Reads anime rows from stdin as CSV (exported from PostgreSQL) and resolves
// each entry to a Wikidata QID using three methods in priority order:
//
//  1. MAL ID     → batch SPARQL via property P4086
//  2. AniList ID → batch SPARQL via property P8729
//  3. Title search → wbsearchentities + MediaWiki full-text search (merged),
//                    then P31 property gate (format-specific)
//
// After QID resolution, Phase 4 fetches extra properties for all resolved
// entries: English/Japanese Wikipedia links, livechart, notify, TheTVDB,
// TMDB (movie + TV), TVmaze, MyWaifuList, and Unconsenting Media IDs.
//
// Writes results CSV to stdout:
//   anilist_id, wikidata_qid, method,
//   wikipedia_en, wikipedia_ja,
//   livechart_id, notify_id, tvdb_id, tmdb_movie_id, tmdb_tv_id,
//   tvmaze_id, mywaifulist_id, unconsenting_media_id
//
// Writes ambiguous candidates to -ambiguous file for manual review.
//
// Usage:
//
//	psql "$DATABASE_URL" -t -A -F',' \
//	  -c "SELECT anilist_id, COALESCE(mal_id::text,''), COALESCE(title_english,''),
//	            COALESCE(title_romaji,''), COALESCE(season_year::text,''),
//	            COALESCE(type,''), COALESCE(format,'')
//	     FROM anime WHERE wikidata_qid IS NULL ORDER BY anilist_id" \
//	  | ./wikidata_qid [flags] > results.csv

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
	userAgent      = "AniGraph-WikidataEnricher/1.0 (anigraph.xyz)"
	sparqlBatch    = 50 // IDs per SPARQL VALUES block

	// Decodo proxy pool — same credentials as scrape.go / scrape_incremental.go.
	defaultProxyHost  = "dc.decodo.com"
	defaultProxyUser  = "spawee4ylf"
	defaultProxyPass  = "yIzsp7~aeb7Yrz87RQ"
	defaultPortStart  = 10001
	defaultPortEnd    = 10100
	defaultMinSeconds = 3.8 // conservative rate (same as scrape_incremental.go)
)

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

type animeRow struct {
	anilistID    string
	malID        string
	titleEnglish string
	titleRomaji  string
	year         string
	mediaType    string // "ANIME" or "MANGA"
	format       string // "TV", "TV_SHORT", "MOVIE", "OVA", "ONA", "SPECIAL", "MUSIC", "MANGA", "NOVEL", "ONE_SHOT", "MANHWA", "MANHUA"
}

type result struct {
	anilistID   string
	wikidataQID string
	method      string // "mal" | "anilist" | "title" | "not_found"
}

type searchHit struct {
	qid   string
	label string
}

type ambiguousEntry struct {
	anime      animeRow
	candidates []searchHit
}

// extraProps holds all additional Wikidata-sourced identifiers fetched in Phase 4.
type extraProps struct {
	wikipediaEn    string // full URL e.g. https://en.wikipedia.org/wiki/One_Piece
	wikipediaJa    string // full URL e.g. https://ja.wikipedia.org/wiki/…
	livechartID    string // P12489
	notifyID       string // P12427
	tvdbID         string // P4835
	tmdbMovieID    string // P4947
	tmdbTvID       string // P4983
	tvmazeID       string // P8600
	mywaifulistID  string // P14051
	unconsentingID string // P9821
}

// ---------------------------------------------------------------------------
// P31 type sets (format-specific)
// ---------------------------------------------------------------------------

// formatP31Set returns the Wikidata P31 QIDs that match the given AniList format.
// Falls back to broad type sets when format is unknown/empty.
// Returns nil when both format and mediaType are unrecognised (P31 skipped).
func formatP31Set(format, mediaType string) map[string]bool {
	switch strings.ToUpper(format) {
	case "TV", "TV_SHORT":
		return map[string]bool{
			"Q63952888": true, // anime television series
			"Q1107":     true, // anime
			"Q5398426":  true, // television series
			"Q24856":    true, // animated series
			"Q15416":    true, // television program
		}
	case "MOVIE":
		return map[string]bool{
			"Q63952273": true, // anime film
			"Q1107":     true, // anime
			"Q11424":    true, // film
			"Q581714":   true, // animated film
			"Q229390":   true, // 3D computer-animated film
		}
	case "OVA":
		return map[string]bool{
			"Q220898":  true, // original video animation
			"Q1107":    true, // anime
			"Q11424":   true, // film (some OVAs classified as film)
			"Q581714":  true, // animated film
			"Q5398426": true, // television series (OVA series)
			"Q24856":   true, // animated series
		}
	case "ONA":
		return map[string]bool{
			"Q1066454": true, // original net animation
			"Q1107":    true, // anime
			"Q5398426": true, // television series
			"Q24856":   true, // animated series
			"Q15416":   true, // television program
		}
	case "SPECIAL":
		return map[string]bool{
			"Q1070307":  true, // television special
			"Q63952888": true, // anime television series
			"Q1107":     true, // anime
			"Q5398426":  true, // television series
			"Q15416":    true, // television program
			"Q21191270": true, // television series episode
			"Q11424":    true, // film
		}
	case "MUSIC":
		return map[string]bool{
			"Q63952888": true, // anime television series
			"Q1107":     true, // anime
			"Q134556":   true, // music video
		}
	case "MANGA", "ONE_SHOT":
		return map[string]bool{"Q21198342": true}
	case "NOVEL":
		return map[string]bool{"Q747381": true}
	case "MANHWA":
		return map[string]bool{"Q1105937": true}
	case "MANHUA":
		return map[string]bool{"Q2389636": true}
	}
	switch strings.ToUpper(mediaType) {
	case "ANIME":
		return map[string]bool{
			"Q63952888": true, // anime television series
			"Q1107":     true, // anime
			"Q220898":   true, // OVA
			"Q63952273": true, // anime film
			"Q1070307":  true, // television special
			"Q1066454":  true, // ONA
			"Q11424":    true, // film
			"Q581714":   true, // animated film
			"Q5398426":  true, // television series
			"Q24856":    true, // animated series
			"Q15416":    true, // television program
			"Q229390":   true, // 3D computer-animated film
			"Q21191270": true, // television series episode
		}
	case "MANGA":
		return map[string]bool{
			"Q21198342": true, "Q747381": true, "Q693285": true,
			"Q1105937": true, "Q2389636": true,
		}
	}
	return nil
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
// This endpoint surfaces different candidates than wbsearchentities; the two are merged.
func mediawikiSearch(worker *ProxyWorker, title string) ([]searchHit, error) {
	const maxAttempts = 4

	params := url.Values{
		"action":      {"query"},
		"list":        {"search"},
		"srsearch":    {title},
		"srnamespace": {"0"},
		"srlimit":     {"10"},
		"srprop":      {""}, // no extra fields needed — just QIDs
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
					Title string `json:"title"` // "Q12345" on Wikidata
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
// (rate-limiting serialises them anyway) and merges results.
// wbsearchentities results take priority since they carry labels.
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
			merged = append(merged, h) // no label — P31 still works
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
	// Bare QID (no slash)
	if strings.HasPrefix(uri, "Q") {
		return uri
	}
	return ""
}

// ---------------------------------------------------------------------------
// Phase 1 & 2 — concurrent batch SPARQL
// ---------------------------------------------------------------------------

func batchSPARQL(items []animeRow, property string, getID func(animeRow) string, workers []*ProxyWorker) map[string]string {
	var withID []animeRow
	for _, a := range items {
		if getID(a) != "" {
			withID = append(withID, a)
		}
	}
	if len(withID) == 0 {
		return map[string]string{}
	}

	type batch struct {
		items []animeRow
		index int
	}
	var batches []batch
	for i := 0; i < len(withID); i += sparqlBatch {
		end := i + sparqlBatch
		if end > len(withID) {
			end = len(withID)
		}
		batches = append(batches, batch{withID[i:end], i / sparqlBatch})
	}

	batchCh := make(chan batch, len(batches))
	for _, b := range batches {
		batchCh <- b
	}
	close(batchCh)

	var mu sync.Mutex
	out := make(map[string]string)
	var wg sync.WaitGroup

	for _, w := range workers {
		wg.Add(1)
		go func(worker *ProxyWorker) {
			defer wg.Done()
			for b := range batchCh {
				vals := make([]string, len(b.items))
				for i, a := range b.items {
					vals[i] = `"` + getID(a) + `"`
				}
				query := `SELECT ?item ?extId WHERE {
  VALUES ?extId { ` + strings.Join(vals, " ") + ` }
  ?item wdt:` + property + ` ?extId .
} LIMIT 200`

				rows, err := sparqlQuery(worker, query)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  batch %d SPARQL error: %v\n", b.index, err)
					continue
				}
				mu.Lock()
				for _, row := range rows {
					qid := extractQID(row["item"])
					extID := row["extId"]
					if qid != "" && extID != "" {
						if _, exists := out[extID]; !exists {
							out[extID] = qid
						}
					}
				}
				mu.Unlock()

				if (b.index+1)%10 == 0 {
					fmt.Fprintf(os.Stderr, "  %s: batch %d/%d done\n", property, b.index+1, len(batches))
				}
			}
		}(w)
	}
	wg.Wait()
	return out
}

// ---------------------------------------------------------------------------
// Phase 3 — title search + P31 gate + disambiguation
// ---------------------------------------------------------------------------

// pendingSearch holds title-search results for one anime before SPARQL filtering.
type pendingSearch struct {
	anime       animeRow
	englishHits []searchHit // from wikidataSearchDual on titleEnglish
	romajiHits  []searchHit // from wikidataSearchDual on titleRomaji
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

// batchFetchYears fetches P577/P580 years for all given QIDs using batched SPARQL.
// Returns map[QID]→[]years.
func batchFetchYears(qids []string, workers []*ProxyWorker) map[string][]string {
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
				query := `SELECT ?item (YEAR(?date) AS ?year) WHERE {
  VALUES ?item { ` + strings.Join(vals, " ") + ` }
  { ?item wdt:P577 ?date . } UNION { ?item wdt:P580 ?date . }
}`
				rows, err := sparqlQuery(worker, query)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  year batch %d SPARQL error: %v\n", b.index, err)
					continue
				}
				mu.Lock()
				for _, row := range rows {
					item := extractQID(row["item"])
					year := row["year"]
					if item != "" && year != "" {
						out[item] = append(out[item], year)
					}
				}
				mu.Unlock()

				if (b.index+1)%50 == 0 {
					fmt.Fprintf(os.Stderr, "  year-batch: %d/%d done\n", b.index+1, len(batches))
				}
			}
		}(w)
	}
	wg.Wait()
	fmt.Fprintf(os.Stderr, "  year-batch: all %d batches done (%d unique QIDs)\n", len(batches), len(qids))
	return out
}

// p31FilterHits applies P31 type filtering locally using a pre-fetched p31Map.
// Returns only hits whose P31 types match the format/mediaType set.
func p31FilterHits(hits []searchHit, format, mediaType string, p31Map map[string][]string) []searchHit {
	p31Set := formatP31Set(format, mediaType)
	if p31Set == nil {
		return hits
	}
	var matched []searchHit
	for _, h := range hits {
		for _, t := range p31Map[h.qid] {
			if p31Set[t] {
				matched = append(matched, h)
				break
			}
		}
	}
	return matched
}

// yearFilterHits applies year filtering locally using a pre-fetched yearMap.
// Falls back to original list when no candidate has matching date data.
func yearFilterHits(hits []searchHit, year string, yearMap map[string][]string) []searchHit {
	if year == "" || len(hits) == 0 {
		return hits
	}
	// Check if any candidate has year data at all.
	anyData := false
	for _, h := range hits {
		if len(yearMap[h.qid]) > 0 {
			anyData = true
			break
		}
	}
	if !anyData {
		return hits // no date props found at all — don't discard
	}

	var matched []searchHit
	for _, h := range hits {
		for _, y := range yearMap[h.qid] {
			if y == year {
				matched = append(matched, h)
				break
			}
		}
	}
	if len(matched) == 0 {
		return hits // dates exist but none matched year — don't discard
	}
	return matched
}

// ---------------------------------------------------------------------------
// Phase 4 — fetch extra properties for all resolved QIDs
// ---------------------------------------------------------------------------

func fetchExtraProps(resolved map[string]result, workers []*ProxyWorker) map[string]extraProps {
	// Build slice of (anilistID, QID) pairs and reverse lookup.
	type pair struct{ anilistID, qid string }
	var pairs []pair
	qidToAnilist := make(map[string]string)
	for anilistID, res := range resolved {
		if res.wikidataQID == "" {
			continue
		}
		pairs = append(pairs, pair{anilistID, res.wikidataQID})
		qidToAnilist[res.wikidataQID] = anilistID
	}
	if len(pairs) == 0 {
		return map[string]extraProps{}
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
	out := make(map[string]extraProps)
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

				rows, err := sparqlQuery(worker, query)
				if err != nil {
					fmt.Fprintf(os.Stderr, "  extra-props batch %d SPARQL error: %v\n", b.index, err)
					continue
				}

				mu.Lock()
				for _, row := range rows {
					qid := extractQID(row["item"])
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
	proxyStr           := flag.String("proxy", "", "Proxy template host:port:user:pass (overrides hardcoded Decodo creds)")
	proxyStart         := flag.Int("proxy-start", defaultPortStart, "First proxy port")
	proxyEnd           := flag.Int("proxy-end", defaultPortEnd, "Last proxy port")
	minInterval        := flag.Float64("min-interval", defaultMinSeconds, "Minimum seconds between requests per proxy")
	sparqlWorkerCount  := flag.Int("sparql-workers", 8, "Concurrent workers for SPARQL batch phases")
	titleWorkerCount   := flag.Int("workers", 100, "Concurrent workers for title search API phase (no SPARQL)")
	ambigFile          := flag.String("ambiguous", "wikidata_ambiguous.csv", "File for ambiguous candidates")
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

	sparqlWorkers := allWorkers
	if *sparqlWorkerCount < len(allWorkers) {
		sparqlWorkers = allWorkers[:*sparqlWorkerCount]
	}
	titleWorkers := allWorkers
	if *titleWorkerCount < len(allWorkers) {
		titleWorkers = allWorkers[:*titleWorkerCount]
	}

	// ---- Read stdin CSV ----
	// Columns: anilist_id, mal_id, title_english, title_romaji, season_year, type, format
	r := csv.NewReader(bufio.NewReader(os.Stdin))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var all []animeRow
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "CSV error: %v\n", err)
			continue
		}
		if len(row) < 1 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		a := animeRow{anilistID: strings.TrimSpace(row[0])}
		if len(row) > 1 { a.malID = strings.TrimSpace(row[1]) }
		if len(row) > 2 { a.titleEnglish = strings.TrimSpace(row[2]) }
		if len(row) > 3 { a.titleRomaji = strings.TrimSpace(row[3]) }
		if len(row) > 4 { a.year = strings.TrimSpace(row[4]) }
		if len(row) > 5 { a.mediaType = strings.TrimSpace(row[5]) }
		if len(row) > 6 { a.format = strings.TrimSpace(row[6]) }
		all = append(all, a)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d anime to resolve\n\n", len(all))

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

	resolved := make(map[string]result)

	// ---- Phase 1: MAL ID ----
	fmt.Fprintf(os.Stderr, "Phase 1: MAL ID SPARQL (P4086) — %d workers...\n", len(sparqlWorkers))
	malMap := batchSPARQL(all, "P4086", func(a animeRow) string { return a.malID }, sparqlWorkers)
	for _, a := range all {
		if qid, ok := malMap[a.malID]; ok {
			resolved[a.anilistID] = result{a.anilistID, qid, "mal"}
		}
	}
	fmt.Fprintf(os.Stderr, "  -> %d found via MAL ID\n\n", len(resolved))

	// ---- Phase 2: AniList ID ----
	var rem2 []animeRow
	for _, a := range all {
		if _, done := resolved[a.anilistID]; !done {
			rem2 = append(rem2, a)
		}
	}
	fmt.Fprintf(os.Stderr, "Phase 2: AniList ID SPARQL (P8729) — %d workers, %d remaining...\n", len(sparqlWorkers), len(rem2))
	phase2start := len(resolved)
	anilistMap := batchSPARQL(rem2, "P8729", func(a animeRow) string { return a.anilistID }, sparqlWorkers)
	for _, a := range rem2 {
		if qid, ok := anilistMap[a.anilistID]; ok {
			resolved[a.anilistID] = result{a.anilistID, qid, "anilist"}
		}
	}
	fmt.Fprintf(os.Stderr, "  -> %d found via AniList ID\n\n", len(resolved)-phase2start)

	// ---- Phase 3: Title search pipeline (3a→3b→3c→3d) ----
	var rem3 []animeRow
	for _, a := range all {
		if _, done := resolved[a.anilistID]; !done {
			rem3 = append(rem3, a)
		}
	}

	// ---- Phase 3a: Title search (API only, no SPARQL) ----
	fmt.Fprintf(os.Stderr, "Phase 3a: Title search (dual-API) — %d workers, %d remaining...\n", len(titleWorkers), len(rem3))

	type indexedPending struct {
		idx int
		ps  pendingSearch
	}
	jobs := make(chan indexedPending, len(rem3))
	for i, a := range rem3 {
		jobs <- indexedPending{i, pendingSearch{anime: a}}
	}
	close(jobs)

	pending := make([]pendingSearch, len(rem3))
	var wg sync.WaitGroup

	for i, w := range titleWorkers {
		if i >= len(rem3) {
			break
		}
		wg.Add(1)
		go func(worker *ProxyWorker) {
			defer wg.Done()
			for job := range jobs {
				a := job.ps.anime
				if a.titleEnglish != "" {
					job.ps.englishHits = wikidataSearchDual(worker, a.titleEnglish)
				}
				if a.titleRomaji != "" && a.titleRomaji != a.titleEnglish {
					job.ps.romajiHits = wikidataSearchDual(worker, a.titleRomaji)
				}
				pending[job.idx] = job.ps
			}
		}(w)
	}
	wg.Wait()
	fmt.Fprintf(os.Stderr, "  -> Phase 3a done: %d anime searched\n\n", len(rem3))

	// ---- Phase 3b: Batch P31 filter ----
	// Collect all unique candidate QIDs.
	uniqueQIDs := make(map[string]bool)
	for _, ps := range pending {
		for _, h := range ps.englishHits {
			uniqueQIDs[h.qid] = true
		}
		for _, h := range ps.romajiHits {
			uniqueQIDs[h.qid] = true
		}
	}
	allQIDs := make([]string, 0, len(uniqueQIDs))
	for q := range uniqueQIDs {
		allQIDs = append(allQIDs, q)
	}
	fmt.Fprintf(os.Stderr, "Phase 3b: Batch P31 filter — %d unique QIDs, %d workers...\n", len(allQIDs), len(sparqlWorkers))

	p31Map := batchFetchP31(allQIDs, sparqlWorkers)

	// Apply P31 filter locally per anime.
	var mu sync.Mutex
	var ambiguous []ambiguousEntry
	phase3found, phase3ambiguous := 0, 0
	type needsYear struct {
		idx  int          // index into pending
		hits []searchHit  // P31-surviving candidates
	}
	var needYearFilter []needsYear

	for i, ps := range pending {
		a := ps.anime
		// Try English hits first.
		engFiltered := p31FilterHits(ps.englishHits, a.format, a.mediaType, p31Map)
		if len(engFiltered) == 1 {
			resolved[a.anilistID] = result{a.anilistID, engFiltered[0].qid, "title"}
			phase3found++
			continue
		}
		// Try romaji hits if English produced 0.
		if len(engFiltered) == 0 {
			romFiltered := p31FilterHits(ps.romajiHits, a.format, a.mediaType, p31Map)
			if len(romFiltered) == 1 {
				resolved[a.anilistID] = result{a.anilistID, romFiltered[0].qid, "title"}
				phase3found++
				continue
			}
			if len(romFiltered) == 0 {
				// No P31 matches from either title — not found
				continue
			}
			// Multiple romaji survivors → need year filter
			needYearFilter = append(needYearFilter, needsYear{i, romFiltered})
			continue
		}
		// Multiple English survivors → need year filter
		needYearFilter = append(needYearFilter, needsYear{i, engFiltered})
	}
	fmt.Fprintf(os.Stderr, "  -> Phase 3b: %d resolved, %d need year filter\n\n", phase3found, len(needYearFilter))

	// ---- Phase 3c: Batch year filter ----
	if len(needYearFilter) > 0 {
		// Collect unique QIDs that still need year data.
		yearQIDSet := make(map[string]bool)
		for _, ny := range needYearFilter {
			for _, h := range ny.hits {
				yearQIDSet[h.qid] = true
			}
		}
		yearQIDs := make([]string, 0, len(yearQIDSet))
		for q := range yearQIDSet {
			yearQIDs = append(yearQIDs, q)
		}
		fmt.Fprintf(os.Stderr, "Phase 3c: Batch year filter — %d unique QIDs, %d workers...\n", len(yearQIDs), len(sparqlWorkers))

		yearMap := batchFetchYears(yearQIDs, sparqlWorkers)

		// Apply year filter locally.
		phase3cResolved := 0
		var stillAmbiguous []needsYear
		for _, ny := range needYearFilter {
			a := pending[ny.idx].anime
			filtered := yearFilterHits(ny.hits, a.year, yearMap)
			if len(filtered) == 1 {
				resolved[a.anilistID] = result{a.anilistID, filtered[0].qid, "title"}
				phase3found++
				phase3cResolved++
				continue
			}
			stillAmbiguous = append(stillAmbiguous, needsYear{ny.idx, filtered})
		}
		fmt.Fprintf(os.Stderr, "  -> Phase 3c: %d resolved, %d still ambiguous\n\n", phase3cResolved, len(stillAmbiguous))

		// ---- Phase 3d: Exact label match (pure local, no network) ----
		phase3dResolved := 0
		for _, ny := range stillAmbiguous {
			a := pending[ny.idx].anime
			searchTitles := []string{a.titleEnglish, a.titleRomaji}
			var exactMatches []searchHit
			for _, h := range ny.hits {
				label := strings.ToLower(strings.TrimSpace(h.label))
				for _, t := range searchTitles {
					if t != "" && label == strings.ToLower(strings.TrimSpace(t)) {
						exactMatches = append(exactMatches, h)
						break
					}
				}
			}
			if len(exactMatches) == 1 {
				resolved[a.anilistID] = result{a.anilistID, exactMatches[0].qid, "title"}
				phase3found++
				phase3dResolved++
			} else {
				mu.Lock()
				ambiguous = append(ambiguous, ambiguousEntry{a, ny.hits})
				phase3ambiguous++
				mu.Unlock()
			}
		}
		fmt.Fprintf(os.Stderr, "  -> Phase 3d: %d resolved via exact label, %d ambiguous\n\n", phase3dResolved, phase3ambiguous)
	}

	fmt.Fprintf(os.Stderr, "  -> Phase 3 total: %d found, %d ambiguous (-> %s)\n\n", phase3found, phase3ambiguous, *ambigFile)

	// ---- Phase 4: Extra properties for all resolved QIDs ----
	fmt.Fprintf(os.Stderr, "Phase 4: Extra properties — %d resolved QIDs, %d workers...\n", len(resolved), len(sparqlWorkers))
	extraMap := fetchExtraProps(resolved, sparqlWorkers)
	fmt.Fprintf(os.Stderr, "  -> extra-props fetched for %d entries\n\n", len(extraMap))

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
			aw.Write([]string{"anilist_id", "title_english", "title_romaji", "year", "type", "format", "candidate_qid", "candidate_label"})
			totalRows := 0
			for _, e := range ambiguous {
				for _, c := range e.candidates {
					aw.Write([]string{
						e.anime.anilistID, e.anime.titleEnglish, e.anime.titleRomaji,
						e.anime.year, e.anime.mediaType, e.anime.format,
						c.qid, c.label,
					})
					totalRows++
				}
			}
			aw.Flush()
			af.Close()
			fmt.Fprintf(os.Stderr, "Wrote %d ambiguous anime (%d candidate rows) to %s\n",
				len(ambiguous), totalRows, *ambigFile)
		}
	}

	// ---- Write main results CSV ----
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{
		"anilist_id", "wikidata_qid", "method",
		"wikipedia_en", "wikipedia_ja",
		"livechart_id", "notify_id", "tvdb_id", "tmdb_movie_id", "tmdb_tv_id",
		"tvmaze_id", "mywaifulist_id", "unconsenting_media_id",
	})
	for _, a := range all {
		ep := extraMap[a.anilistID]
		if res, ok := resolved[a.anilistID]; ok {
			w.Write([]string{
				res.anilistID, res.wikidataQID, res.method,
				ep.wikipediaEn, ep.wikipediaJa,
				ep.livechartID, ep.notifyID, ep.tvdbID, ep.tmdbMovieID, ep.tmdbTvID,
				ep.tvmazeID, ep.mywaifulistID, ep.unconsentingID,
			})
		} else {
			w.Write([]string{
				a.anilistID, "", "not_found",
				"", "", "", "", "", "", "", "", "", "",
			})
		}
	}
	w.Flush()
}
