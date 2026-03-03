package main

// wikipedia_studio_content.go
//
// Scrapes Wikipedia content for animation studios that have a wikipedia_en URL.
// Fetches the lead section (summary) plus all informational sections, excluding
// works/productions lists, references, staff lists, and geographic subsections.
//
// Input:  CSV with columns: studio_id, wikipedia_en
// Output: CSV with columns: studio_id, wikipedia_url, content_html
//
// Uses the same MediaWiki Parse API and proxy worker pool as wikipedia_production.go.
//
// Build & run:
//   go build -o wikipedia_studio_content wikipedia_studio_content.go
//   ./wikipedia_studio_content -in studio_wikipedia.csv -out studio_content.csv

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
	scAPIBase  = "https://en.wikipedia.org/w/api.php"
	scAgent    = "AniGraph-WikiStudioContent/1.0 (anigraph.xyz)"

	scProxyExample  = "dc.decodo.com:10001:spawee4ylf:yIzsp7~aeb7Yrz87RQ"
	scProxyPortLow  = 10001
	scProxyPortHigh = 10100

	scDefaultDelay = 0.5
)

// ---------------------------------------------------------------------------
// Proxy worker (same pattern as wikipedia_production.go)
// ---------------------------------------------------------------------------

type scProxyWorker struct {
	proxyURL    string
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func newSCProxyWorker(proxyURL string, minInterval time.Duration) *scProxyWorker {
	proxy, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxy),
		MaxIdleConnsPerHost: 4,
	}
	return &scProxyWorker{
		proxyURL:    proxyURL,
		client:      &http.Client{Transport: transport, Timeout: 30 * time.Second},
		minInterval: minInterval,
	}
}

func (w *scProxyWorker) get(rawURL string) (*http.Response, error) {
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
	req.Header.Set("User-Agent", scAgent)
	return w.client.Do(req)
}

func buildSCProxies(example string, portLow, portHigh int) []*scProxyWorker {
	parts := strings.Split(example, ":")
	if len(parts) < 4 {
		panic("Proxy format must be host:port:username:password")
	}
	host := parts[0]
	username := parts[2]
	password := parts[3]

	interval := time.Duration(scDefaultDelay * float64(time.Second))
	var workers []*scProxyWorker
	for port := portLow; port <= portHigh; port++ {
		proxyURL := fmt.Sprintf("http://%s:%s@%s:%d", username, password, host, port)
		workers = append(workers, newSCProxyWorker(proxyURL, interval))
	}
	return workers
}

// ---------------------------------------------------------------------------
// MediaWiki API response types
// ---------------------------------------------------------------------------

