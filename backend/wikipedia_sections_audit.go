package main

// wikipedia_sections_audit.go
//
// Audits all anime Wikipedia pages and prints all unique section names with counts.
// Uses rotating proxies for concurrency (same proxy pool as scrape_incremental).
//
// Build & run:
//   go build -o wikipedia_sections_audit wikipedia_sections_audit.go
//   ./wikipedia_sections_audit -in anime_wikipedia_audit.csv -n 0

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	auditRawBase         = "https://en.wikipedia.org/w/index.php"
	auditAgent           = "AniGraph/1.0 (addisonbaum@gmail.com)"
	auditProxyPortStart  = 10001
	auditProxyPortEnd    = 10100
	auditMinSecPerProxy  = 4.0
	auditExampleProxy    = "dc.decodo.com:10001:spawee4ylf:yIzsp7~aeb7Yrz87RQ"
	auditMaxRetries      = 3
)

type auditJob struct {
	anilistID    string
	wikipediaURL string
	pageTitle    string
}

type sectionInfo struct {
	name  string
	level string
}

type auditResult struct {
	pageTitle string
	sections  []sectionInfo
	err       error
}

// auditProxyWorker manages rate limiting for a single proxy
type auditProxyWorker struct {
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func newAuditProxyWorker(proxyURL string) *auditProxyWorker {
	proxy, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxy),
		MaxIdleConnsPerHost: 10,
	}
	return &auditProxyWorker{
		client:      &http.Client{Transport: transport, Timeout: 30 * time.Second},
		minInterval: time.Duration(auditMinSecPerProxy * float64(time.Second)),
	}
}

func (pw *auditProxyWorker) Do(req *http.Request) (*http.Response, error) {
	pw.mu.Lock()
	elapsed := time.Since(pw.lastUsed)
	if elapsed < pw.minInterval {
		time.Sleep(pw.minInterval - elapsed)
	}
	pw.lastUsed = time.Now()
	pw.mu.Unlock()
	return pw.client.Do(req)
}

func buildAuditProxies(example string, start, end int) []string {
	parts := strings.Split(example, ":")
	if len(parts) < 4 {
		panic("Proxy format must be host:port:username:password")
	}
	host := parts[0]
	username := parts[2]
	password := parts[3]

	var proxies []string
	for port := start; port <= end; port++ {
		proxies = append(proxies, fmt.Sprintf("http://%s:%s@%s:%d", username, password, host, port))
	}
	return proxies
}

var auditURLRe = regexp.MustCompile(`/wiki/(.+)$`)

func auditExtractTitle(wikiURL string) string {
	m := auditURLRe.FindStringSubmatch(wikiURL)
	if len(m) < 2 {
		return ""
	}
	decoded, err := url.PathUnescape(m[1])
	if err != nil {
		return m[1]
	}
	return decoded
}

// sectionRe matches wikitext headings like == Plot == or === Development ===
var sectionRe = regexp.MustCompile(`(?m)^(={2,})\s*(.+?)\s*=+\s*$`)

func fetchSections(worker *auditProxyWorker, job auditJob) auditResult {
	result := auditResult{pageTitle: job.pageTitle}

	// Fetch raw wikitext via action=raw (not the API, just plain text)
	rawURL := auditRawBase + "?title=" + url.QueryEscape(job.pageTitle) + "&action=raw"

	for attempt := 0; attempt <= auditMaxRetries; attempt++ {
		req, err := http.NewRequest("GET", rawURL, nil)
		if err != nil {
			result.err = err
			return result
		}
		req.Header.Set("User-Agent", auditAgent)

		resp, err := worker.Do(req)
		if err != nil {
			result.err = err
			if attempt < auditMaxRetries {
				time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
				continue
			}
			return result
		}

		if resp.StatusCode == 429 {
			resp.Body.Close()
			wait := time.Duration(5*(attempt+1)) * time.Second
			if rh := resp.Header.Get("Retry-After"); rh != "" {
				if secs, err := time.ParseDuration(rh + "s"); err == nil {
					wait = secs
				}
			}
			fmt.Fprintf(os.Stderr, "[429] %s — waiting %v before retry %d/%d\n", job.pageTitle, wait, attempt+1, auditMaxRetries)
			time.Sleep(wait)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			result.err = fmt.Errorf("HTTP %d", resp.StatusCode)
			if attempt < auditMaxRetries {
				time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
				continue
			}
			return result
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			result.err = fmt.Errorf("read body: %w", err)
			return result
		}

		// Parse == Section == headings from raw wikitext
		matches := sectionRe.FindAllSubmatch(body, -1)
		for _, m := range matches {
			level := strconv.Itoa(len(m[1])) // number of '=' signs
			name := string(m[2])
			result.sections = append(result.sections, sectionInfo{
				name:  name,
				level: level,
			})
		}
		return result
	}

	return result
}

func loadAuditCSV(filename string, maxRows int) ([]auditJob, error) {
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

	var jobs []auditJob
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		aid := get(row, "anilist_id")
		wurl := get(row, "wikipedia_en")
		if aid == "" || wurl == "" {
			continue
		}
		title := auditExtractTitle(wurl)
		if title == "" {
			continue
		}
		jobs = append(jobs, auditJob{anilistID: aid, wikipediaURL: wurl, pageTitle: title})
		if maxRows > 0 && len(jobs) >= maxRows {
			break
		}
	}
	return jobs, nil
}

