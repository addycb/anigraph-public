package main

// wikipedia_production.go
//
// Scrapes the "Production" section from English Wikipedia articles for anime
// that have a wikipedia_en URL in the database.
//
// Input:  CSV with columns: anilist_id, wikipedia_en
//         (export from DB: SELECT anilist_id, wikipedia_en FROM anime WHERE wikipedia_en IS NOT NULL)
//
// Output: CSV with columns: anilist_id, wikipedia_url, production_html
//
// Uses the MediaWiki Parse API:
//   1. GET action=parse&page=TITLE&prop=sections  → find "Production" section index
//   2. GET action=parse&page=TITLE&section=N&prop=text → get section HTML
//
// Uses the same proxy worker pool pattern as sakugabooru_posts.go.
//
// Build & run:
//   go build -o wikipedia_production wikipedia_production.go
//   ./wikipedia_production -in anime_wikipedia.csv -out wikipedia_production_notes.csv

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	wikiAPIBase  = "https://en.wikipedia.org/w/api.php"
	wikiAgent    = "AniGraph-WikiProduction/1.0 (anigraph.xyz)"

	// Proxy config — same pool as sakugabooru_posts.go
	wikiProxyExample  = "dc.decodo.com:10001:spawee4ylf:yIzsp7~aeb7Yrz87RQ"
	wikiProxyPortLow  = 10001
	wikiProxyPortHigh = 10100

	// Wikipedia is generous (~200 req/s) but we're polite.
	// 0.5s per proxy × 100 proxies = ~200 req/s max.
	wikiDefaultDelay = 0.5
)

// ---------------------------------------------------------------------------
// Proxy worker (same pattern as sakugabooru_posts.go)
// ---------------------------------------------------------------------------

type wikiProxyWorker struct {
	proxyURL    string
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func newWikiProxyWorker(proxyURL string, minInterval time.Duration) *wikiProxyWorker {
	proxy, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxy),
		MaxIdleConnsPerHost: 4,
	}
	return &wikiProxyWorker{
		proxyURL:    proxyURL,
		client:      &http.Client{Transport: transport, Timeout: 30 * time.Second},
		minInterval: minInterval,
	}
}

func (w *wikiProxyWorker) get(rawURL string) (*http.Response, error) {
	w.mu.Lock()
	if elapsed := time.Since(w.lastUsed); elapsed < w.minInterval {
		time.Sleep(w.minInterval - elapsed)
	}
	w.lastUsed = time.Now()
	w.mu.Unlock()

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", wikiAgent)
	return w.client.Do(req)
}

func buildWikiProxies(example string, portLow, portHigh int) []*wikiProxyWorker {
	parts := strings.Split(example, ":")
	if len(parts) < 4 {
		panic("Proxy format must be host:port:username:password")
	}
	host := parts[0]
	username := parts[2]
	password := parts[3]

	interval := time.Duration(wikiDefaultDelay * float64(time.Second))
	var workers []*wikiProxyWorker
	for port := portLow; port <= portHigh; port++ {
		proxyURL := fmt.Sprintf("http://%s:%s@%s:%d", username, password, host, port)
		workers = append(workers, newWikiProxyWorker(proxyURL, interval))
	}
	return workers
}

// ---------------------------------------------------------------------------
// MediaWiki API response types
// ---------------------------------------------------------------------------