type scSectionsResponse struct {
	Parse struct {
		Sections []struct {
			Index  string `json:"index"`
			Line   string `json:"line"`
			Level  string `json:"level"`
			Number string `json:"number"`
		} `json:"sections"`
	} `json:"parse"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

type scTextResponse struct {
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

type scJob struct {
	studioID     string
	wikipediaURL string
	pageTitle    string
}

var scURLRe = regexp.MustCompile(`/wiki/(.+)$`)

func scExtractPageTitle(wikiURL string) string {
	m := scURLRe.FindStringSubmatch(wikiURL)
	if len(m) < 2 {
		return ""
	}
	decoded, err := url.PathUnescape(m[1])
	if err != nil {
		return m[1]
	}
	return decoded
}

// ---------------------------------------------------------------------------
// Section blacklist — sections to EXCLUDE
// ---------------------------------------------------------------------------

var scHTMLTagRe = regexp.MustCompile(`<[^>]*>`)

// Exact section names to exclude (case-insensitive)
var excludedSectionNames = map[string]bool{
	// Works/productions lists
	"works":                       true,
	"television series":           true,
	"films":                       true,
	"video games":                 true,
	"productions":                 true,
	"filmography":                 true,
	"tv series":                   true,
	"ovas":                        true,
	"products":                    true,
	"anime":                       true,
	"games":                       true,
	"original video animations":   true,
	"original net animations":     true,
	"anime television series":     true,
	"television":                  true,
	"ova/onas":                    true,
	"music videos":                true,
	"anime films":                 true,
	"feature films":               true,
	"ovas/onas":                   true,
	"animated films":              true,
	"other productions":           true,
	"list of works":               true,
	"games published":             true,
	"games developed":             true,
	"movies":                      true,
	"short films":                 true,
	"manga":                       true,
	"light novels":                true,
	"co-productions":              true,
	"animated works":              true,
	"book series":                 true,
	"produced series":             true,
	"animated series":             true,
	"list of productions":         true,
	"list of games":               true,
	"theatrical films":            true,
	"live-action films":           true,
	"live-action series":          true,
	"outsourced productions":      true,
	"film series":                 true,
	"cooperative works":           true,
	"manga magazines":             true,
	"characters":                  true,
	"other media":                 true,
	"tokusatsu":                   true,
	"specials":                    true,
	"tv specials":                 true,
	"ova":                         true,
	"drama cds":                   true,
	"commercials":                 true,
	"light novel imprints":        true,
	"labels":                      true,
	"magazines":                   true,
	"publications":                true,
	"releases":                    true,
	"published games":             true,
	"other works":                 true,
	"original productions":        true,
	"tv anime":                    true,
	"onas":                        true,
	"original video animation":    true,
	"original net animation":      true,
	"novels":                      true,
	"children's books":            true,
	"video game animation":        true,
	"animated":                    true,
	"series":                      true,
	"cancelled projects":          true,
	"cancelled":                   true,
	"programming":                 true,
	"other":                       true,
	"others":                      true,
	"film":                        true,
	"animation":                   true,
	"music":                       true,
	"sports":                      true,
	"titles":                      true,
	"imprints":                    true,
	"magazines published":         true,
	"products and services":       true,
	"product lines":               true,
	"television films and specials": true,
	"television specials":         true,
	"television dramas":           true,
	"live-action":                 true,
	"dramas":                      true,
	"original video animations (ovas)": true,
	"original net animations (onas)":   true,
	"ovas and oads":               true,
	"other tv series":             true,
	"comics":                      true,
	"gross outsource works":       true,
	"gross outsource":             true,
	"highest-grossing films":      true,
	"currently licensed":          true,
	"formerly licensed":           true,
	"entertainment":               true,
	"hentai":                      true,

	// References/meta
	"references":        true,
	"external links":    true,
	"see also":          true,
	"notes":             true,
	"further reading":   true,
	"sources":           true,
	"citations":         true,
	"bibliography":      true,
	"footnotes":         true,
	"works cited":       true,
	"explanatory notes": true,
	"general references": true,
	"gallery":           true,

	// Staff/people lists
	"representative staff":  true,
	"notable staff":         true,
	"animation producers":   true,
	"directors":             true,
	"artists":               true,
	"animators":             true,
	"notable artists":       true,
	"music artists":         true,
	"production staff":      true,
	"staff":                 true,
	"key people":            true,
	"former artists":        true,
	"individuals":           true,
	"chairmen":              true,
	"ceos":                  true,
	"board of directors":    true,
	"executive management":  true,
	"former members":        true,
	"voice actors":          true,
	"actresses":             true,
	"chief executive officers": true,
	"former representative staff": true,

	// Geographic subsections (country/region lists)
	"united states":  true,
	"japan":          true,
	"china":          true,
	"north america":  true,
	"europe":         true,
	"asia":           true,
	"brazil":         true,
	"mexico":         true,
	"india":          true,
	"indonesia":      true,
	"thailand":       true,
	"australia":      true,
	"canada":         true,
	"turkey":         true,
	"russia":         true,
	"africa":         true,
	"international":  true,

	// Decades (usually sub-sections of filmography)
	"1940s": true,
	"1950s": true,
	"1960s": true,
	"1970s": true,
	"1980s": true,
	"1990s": true,
	"2000s": true,
	"2010s": true,
	"2020s": true,

	// Current/Former subsections (usually under subsidiaries or staff)
	"current": true,
	"former":  true,
	"former subsidiaries": true,

	// Gender categories (for voice actor lists)
	"male":   true,
	"female": true,

	// Platform-specific game lists
	"playstation":       true,
	"gamecube":          true,
	"game boy":          true,
	"arcade":            true,
	"arcade video games": true,

	// Misc works-adjacent
	"episodes":          true,
	"upcoming":          true,
	"defunct magazines":  true,
	"monthly":           true,
	"seasonal":          true,
	"variety shows":     true,
	"developed":         true,
	"published":         true,
	"news":              true,
	"other services":    true,
	"other magazines":   true,
}

// Keyword-based exclusion fallback: if section name CONTAINS any of these
var excludedKeywords = []string{
	"filmography",
	"list of",
	"games published",
	"games developed",
	"licensed",
	"voice actor",
	"osamu tezuka works",
	"non-osamu tezuka works",
}

func isExcludedSection(sectionLine string) bool {
	lower := strings.ToLower(strings.TrimSpace(sectionLine))
	lower = scHTMLTagRe.ReplaceAllString(lower, "")
	lower = strings.TrimSpace(lower)

	// Check exact match
	if excludedSectionNames[lower] {
		return true
	}

	// Check keyword-based exclusion
	for _, kw := range excludedKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	return false
}

// stripInfoboxes removes <table class="infobox ...">...</table> blocks from HTML.
var infoboxRe = regexp.MustCompile(`(?is)<table[^>]*class="[^"]*infobox[^"]*"[^>]*>.*?</table>`)

// stripCiteErrors removes cite error messages from HTML.
var citeErrorRe = regexp.MustCompile(`(?is)<[^>]*class="[^"]*(?:mw-ext-cite-error|cite-error)[^"]*"[^>]*>.*?</[^>]+>`)
var citeErrorTextRe = regexp.MustCompile(`(?i)Cite error:[^\n<]*`)

func cleanHTML(html string) string {
	html = infoboxRe.ReplaceAllString(html, "")
	html = citeErrorRe.ReplaceAllString(html, "")
	html = citeErrorTextRe.ReplaceAllString(html, "")
	return strings.TrimSpace(html)
}

// ---------------------------------------------------------------------------
// Fetch logic
// ---------------------------------------------------------------------------

type scResult struct {
	studioID     string
	wikipediaURL string
	html         string
	err          error
}

func scFetchWithRetry(worker *scProxyWorker, rawURL string, maxAttempts int) (*http.Response, error) {
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
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body[:scMin(len(body), 200)]))
		}
	}
	return nil, fmt.Errorf("failed after %d attempts", maxAttempts)
}

func fetchStudioContent(worker *scProxyWorker, job scJob, requestCount *int64) scResult {
	result := scResult{
		studioID:     job.studioID,
		wikipediaURL: job.wikipediaURL,
	}

	// Step 1: Get sections list
	params := url.Values{}
	params.Set("action", "parse")
	params.Set("page", job.pageTitle)
	params.Set("prop", "sections")
	params.Set("format", "json")
	sectionsURL := scAPIBase + "?" + params.Encode()

	resp, err := scFetchWithRetry(worker, sectionsURL, 4)
	atomic.AddInt64(requestCount, 1)
	if err != nil {
		result.err = fmt.Errorf("sections request: %w", err)
		return result
	}

	var sectionsResp scSectionsResponse
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

	// Step 2: Fetch lead section (section 0 — summary before any headings)
	leadParams := url.Values{}
	leadParams.Set("action", "parse")
	leadParams.Set("page", job.pageTitle)
	leadParams.Set("section", "0")
	leadParams.Set("prop", "text")
	leadParams.Set("format", "json")
	leadParams.Set("disabletoc", "true")
	leadURL := scAPIBase + "?" + leadParams.Encode()

	leadResp, err := scFetchWithRetry(worker, leadURL, 4)
	atomic.AddInt64(requestCount, 1)
	if err != nil {
		result.err = fmt.Errorf("lead section request: %w", err)
		return result
	}

	var leadText scTextResponse
	if err := json.NewDecoder(leadResp.Body).Decode(&leadText); err != nil {
		leadResp.Body.Close()
		result.err = fmt.Errorf("lead section decode: %w", err)
		return result
	}
	leadResp.Body.Close()

	if leadText.Error != nil {
		result.err = fmt.Errorf("lead section API error: %s — %s", leadText.Error.Code, leadText.Error.Info)
		return result
	}

	var htmlParts []string
	if leadContent := cleanHTML(leadText.Parse.Text.Content); leadContent != "" {
		htmlParts = append(htmlParts, leadContent)
	}

	// Step 3: Filter sections — keep only non-blacklisted top-level (L2) sections
	// When we include a L2 section, we fetch it (which includes its subsections).
	// We only check L2 sections; subsections are included via the parent fetch.
	var includedIndices []string
	for _, sec := range sectionsResp.Parse.Sections {
		if sec.Level != "2" {
			continue // Only check top-level sections
		}
		if isExcludedSection(sec.Line) {
			continue
		}
		includedIndices = append(includedIndices, sec.Index)
	}

	// Step 4: Fetch HTML for each included section
	for _, idx := range includedIndices {
		secParams := url.Values{}
		secParams.Set("action", "parse")
		secParams.Set("page", job.pageTitle)
		secParams.Set("section", idx)
		secParams.Set("prop", "text")
		secParams.Set("format", "json")
		secParams.Set("disabletoc", "true")
		secURL := scAPIBase + "?" + secParams.Encode()

		secResp, err := scFetchWithRetry(worker, secURL, 4)
		atomic.AddInt64(requestCount, 1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to fetch section index %s: %v\n", idx, err)
			continue
		}

		var secText scTextResponse
		if err := json.NewDecoder(secResp.Body).Decode(&secText); err != nil {
			secResp.Body.Close()
			fmt.Fprintf(os.Stderr, "  warning: failed to decode section index %s: %v\n", idx, err)
			continue
		}
		secResp.Body.Close()

		if secText.Error != nil {
			fmt.Fprintf(os.Stderr, "  warning: API error for section index %s: %s — %s\n", idx, secText.Error.Code, secText.Error.Info)
			continue
		}

		if content := cleanHTML(secText.Parse.Text.Content); content != "" {
			htmlParts = append(htmlParts, content)
		}
	}

	if len(htmlParts) == 0 {
		result.err = fmt.Errorf("no content found")
		return result
	}

	result.html = strings.Join(htmlParts, "\n")
	return result
}

// ---------------------------------------------------------------------------
// CSV loading
// ---------------------------------------------------------------------------

func loadStudioCSV(filename string) ([]scJob, error) {
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
	for _, need := range []string{"studio_id", "wikipedia_en"} {
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

	var jobs []scJob
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "CSV read error: %v\n", err)
			continue
		}
		studioID := get(row, "studio_id")
		wikiURL := get(row, "wikipedia_en")
		if studioID == "" || wikiURL == "" {
			continue
		}
		title := scExtractPageTitle(wikiURL)
		if title == "" {
			fmt.Fprintf(os.Stderr, "Could not extract page title from %q (studio_id=%s)\n", wikiURL, studioID)
			continue
		}
		jobs = append(jobs, scJob{
			studioID:     studioID,
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
	delay := flag.Float64("delay", scDefaultDelay, "Min seconds between requests PER PROXY")
	numWorkers := flag.Int("workers", 0, "Number of proxy workers (0 = all available)")
	noProxy := flag.Bool("no-proxy", false, "Use direct connections (no proxy pool)")
	flag.Parse()

	// Load input
	jobs, err := loadStudioCSV(*inFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading CSV: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d studios with Wikipedia URLs\n", len(jobs))

	// Build proxy pool (or direct workers)
	var proxyPool []*scProxyWorker
	if *noProxy {
		interval := time.Duration(*delay * float64(time.Second))
		proxyPool = []*scProxyWorker{{
			client:      &http.Client{Timeout: 30 * time.Second},
			minInterval: interval,
		}}
	} else {
		proxyPool = buildSCProxies(scProxyExample, scProxyPortLow, scProxyPortHigh)
		if *delay != scDefaultDelay {
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
	jobCh := make(chan scJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	// Results
	var mu sync.Mutex
	var results []scResult
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

				fmt.Fprintf(os.Stderr, "[STATS] %d/%d studios (%.0f%%) | %d found | %d reqs (%.1f req/s) | %.0fs elapsed\n",
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
		go func(workerID int, proxy *scProxyWorker) {
			defer wg.Done()
			for job := range jobCh {
				result := fetchStudioContent(proxy, job, &totalRequests)

				mu.Lock()
				results = append(results, result)
				mu.Unlock()

				done := atomic.AddInt64(&completedJobs, 1)

				if result.err != nil {
					fmt.Fprintf(os.Stderr, "[w%d] (%d/%d) %s → %v\n",
						workerID, done, totalJobs, job.pageTitle, result.err)
				} else {
					htmlLen := len(result.html)
					fmt.Fprintf(os.Stderr, "[w%d] (%d/%d) %s → %d chars\n",
						workerID, done, totalJobs, job.pageTitle, htmlLen)
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
		} else if strings.Contains(r.err.Error(), "no content found") {
			notFound++
		} else {
			errored++
		}
	}

	fmt.Fprintf(os.Stderr, "\nDone: %d found, %d no content, %d errors | %d reqs in %.1fs (%.1f req/s)\n",
		found, notFound, errored, reqs, elapsed.Seconds(), float64(reqs)/elapsed.Seconds())

	// Sort results by studio_id for deterministic output
	sort.Slice(results, func(i, j int) bool {
		return results[i].studioID < results[j].studioID
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
	w.Write([]string{"studio_id", "wikipedia_url", "content_html"})

	written := 0
	for _, r := range results {
		if r.err != nil {
			continue
		}
		w.Write([]string{r.studioID, r.wikipediaURL, r.html})
		written++
	}
	w.Flush()

	dest := *outFile
	if dest == "" || dest == "-" {
		dest = "stdout"
	}
	fmt.Fprintf(os.Stderr, "Wrote %d rows → %s\n", written, dest)
}

func scMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
