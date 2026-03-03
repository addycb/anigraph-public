package main

// wikipedia_studio_sections_audit.go
//
// Audits all studio Wikipedia pages and prints all unique section names with counts.
// Uses rotating proxies for concurrency (same proxy pool as scrape_incremental).
//
// Build & run:
//   go build -o wikipedia_studio_sections_audit wikipedia_studio_sections_audit.go
//   psql "$DATABASE_URL" -t -A -F',' \
//     -c "SELECT id AS studio_id, wikipedia_en FROM studio
//         WHERE wikidata_qid IS NOT NULL AND wikipedia_en IS NOT NULL ORDER BY id" \
//     | sed '1i studio_id,wikipedia_en' \
//     | ./wikipedia_studio_sections_audit -in - -n 0

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
	sauditRawBase        = "https://en.wikipedia.org/w/index.php"
	sauditAgent          = "AniGraph/1.0 (addisonbaum@gmail.com)"
	sauditProxyPortStart = 10001
	sauditProxyPortEnd   = 10100
	sauditMinSecPerProxy = 4.0
	sauditExampleProxy   = "dc.decodo.com:10001:spawee4ylf:yIzsp7~aeb7Yrz87RQ"
	sauditMaxRetries     = 3
)

type sauditJob struct {
	studioID     string
	wikipediaURL string
	pageTitle    string
}

type sauditSectionInfo struct {
	name  string
	level string
}

type sauditResult struct {
	pageTitle string
	sections  []sauditSectionInfo
	err       error
}

// sauditProxyWorker manages rate limiting for a single proxy
type sauditProxyWorker struct {
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func newSauditProxyWorker(proxyURL string) *sauditProxyWorker {
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
	return &sauditProxyWorker{
		client:      &http.Client{Transport: transport, Timeout: 30 * time.Second},
		minInterval: time.Duration(sauditMinSecPerProxy * float64(time.Second)),
	}
}

func (pw *sauditProxyWorker) Do(req *http.Request) (*http.Response, error) {
	pw.mu.Lock()
	elapsed := time.Since(pw.lastUsed)
	if elapsed < pw.minInterval {
		time.Sleep(pw.minInterval - elapsed)
	}
	pw.lastUsed = time.Now()
	pw.mu.Unlock()
	return pw.client.Do(req)
}

func buildSauditProxies(example string, start, end int) []string {
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

var sauditURLRe = regexp.MustCompile(`/wiki/(.+)$`)

func sauditExtractTitle(wikiURL string) string {
	m := sauditURLRe.FindStringSubmatch(wikiURL)
	if len(m) < 2 {
		return ""
	}
	decoded, err := url.PathUnescape(m[1])
	if err != nil {
		return m[1]
	}
	return decoded
}

// sauditSectionRe matches wikitext headings like == Plot == or === Development ===
var sauditSectionRe = regexp.MustCompile(`(?m)^(={2,})\s*(.+?)\s*=+\s*$`)

func fetchStudioSections(worker *sauditProxyWorker, job sauditJob) sauditResult {
	result := sauditResult{pageTitle: job.pageTitle}

	rawURL := sauditRawBase + "?title=" + url.QueryEscape(job.pageTitle) + "&action=raw"

	for attempt := 0; attempt <= sauditMaxRetries; attempt++ {
		req, err := http.NewRequest("GET", rawURL, nil)
		if err != nil {
			result.err = err
			return result
		}
		req.Header.Set("User-Agent", sauditAgent)

		resp, err := worker.Do(req)
		if err != nil {
			result.err = err
			if attempt < sauditMaxRetries {
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
			fmt.Fprintf(os.Stderr, "[429] %s — waiting %v before retry %d/%d\n", job.pageTitle, wait, attempt+1, sauditMaxRetries)
			time.Sleep(wait)
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			result.err = fmt.Errorf("HTTP %d", resp.StatusCode)
			if attempt < sauditMaxRetries {
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

		matches := sauditSectionRe.FindAllSubmatch(body, -1)
		for _, m := range matches {
			level := strconv.Itoa(len(m[1]))
			name := string(m[2])
			result.sections = append(result.sections, sauditSectionInfo{
				name:  name,
				level: level,
			})
		}
		return result
	}

	return result
}

func loadSauditCSV(filename string, maxRows int) ([]sauditJob, error) {
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

	var jobs []sauditJob
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		sid := get(row, "studio_id")
		wurl := get(row, "wikipedia_en")
		if sid == "" || wurl == "" {
			continue
		}
		title := sauditExtractTitle(wurl)
		if title == "" {
			continue
		}
		jobs = append(jobs, sauditJob{studioID: sid, wikipediaURL: wurl, pageTitle: title})
		if maxRows > 0 && len(jobs) >= maxRows {
			break
		}
	}
	return jobs, nil
}

func main() {
	inFile := flag.String("in", "", "Input CSV with studio_id,wikipedia_en columns")
	sampleSize := flag.Int("n", 100, "Number of studios to sample (0 = all)")
	useProxies := flag.Bool("proxies", true, "Use rotating proxies")
	numProxies := flag.Int("num-proxies", 100, "Number of proxies to use (from port 10001 up)")
	directWorkers := flag.Int("workers", 10, "Concurrent workers (only used with -proxies=false)")
	flag.Parse()

	jobs, err := loadSauditCSV(*inFile, *sampleSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d studios to audit\n", len(jobs))

	jobCh := make(chan sauditJob, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var mu sync.Mutex
	var results []sauditResult
	var completed int64
	total := int64(len(jobs))

	var wg sync.WaitGroup

	if *useProxies {
		endPort := sauditProxyPortStart + *numProxies - 1
		if endPort > sauditProxyPortEnd {
			endPort = sauditProxyPortEnd
		}
		proxyURLs := buildSauditProxies(sauditExampleProxy, sauditProxyPortStart, endPort)
		fmt.Fprintf(os.Stderr, "Using %d proxies\n", len(proxyURLs))

		var workers []*sauditProxyWorker
		for _, p := range proxyURLs {
			workers = append(workers, newSauditProxyWorker(p))
		}

		for i, worker := range workers {
			wg.Add(1)
			go func(id int, w *sauditProxyWorker) {
				defer wg.Done()
				for job := range jobCh {
					res := fetchStudioSections(w, job)

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
		directClient := newSauditProxyWorker("")
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
					res := fetchStudioSections(directClient, job)

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
		levels    map[string]int
	}
	sectionCounts := make(map[string]*tally)

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
	fmt.Printf("\n=== Studio Section Name Audit (%d pages successfully fetched) ===\n\n", successes)
	fmt.Printf("%-5s  %-8s  %s\n", "COUNT", "LEVELS", "SECTION NAME")
	fmt.Printf("%-5s  %-8s  %s\n", "-----", "------", "------------")
	for _, e := range entries {
		var lvlParts []string
		for l, c := range e.levels {
			lvlParts = append(lvlParts, fmt.Sprintf("L%s:%d", l, c))
		}
		sort.Strings(lvlParts)
		fmt.Printf("%-5d  %-8s  %s\n", e.count, strings.Join(lvlParts, ","), e.name)
	}

	fmt.Printf("\nTotal unique section names: %d\n", len(entries))
}