type mwSectionsResponse struct {
	Parse struct {
		Sections []struct {
			Index   string `json:"index"`
			Line    string `json:"line"`
			Level   string `json:"level"`
			Number  string `json:"number"`
		} `json:"sections"`
	} `json:"parse"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

type mwTextResponse struct {
	Parse struct {
		Title string `json:"title"`
		Text  struct {
			Content string `json:"*"`
		} `json:"text"`
	} `json:"parse"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// Input row
// ---------------------------------------------------------------------------

type wikiJob struct {
	anilistID    string
	wikipediaURL string
	pageTitle    string // extracted from URL
}

// Extract page title from a Wikipedia URL like https://en.wikipedia.org/wiki/One_Piece
var wikiURLRe = regexp.MustCompile(`/wiki/(.+)$`)

func extractPageTitle(wikiURL string) string {
	m := wikiURLRe.FindStringSubmatch(wikiURL)
	if len(m) < 2 {
		return ""
	}
	// Decode percent-encoded characters (e.g. %C5%8D → ō) — the MediaWiki API
	// requires decoded Unicode titles, not percent-encoded ones.
	decoded, err := url.PathUnescape(m[1])
	if err != nil {
		return m[1]
	}
	return decoded
}

// ---------------------------------------------------------------------------
// Section name matching
// ---------------------------------------------------------------------------

// productionSectionNames lists the section titles we consider as "production notes".
// Checked case-insensitively. Order matters — first match wins.
var productionSectionNames = []string{
	// High-frequency exact matches
	"production",
	"development",
	"production and development",
	"production notes",
	"development and production",
	"production and release",
	"development and release",
	"creation and conception",
	"publication and conception",
	"production and broadcasting",
	"production and publication",
	"concept and creation",
	"background and release",
	"background and development",
	"background and production",
	"conception and development",
	"creation and development",
	"creation and release",
	"concept and development",
	"concept and design",
	"conception and creation",
	"creation and design",
	"production and style",
	"production and themes",
	"writing and production",
	"airing and production",
	"production background",
	"production and filming",
	"production and broadcast",
	"production overview and history",
	"inspiration and concept",
	"background and writing",
	"creation and publication history",

	// Lower-frequency exact matches
	"background and creation",
	"background and inspiration",
	"background and premise",
	"background and promotion",
	"background, publication and influences",
	"production and distribution",
	"production history",
	"production and reception",
	"release and production",
	"recording and production",
	"production and cult status",
	"production and promotion",
	"production and main cast",
	"production and media",
	"episodes and production details",
	"production and release notes",
	"staff and production notes",
	"staff & production notes",
	"historical background and production",
	"extra production information",
	"conception and design",
	"conception and voice",
	"preproduction",
	"post production",
	"origin and development",
	"development and publication",
	"development and releases",
	"creation and history",
	"publication and creation",
	"design and development",
	"history and publication",
	"production note",
	"conception",
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// productionKeywords are checked as a fallback when no exact section name matches.
// If the section title contains any of these keywords, it's considered a match.
var productionKeywords = []string{
	"production",
	"development",
	"conception",
	"pre-production",
	"preproduction",
	"post-production",
}

func isProductionSection(sectionLine string) bool {
	lower := strings.ToLower(strings.TrimSpace(sectionLine))
	// Strip any HTML tags that MediaWiki might include in the line field
	lower = htmlTagRe.ReplaceAllString(lower, "")
	lower = strings.TrimSpace(lower)

	// First: try exact match against known section names
	for _, name := range productionSectionNames {
		if lower == name {
			return true
		}
	}
	return false
}

// fallbackExcludedSections are section names that contain a production keyword
// but are NOT about the production process (e.g. staff credits, trivia, biology).
var fallbackExcludedSections = []string{
	"production staff",
	"production credits",
	"production team",
	"production studio",
	"production crew",
	"production company",
	"commercial production",
	"ova production staff",
	"last years of film production",
	"japanese film production",
	"development in humans",
	"product development",
}

// isProductionSectionFallback uses keyword-based matching as a fallback.
// Only call this when isProductionSection returns false for ALL sections on a page.
func isProductionSectionFallback(sectionLine string) bool {
	lower := strings.ToLower(strings.TrimSpace(sectionLine))
	lower = htmlTagRe.ReplaceAllString(lower, "")
	lower = strings.TrimSpace(lower)

	for _, excluded := range fallbackExcludedSections {
		if lower == excluded {
			return false
		}
	}
	for _, kw := range productionKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Fetch logic
// ---------------------------------------------------------------------------

type matchedSection struct {
	index string
	title string
}

type productionResult struct {
	anilistID    string
	wikipediaURL string
	sectionTitle string // comma-separated if multiple
	html         string // concatenated HTML of all matched sections
	err          error
}

func fetchWithRetry(worker *wikiProxyWorker, rawURL string, maxAttempts int) (*http.Response, error) {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*3) * time.Second
			time.Sleep(backoff)
		}

		resp, err := worker.get(rawURL)
		if err != nil {
			continue
		}

		switch {
		case resp.StatusCode == 429:
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "  rate limited (429), waiting 60s...\n")
			time.Sleep(60 * time.Second)
			continue
		case resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504:
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "  HTTP %d (attempt %d/%d), retrying...\n", resp.StatusCode, attempt+1, maxAttempts)
			continue
		case resp.StatusCode == 200:
			return resp, nil
		default:
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:min(len(body), 200)]))
		}
	}
	return nil, fmt.Errorf("failed after %d attempts", maxAttempts)
}

