package main

// sakugabooru_posts.go
//
// Fetches Sakugabooru posts for all known anime copyright tags and staff artist
// tags using a pool of proxy workers for true parallelism. Each proxy gets its
// own rate-limited HTTP client, same pattern as scrape_incremental.go.
//
// Writes three CSV files to -out directory:
//   sakugabooru_posts.csv       — one row per unique post
//   sakugabooru_anime_posts.csv — (anilist_id, post_id) links
//   sakugabooru_staff_posts.csv — (staff_id,   post_id) links

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
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	sakugaPostBase  = "https://www.sakugabooru.com"
	sakugaPostAgent = "AniGraph-SakugabooruPosts/1.0 (anigraph.xyz)"
	postsPerPage    = 100

	// Proxy config — same as scrape_incremental.go
	sakugaProxyExample  = "dc.decodo.com:10001:spawee4ylf:yIzsp7~aeb7Yrz87RQ"
	sakugaProxyPortLow  = 10001
	sakugaProxyPortHigh = 10100

	// Per-proxy delay: Sakugabooru is smaller than AniList, so we're
	// conservative. 2s per proxy × 100 proxies = ~50 req/s max throughput.
	defaultPerProxyDelay = 2.0
)

// ---------------------------------------------------------------------------
// Proxy worker — one per proxy IP, each with its own rate limiter
// ---------------------------------------------------------------------------

type sakugaProxyWorker struct {
	proxyURL    string
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func newSakugaProxyWorker(proxyURL string, minInterval time.Duration) *sakugaProxyWorker {
	proxy, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxy),
		MaxIdleConnsPerHost: 4,
	}
	return &sakugaProxyWorker{
		proxyURL:    proxyURL,
		client:      &http.Client{Transport: transport, Timeout: 30 * time.Second},
		minInterval: minInterval,
	}
}

func (w *sakugaProxyWorker) get(rawURL string) (*http.Response, error) {
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
	req.Header.Set("User-Agent", sakugaPostAgent)
	return w.client.Do(req)
}

func buildSakugaProxies(example string, portLow, portHigh int) []*sakugaProxyWorker {
	parts := strings.Split(example, ":")
	if len(parts) < 4 {
		panic("Proxy format must be host:port:username:password")
	}
	host := parts[0]
	username := parts[2]
	password := parts[3]

	interval := time.Duration(defaultPerProxyDelay * float64(time.Second))
	var workers []*sakugaProxyWorker
	for port := portLow; port <= portHigh; port++ {
		proxyURL := fmt.Sprintf("http://%s:%s@%s:%d", username, password, host, port)
		workers = append(workers, newSakugaProxyWorker(proxyURL, interval))
	}
	return workers
}

// ---------------------------------------------------------------------------
// Sakugabooru post structure
// ---------------------------------------------------------------------------

type sakugaPost struct {
	ID         int    `json:"id"`
	FileURL    string `json:"file_url"`
	FileExt    string `json:"file_ext"`
	PreviewURL string `json:"preview_url"`
	Source     string `json:"source"`
	Rating     string `json:"rating"`
}

// ---------------------------------------------------------------------------
// Tag → entity links
// ---------------------------------------------------------------------------

type entityLink struct {
	entityType string // "anime" or "staff"
	entityID   string // anilist_id or staff_id (as string)
	tag        string
}

type tagGroup struct {
	tag   string
	links []entityLink
}

// ---------------------------------------------------------------------------
// CSV input: header-based column detection
// ---------------------------------------------------------------------------

func loadTagCSV(filename, entityType, idCol, tagCol, foundCol string) ([]entityLink, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(bufio.NewReader(f))
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
	for _, need := range []string{idCol, tagCol, foundCol} {
		if _, ok := colIdx[need]; !ok {
			return nil, fmt.Errorf("column %q not found in %s (columns: %v)", need, filename, header)
		}
	}

	get := func(row []string, col string) string {
		i := colIdx[col]
		if i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	var links []entityLink
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "CSV read error in %s: %v\n", filename, err)
			continue
		}
		if get(row, foundCol) != "1" {
			continue
		}
		tag := get(row, tagCol)
		id := get(row, idCol)
		if tag == "" || id == "" {
			continue
		}
		links = append(links, entityLink{entityType: entityType, entityID: id, tag: tag})
	}
	return links, nil
}

// ---------------------------------------------------------------------------
// Post fetching with pagination (uses a specific proxy worker)
// ---------------------------------------------------------------------------