func main() {
	inFile := flag.String("in", "", "Input CSV with anilist_id,wikipedia_en columns")
	sampleSize := flag.Int("n", 100, "Number of anime to sample (0 = all)")
	useProxies := flag.Bool("proxies", true, "Use rotating proxies (same as scrape_incremental)")
	numProxies := flag.Int("num-proxies", 100, "Number of proxies to use (from port 10001 up)")
	directWorkers := flag.Int("workers", 10, "Concurrent workers (only used with -proxies=false)")
	flag.Parse()

	jobs, err := loadAuditCSV(*inFile, *sampleSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d anime to audit\n", len(jobs))

	jobCh := make(chan auditJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var mu sync.Mutex
	var results []auditResult
	var completed int64
	total := int64(len(jobs))

	var wg sync.WaitGroup

	if *useProxies {
		// Build proxy workers (same pool as scrape_incremental)
		endPort := auditProxyPortStart + *numProxies - 1
		if endPort > auditProxyPortEnd {
			endPort = auditProxyPortEnd
		}
		proxyURLs := buildAuditProxies(auditExampleProxy, auditProxyPortStart, endPort)
		fmt.Fprintf(os.Stderr, "Using %d proxies\n", len(proxyURLs))

		var workers []*auditProxyWorker
		for _, p := range proxyURLs {
			workers = append(workers, newAuditProxyWorker(p))
		}

		// One goroutine per proxy worker
		for i, worker := range workers {
			wg.Add(1)
			go func(id int, w *auditProxyWorker) {
				defer wg.Done()
				for job := range jobCh {
					res := fetchSections(w, job)

					mu.Lock()
					results = append(results, res)
					mu.Unlock()

					done := atomic.AddInt64(&completed, 1)
					if res.err != nil {
						fmt.Fprintf(os.Stderr, "(%d/%d) [proxy-%d] %s → ERROR: %v\n", done, total, id, job.pageTitle, res.err)
					} else {
						fmt.Fprintf(os.Stderr, "(%d/%d) [proxy-%d] %s → %d sections\n", done, total, id, job.pageTitle, len(res.sections))
					}
				}
			}(i, worker)
		}
	} else {
		// Direct mode (no proxies) - original behavior
		directClient := newAuditProxyWorker("") // no proxy
		directClient.client = &http.Client{Timeout: 30 * time.Second}
		directClient.minInterval = 100 * time.Millisecond

		poolSize := *directWorkers
		if poolSize > len(jobs) {
			poolSize = len(jobs)
		}

		for i := 0; i < poolSize; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobCh {
					res := fetchSections(directClient, job)

					mu.Lock()
					results = append(results, res)
					mu.Unlock()

					done := atomic.AddInt64(&completed, 1)
					if res.err != nil {
						fmt.Fprintf(os.Stderr, "(%d/%d) %s → ERROR: %v\n", done, total, job.pageTitle, res.err)
					} else {
						fmt.Fprintf(os.Stderr, "(%d/%d) %s → %d sections\n", done, total, job.pageTitle, len(res.sections))
					}
				}
			}()
		}
	}

	wg.Wait()

	// Tally section names (case-insensitive, but preserve original casing from first occurrence)
	type tally struct {
		canonical string
		count     int
		levels    map[string]int // level → count
	}
	sectionCounts := make(map[string]*tally) // lowercase key → tally

	for _, r := range results {
		if r.err != nil {
			continue
		}
		for _, sec := range r.sections {
			stripped := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(sec.name, "")
			stripped = strings.TrimSpace(stripped)
			key := strings.ToLower(stripped)
			if key == "" {
				continue
			}
			if t, ok := sectionCounts[key]; ok {
				t.count++
				t.levels[sec.level]++
			} else {
				sectionCounts[key] = &tally{
					canonical: stripped,
					count:     1,
					levels:    map[string]int{sec.level: 1},
				}
			}
		}
	}

	// Sort by count descending
	type entry struct {
		name   string
		count  int
		levels map[string]int
	}
	var entries []entry
	for _, t := range sectionCounts {
		entries = append(entries, entry{name: t.canonical, count: t.count, levels: t.levels})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].count > entries[j].count
	})

	// Print results
	successes := 0
	for _, r := range results {
		if r.err == nil {
			successes++
		}
	}
	fmt.Printf("\n=== Section Name Audit (%d pages successfully fetched) ===\n\n", successes)
	fmt.Printf("%-5s  %-8s  %s\n", "COUNT", "LEVELS", "SECTION NAME")
	fmt.Printf("%-5s  %-8s  %s\n", "-----", "------", "------------")
	for _, e := range entries {
		// Format levels
		var lvlParts []string
		for l, c := range e.levels {
			lvlParts = append(lvlParts, fmt.Sprintf("L%s:%d", l, c))
		}
		sort.Strings(lvlParts)
		fmt.Printf("%-5d  %-8s  %s\n", e.count, strings.Join(lvlParts, ","), e.name)
	}

	fmt.Printf("\nTotal unique section names: %d\n", len(entries))
}