func fetchProductionSection(worker *wikiProxyWorker, job wikiJob, requestCount *int64) productionResult {
	result := productionResult{
		anilistID:    job.anilistID,
		wikipediaURL: job.wikipediaURL,
	}

	// Step 1: Get sections list
	params := url.Values{}
	params.Set("action", "parse")
	params.Set("page", job.pageTitle)
	params.Set("prop", "sections")
	params.Set("format", "json")
	sectionsURL := wikiAPIBase + "?" + params.Encode()

	resp, err := fetchWithRetry(worker, sectionsURL, 4)
	atomic.AddInt64(requestCount, 1)
	if err != nil {
		result.err = fmt.Errorf("sections request: %w", err)
		return result
	}

	var sectionsResp mwSectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&sectionsResp); err != nil {
		resp.Body.Close()
		result.err = fmt.Errorf("sections decode: %w", err)
		return result
	}
	resp.Body.Close()

	if sectionsResp.Error != nil {
		result.err = fmt.Errorf("API error: %s — %s", sectionsResp.Error.Code, sectionsResp.Error.Info)
		return result
	}

	// Step 2: Find ALL matching production sections (exact match first, then keyword fallback)
	var matched []matchedSection
	for _, sec := range sectionsResp.Parse.Sections {
		if isProductionSection(sec.Line) {
			matched = append(matched, matchedSection{index: sec.Index, title: sec.Line})
		}
	}

	// Fallback: keyword-based matching if no exact matches found
	if len(matched) == 0 {
		for _, sec := range sectionsResp.Parse.Sections {
			if isProductionSectionFallback(sec.Line) {
				matched = append(matched, matchedSection{index: sec.Index, title: sec.Line})
			}
		}
	}

	if len(matched) == 0 {
		result.err = fmt.Errorf("no production section found")
		return result
	}

	// Step 3: Fetch HTML for each matched section
	var htmlParts []string
	var titles []string
	for _, m := range matched {
		params2 := url.Values{}
		params2.Set("action", "parse")
		params2.Set("page", job.pageTitle)
		params2.Set("section", m.index)
		params2.Set("prop", "text")
		params2.Set("format", "json")
		params2.Set("disabletoc", "true")
		textURL := wikiAPIBase + "?" + params2.Encode()

		resp2, err := fetchWithRetry(worker, textURL, 4)
		atomic.AddInt64(requestCount, 1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to fetch section %q (index %s): %v\n", m.title, m.index, err)
			continue
		}

		var textResp mwTextResponse
		if err := json.NewDecoder(resp2.Body).Decode(&textResp); err != nil {
			resp2.Body.Close()
			fmt.Fprintf(os.Stderr, "  warning: failed to decode section %q (index %s): %v\n", m.title, m.index, err)
			continue
		}
		resp2.Body.Close()

		if textResp.Error != nil {
			fmt.Fprintf(os.Stderr, "  warning: API error for section %q: %s — %s\n", m.title, textResp.Error.Code, textResp.Error.Info)
			continue
		}

		htmlParts = append(htmlParts, textResp.Parse.Text.Content)
		titles = append(titles, m.title)
	}

	if len(htmlParts) == 0 {
		result.err = fmt.Errorf("matched %d sections but failed to fetch all", len(matched))
		return result
	}

	result.sectionTitle = strings.Join(titles, ", ")
	result.html = strings.Join(htmlParts, "\n")
	return result
}

// ---------------------------------------------------------------------------
// CSV loading
// ---------------------------------------------------------------------------