func fetchPostPage(worker *sakugaProxyWorker, tag string, page int) ([]sakugaPost, error) {
	const maxAttempts = 4

	params := url.Values{}
	params.Set("tags", tag)
	params.Set("limit", strconv.Itoa(postsPerPage))
	params.Set("page", strconv.Itoa(page))
	rawURL := sakugaPostBase + "/post.json?" + params.Encode()

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*3) * time.Second
			fmt.Fprintf(os.Stderr, "  retry %d in %.0fs (tag=%q page=%d)...\n", attempt+1, backoff.Seconds(), tag, page)
			time.Sleep(backoff)
		}

		resp, err := worker.get(rawURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  request error (tag=%q page=%d): %v\n", tag, page, err)
			continue
		}

		switch resp.StatusCode {
		case 429:
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "  rate limited (429), waiting 60s...\n")
			time.Sleep(60 * time.Second)
			continue
		case 200:
			// ok
		default:
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d (tag=%q page=%d): %s", resp.StatusCode, tag, page, string(body))
		}

		var posts []sakugaPost
		if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode error (tag=%q page=%d): %w", tag, page, err)
		}
		resp.Body.Close()
		return posts, nil
	}
	return nil, fmt.Errorf("failed after %d attempts (tag=%q page=%d)", maxAttempts, tag, page)
}

func fetchAllPostsForTag(worker *sakugaProxyWorker, tag string) ([]sakugaPost, error) {
	var all []sakugaPost
	for page := 1; ; page++ {
		posts, err := fetchPostPage(worker, tag, page)
		if err != nil {
			return all, err
		}
		all = append(all, posts...)
		if len(posts) < postsPerPage {
			break
		}
	}
	return all, nil
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	animeFile := flag.String("anime", "", "Anime tags CSV")
	staffFile := flag.String("staff", "", "Staff tags CSV")
	outDir := flag.String("out", ".", "Output directory for the three CSV files")
	delay := flag.Float64("delay", defaultPerProxyDelay, "Min seconds between requests PER PROXY")
	workers := flag.Int("workers", 0, "Number of proxy workers (0 = all available proxies)")
	flag.Parse()

	if *animeFile == "" && *staffFile == "" {
		fmt.Fprintln(os.Stderr, "usage: sakugabooru_posts -anime <file> -staff <file> [-out <dir>]")
		os.Exit(1)
	}

	// Build proxy pool
	allProxies := buildSakugaProxies(sakugaProxyExample, sakugaProxyPortLow, sakugaProxyPortHigh)

	// Override per-proxy delay if specified
	if *delay != defaultPerProxyDelay {
		interval := time.Duration(*delay * float64(time.Second))
		for _, w := range allProxies {
			w.minInterval = interval
		}
	}

	// Limit worker count if requested
	numWorkers := len(allProxies)
	if *workers > 0 && *workers < numWorkers {
		numWorkers = *workers
	}
	proxyPool := allProxies[:numWorkers]

	// ---- Load entity→tag mappings ----
	var allLinks []entityLink
	if *animeFile != "" {
		links, err := loadTagCSV(*animeFile, "anime", "anilist_id", "sakugabooru_tag", "found")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading anime tags from %s: %v\n", *animeFile, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Loaded %d anime tag mappings from %s\n", len(links), *animeFile)
		allLinks = append(allLinks, links...)
	}
	if *staffFile != "" {
		links, err := loadTagCSV(*staffFile, "staff", "staff_id", "sakugabooru_tag", "found")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading staff tags from %s: %v\n", *staffFile, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Loaded %d staff tag mappings from %s\n", len(links), *staffFile)
		allLinks = append(allLinks, links...)
	}

	// ---- Group by tag (deduplicate tag fetches) ----
	tagGroupMap := make(map[string]*tagGroup)
	for _, link := range allLinks {
		if _, ok := tagGroupMap[link.tag]; !ok {
			tagGroupMap[link.tag] = &tagGroup{tag: link.tag}
		}
		tagGroupMap[link.tag].links = append(tagGroupMap[link.tag].links, link)
	}

	var groups []*tagGroup
	for _, g := range tagGroupMap {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].tag < groups[j].tag })

	fmt.Fprintf(os.Stderr, "\n%d unique tags to fetch (%d proxy workers, %.1fs per-proxy delay, ~%.0f req/s max)\n\n",
		len(groups), numWorkers, *delay, float64(numWorkers) / *delay)

	startTime := time.Now()

	// ---- Job channel ----
	jobCh := make(chan *tagGroup, len(groups))
	for _, g := range groups {
		jobCh <- g
	}
	close(jobCh)

	// ---- Shared state ----
	var mu sync.Mutex
	allPosts := make(map[int]sakugaPost)
	animeLinks := make(map[string]map[int]bool)
	staffLinks := make(map[string]map[int]bool)

	var totalRequests int64
	var completedTags int64
	totalTags := int64(len(groups))

	// ---- Stats reporter ----
	stopStats := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				done := atomic.LoadInt64(&completedTags)
				reqs := atomic.LoadInt64(&totalRequests)
				elapsed := time.Since(startTime).Seconds()
				rate := float64(reqs) / elapsed
				mu.Lock()
				postCount := len(allPosts)
				mu.Unlock()
				fmt.Fprintf(os.Stderr, "[STATS] %d/%d tags (%.0f%%) | %d posts | %d reqs (%.1f req/s) | %.0fs elapsed\n",
					done, totalTags, float64(done)/float64(totalTags)*100,
					postCount, reqs, rate, elapsed)
			case <-stopStats:
				return
			}
		}
	}()

	// ---- Worker pool — one goroutine per proxy ----
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int, proxy *sakugaProxyWorker) {
			defer wg.Done()
			for group := range jobCh {
				posts, err := fetchAllPostsForTag(proxy, group.tag)

				// Count requests: 1 per page, minimum 1
				pages := len(posts)/postsPerPage + 1
				atomic.AddInt64(&totalRequests, int64(pages))

				if err != nil {
					fmt.Fprintf(os.Stderr, "[worker %d] tag %q error: %v (got %d partial posts)\n", workerID, group.tag, err, len(posts))
				}

				mu.Lock()
				for _, post := range posts {
					if _, exists := allPosts[post.ID]; !exists {
						allPosts[post.ID] = post
					}
					for _, link := range group.links {
						if link.entityType == "anime" {
							if animeLinks[link.entityID] == nil {
								animeLinks[link.entityID] = make(map[int]bool)
							}
							animeLinks[link.entityID][post.ID] = true
						} else {
							if staffLinks[link.entityID] == nil {
								staffLinks[link.entityID] = make(map[int]bool)
							}
							staffLinks[link.entityID][post.ID] = true
						}
					}
				}
				mu.Unlock()

				done := atomic.AddInt64(&completedTags, 1)

				entityDesc := make([]string, 0, len(group.links))
				for _, l := range group.links {
					entityDesc = append(entityDesc, l.entityType+":"+l.entityID)
				}
				fmt.Fprintf(os.Stderr, "[w%d] (%d/%d) tag:%s → %d posts → [%s]\n",
					workerID, done, totalTags, group.tag, len(posts), strings.Join(entityDesc, ", "))
			}
		}(i, proxyPool[i])
	}
	wg.Wait()
	close(stopStats)

	elapsed := time.Since(startTime)
	reqs := atomic.LoadInt64(&totalRequests)

	// Sort post IDs for deterministic output
	postIDs := make([]int, 0, len(allPosts))
	for id := range allPosts {
		postIDs = append(postIDs, id)
	}
	sort.Ints(postIDs)

	fmt.Fprintf(os.Stderr, "\nFetch complete: %d unique posts, %d requests in %.1fs (%.1f req/s)\n\n",
		len(postIDs), reqs, elapsed.Seconds(), float64(reqs)/elapsed.Seconds())

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create output dir: %v\n", err)
		os.Exit(1)
	}

	// ---- sakugabooru_posts.csv ----
	postsF, err := os.Create(filepath.Join(*outDir, "sakugabooru_posts.csv"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create posts CSV: %v\n", err)
		os.Exit(1)
	}
	pw := csv.NewWriter(postsF)
	pw.Write([]string{"post_id", "file_url", "preview_url", "source", "rating", "file_ext"})
	for _, id := range postIDs {
		p := allPosts[id]
		pw.Write([]string{strconv.Itoa(p.ID), p.FileURL, p.PreviewURL, p.Source, p.Rating, p.FileExt})
	}
	pw.Flush()
	postsF.Close()
	fmt.Fprintf(os.Stderr, "Wrote %d rows → sakugabooru_posts.csv\n", len(postIDs))

	// ---- sakugabooru_anime_posts.csv ----
	animePostF, err := os.Create(filepath.Join(*outDir, "sakugabooru_anime_posts.csv"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create anime posts CSV: %v\n", err)
		os.Exit(1)
	}
	apw := csv.NewWriter(animePostF)
	apw.Write([]string{"anilist_id", "post_id"})
	var animeIDs []string
	for id := range animeLinks {
		animeIDs = append(animeIDs, id)
	}
	sort.Strings(animeIDs)
	animePostCount := 0
	for _, anilistID := range animeIDs {
		pids := sortedKeys(animeLinks[anilistID])
		for _, pid := range pids {
			apw.Write([]string{anilistID, strconv.Itoa(pid)})
			animePostCount++
		}
	}
	apw.Flush()
	animePostF.Close()
	fmt.Fprintf(os.Stderr, "Wrote %d rows → sakugabooru_anime_posts.csv\n", animePostCount)

	// ---- sakugabooru_staff_posts.csv ----
	staffPostF, err := os.Create(filepath.Join(*outDir, "sakugabooru_staff_posts.csv"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create staff posts CSV: %v\n", err)
		os.Exit(1)
	}
	spw := csv.NewWriter(staffPostF)
	spw.Write([]string{"staff_id", "post_id"})
	var staffIDs []string
	for id := range staffLinks {
		staffIDs = append(staffIDs, id)
	}
	sort.Strings(staffIDs)
	staffPostCount := 0
	for _, staffID := range staffIDs {
		pids := sortedKeys(staffLinks[staffID])
		for _, pid := range pids {
			spw.Write([]string{staffID, strconv.Itoa(pid)})
			staffPostCount++
		}
	}
	spw.Flush()
	staffPostF.Close()
	fmt.Fprintf(os.Stderr, "Wrote %d rows → sakugabooru_staff_posts.csv\n", staffPostCount)
}

func sortedKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