func loadWikiCSV(filename string) ([]wikiJob, error) {
	var reader io.Reader
	if filename == "" || filename == "-" {
		reader = os.Stdin
	} else {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		reader = f
	}

	r := csv.NewReader(bufio.NewReader(reader))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(h)] = i
	}
	for _, need := range []string{"anilist_id", "wikipedia_en"} {
		if _, ok := colIdx[need]; !ok {
			return nil, fmt.Errorf("column %q not found (columns: %v)", need, header)
		}
	}

	get := func(row []string, col string) string {
		i := colIdx[col]
		if i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	var jobs []wikiJob
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "CSV read error: %v\n", err)
			continue
		}
		anilistID := get(row, "anilist_id")
		wikiURL := get(row, "wikipedia_en")
		if anilistID == "" || wikiURL == "" {
			continue
		}
		title := extractPageTitle(wikiURL)
		if title == "" {
			fmt.Fprintf(os.Stderr, "Could not extract page title from %q (anilist_id=%s)\n", wikiURL, anilistID)
			continue
		}
		jobs = append(jobs, wikiJob{
			anilistID:    anilistID,
			wikipediaURL: wikiURL,
			pageTitle:    title,
		})
	}
	return jobs, nil
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	inFile := flag.String("in", "", "Input CSV (omit or '-' for stdin)")
	outFile := flag.String("out", "", "Output CSV (omit or '-' for stdout)")
	delay := flag.Float64("delay", wikiDefaultDelay, "Min seconds between requests PER PROXY")
	numWorkers := flag.Int("workers", 0, "Number of proxy workers (0 = all available)")
	noProxy := flag.Bool("no-proxy", false, "Use direct connections (no proxy pool)")
	flag.Parse()

	// Load input
	jobs, err := loadWikiCSV(*inFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading CSV: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d anime with Wikipedia URLs\n", len(jobs))

	// Build proxy pool (or direct workers)
	var proxyPool []*wikiProxyWorker
	if *noProxy {
		// Single direct worker (no proxy)
		interval := time.Duration(*delay * float64(time.Second))
		proxyPool = []*wikiProxyWorker{{
			client:      &http.Client{Timeout: 30 * time.Second},
			minInterval: interval,
		}}
	} else {
		proxyPool = buildWikiProxies(wikiProxyExample, wikiProxyPortLow, wikiProxyPortHigh)
		if *delay != wikiDefaultDelay {
			interval := time.Duration(*delay * float64(time.Second))
			for _, w := range proxyPool {
				w.minInterval = interval
			}
		}
	}

	poolSize := len(proxyPool)
	if *numWorkers > 0 && *numWorkers < poolSize {
		poolSize = *numWorkers
	}
	proxyPool = proxyPool[:poolSize]

	fmt.Fprintf(os.Stderr, "%d workers, %.1fs delay → ~%.0f req/s max\n\n",
		poolSize, *delay, float64(poolSize) / *delay)

	startTime := time.Now()

	// Job channel
	jobCh := make(chan wikiJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	// Results
	var mu sync.Mutex
	var results []productionResult
	var totalRequests int64
	var completedJobs int64
	totalJobs := int64(len(jobs))

	// Stats reporter
	stopStats := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				done := atomic.LoadInt64(&completedJobs)
				reqs := atomic.LoadInt64(&totalRequests)
				elapsed := time.Since(startTime).Seconds()
				rate := float64(reqs) / elapsed

				mu.Lock()
				found := 0
				for _, r := range results {
					if r.err == nil {
						found++
					}
				}
				mu.Unlock()

				fmt.Fprintf(os.Stderr, "[STATS] %d/%d anime (%.0f%%) | %d found | %d reqs (%.1f req/s) | %.0fs elapsed\n",
					done, totalJobs, float64(done)/float64(totalJobs)*100,
					found, reqs, rate, elapsed)
			case <-stopStats:
				return
			}
		}
	}()

	// Worker pool
	var wg sync.WaitGroup
	for i := 0; i < poolSize; i++ {
		wg.Add(1)
		go func(workerID int, proxy *wikiProxyWorker) {
			defer wg.Done()
			for job := range jobCh {
				result := fetchProductionSection(proxy, job, &totalRequests)

				mu.Lock()
				results = append(results, result)
				mu.Unlock()

				done := atomic.AddInt64(&completedJobs, 1)

				if result.err != nil {
					fmt.Fprintf(os.Stderr, "[w%d] (%d/%d) %s → %v\n",
						workerID, done, totalJobs, job.pageTitle, result.err)
				} else {
					htmlLen := len(result.html)
					fmt.Fprintf(os.Stderr, "[w%d] (%d/%d) %s → %q (%d chars)\n",
						workerID, done, totalJobs, job.pageTitle, result.sectionTitle, htmlLen)
				}
			}
		}(i, proxyPool[i])
	}
	wg.Wait()
	close(stopStats)

	elapsed := time.Since(startTime)
	reqs := atomic.LoadInt64(&totalRequests)

	// Count successes
	found := 0
	notFound := 0
	errored := 0
	for _, r := range results {
		if r.err == nil {
			found++
		} else if strings.Contains(r.err.Error(), "no production section") {
			notFound++
		} else {
			errored++
		}
	}

	fmt.Fprintf(os.Stderr, "\nDone: %d found, %d no section, %d errors | %d reqs in %.1fs (%.1f req/s)\n",
		found, notFound, errored, reqs, elapsed.Seconds(), float64(reqs)/elapsed.Seconds())

	// Sort results by anilist_id for deterministic output
	sort.Slice(results, func(i, j int) bool {
		return results[i].anilistID < results[j].anilistID
	})

	// Write output CSV
	var outWriter io.Writer
	if *outFile == "" || *outFile == "-" {
		outWriter = os.Stdout
	} else {
		outF, err := os.Create(*outFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot create output file: %v\n", err)
			os.Exit(1)
		}
		defer outF.Close()
		outWriter = outF
	}
	w := csv.NewWriter(outWriter)
	w.Write([]string{"anilist_id", "wikipedia_url", "production_html"})

	written := 0
	for _, r := range results {
		if r.err != nil {
			continue
		}
		w.Write([]string{r.anilistID, r.wikipediaURL, r.html})
		written++
	}
	w.Flush()

	dest := *outFile
	if dest == "" || dest == "-" {
		dest = "stdout"
	}
	fmt.Fprintf(os.Stderr, "Wrote %d rows → %s\n", written, dest)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
