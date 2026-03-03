package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// Configuration - same proxy pool as scrape_incremental.go
const (
	PROXY_PORT_START      = 10001
	PROXY_PORT_END        = 10100
	MIN_SECONDS_PER_PROXY = 3.8

	EXAMPLE_PROXY = "dc.decodo.com:10001:spawee4ylf:yIzsp7~aeb7Yrz87RQ"
)

// ProxyWorker manages rate limiting for a single proxy
type ProxyWorker struct {
	proxyURL    string
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func NewProxyWorker(proxyURL string) *ProxyWorker {
	proxy, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxy),
		MaxIdleConnsPerHost: 10,
	}

	return &ProxyWorker{
		proxyURL:    proxyURL,
		client:      &http.Client{Transport: transport, Timeout: 30 * time.Second},
		lastUsed:    time.Time{},
		minInterval: time.Duration(MIN_SECONDS_PER_PROXY * float64(time.Second)),
	}
}

func (pw *ProxyWorker) Do(req *http.Request) (*http.Response, error) {
	pw.mu.Lock()
	elapsed := time.Since(pw.lastUsed)
	if elapsed < pw.minInterval {
		time.Sleep(pw.minInterval - elapsed)
	}
	pw.lastUsed = time.Now()
	pw.mu.Unlock()

	return pw.client.Do(req)
}

// Build proxy list
func buildProxies(example string, start, end int) []string {
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

// Jikan API response types

// /v4/anime/{id} response — studios/producers have NO images here
type JikanAnimeResponse struct {
	Data JikanAnimeData `json:"data"`
}

type JikanAnimeData struct {
	Studios   []JikanAnimeProducer `json:"studios"`
	Producers []JikanAnimeProducer `json:"producers"`
}

type JikanAnimeProducer struct {
	MalID int    `json:"mal_id"`
	Name  string `json:"name"`
}

// /v4/producers/{id} response — this has the actual image and description
type JikanProducerResponse struct {
	Data JikanProducerData `json:"data"`
}

type JikanProducerData struct {
	MalID  int              `json:"mal_id"`
	Images JikanProducerImg `json:"images"`
	About  string           `json:"about"`
}

type JikanProducerImg struct {
	JPG JikanImageFormat `json:"jpg"`
}

type JikanImageFormat struct {
	ImageURL string `json:"image_url"`
}

// Task represents a studio to look up, with up to 3 fallback anime IDs
type StudioTask struct {
	StudioName  string
	MALAnimeIDs [3]int // up to 3 fallback anime IDs (0 = unused)
}

// ProducerData holds image URL and description fetched from the producer endpoint
type ProducerData struct {
	ImageURL    string
	Description string
}

// Result represents a found studio image and description
type StudioResult struct {
	StudioName  string
	MalID       int
	ImageURL    string
	Description string
}

// stripToAlphanumeric removes all non-letter/digit characters and lowercases
func stripToAlphanumeric(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// normalizeForComparison prepares a string for fuzzy matching:
// lowercase, strip corporate suffixes, then strip to alphanumeric
func normalizeForComparison(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	// Remove common corporate suffixes (longest first to avoid partial strips)
	suffixes := []string{
		", ltd.", " ltd.", " ltd",
		", inc.", " inc.", " inc",
		" co., ltd.", " co., ltd",
		" co.", " co",
		" corporation", " corp.", " corp",
		" entertainment",
		" animation studio", " animation",  " animations",
		" studios", " studio",
		" production ig", " production", " productions",
		" pictures",
		" films", " film",
		" works",
		" digital",
		" media",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			s = s[:len(s)-len(suffix)]
			break // only strip one suffix
		}
	}

	return stripToAlphanumeric(s)
}

// studioMatchScore returns a similarity score (0.0 - 1.0) between two studio names.
// Higher = better match. Returns 0 if definitely not the same studio.
func studioMatchScore(ourName, malName string) float64 {
	// Exact match (case-insensitive)
	if strings.EqualFold(strings.TrimSpace(ourName), strings.TrimSpace(malName)) {
		return 1.0
	}

	n1 := normalizeForComparison(ourName)
	n2 := normalizeForComparison(malName)

	if n1 == "" || n2 == "" {
		return 0
	}

	// Normalized exact match
	if n1 == n2 {
		return 0.95
	}

	// One contains the other entirely (e.g. "mappa" contained in "mappacolltd")
	if strings.Contains(n1, n2) || strings.Contains(n2, n1) {
		shorter := n1
		longer := n2
		if len(n1) > len(n2) {
			shorter = n2
			longer = n1
		}
		// Score based on how much of the longer string the shorter covers
		ratio := float64(len(shorter)) / float64(len(longer))
		if ratio > 0.4 { // at least 40% coverage
			return 0.7 + ratio*0.2
		}
	}

	// Levenshtein-based similarity on the normalized forms
	dist := levenshtein(n1, n2)
	maxLen := len(n1)
	if len(n2) > maxLen {
		maxLen = len(n2)
	}
	if maxLen == 0 {
		return 0
	}
	similarity := 1.0 - float64(dist)/float64(maxLen)

	// Only accept if reasonably similar (> 0.6 similarity on normalized)
	if similarity > 0.6 {
		return similarity * 0.8 // scale down since this is the loosest match
	}

	return 0
}

// levenshtein computes the edit distance between two strings
func levenshtein(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Use single row optimization
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// findBestMatchingStudio finds the best matching studio/producer from a Jikan anime response
// for our given studio name. Returns the best match if score >= minScore threshold.
func findBestMatchingStudio(ourName string, producers []JikanAnimeProducer) (*JikanAnimeProducer, float64) {
	var bestProducer *JikanAnimeProducer
	bestScore := 0.0

	for i := range producers {
		score := studioMatchScore(ourName, producers[i].Name)
		if score > bestScore {
			bestScore = score
			bestProducer = &producers[i]
		}
	}

	return bestProducer, bestScore
}

// isPlaceholderImage returns true if the image is MAL's generic "no picture" placeholder
func isPlaceholderImage(imageURL string) bool {
	return strings.Contains(imageURL, "company_no_picture") ||
		strings.Contains(imageURL, "questionmark") ||
		imageURL == ""
}

// jikanGet does an HTTP GET via a proxy worker with standard rate limit / error handling
func jikanGet(worker *ProxyWorker, apiURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := worker.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		retryAfter := resp.Header.Get("Retry-After")
		waitSeconds := 60
		if retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				waitSeconds = seconds
			}
		}
		time.Sleep(time.Duration(waitSeconds) * time.Second)
		return nil, fmt.Errorf("rate limited (429), waited %ds", waitSeconds)
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("not found (404)")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// fetchProducerData fetches the image URL and description for a specific MAL producer/studio ID
func fetchProducerData(worker *ProxyWorker, malProducerID int) (ProducerData, error) {
	apiURL := fmt.Sprintf("https://api.jikan.moe/v4/producers/%d", malProducerID)

	body, err := jikanGet(worker, apiURL)
	if err != nil {
		return ProducerData{}, fmt.Errorf("producer %d: %w", malProducerID, err)
	}

	var prodResp JikanProducerResponse
	if err := json.Unmarshal(body, &prodResp); err != nil {
		return ProducerData{}, fmt.Errorf("producer %d parse error: %w", malProducerID, err)
	}

	imageURL := prodResp.Data.Images.JPG.ImageURL
	if isPlaceholderImage(imageURL) {
		return ProducerData{}, fmt.Errorf("producer %d has placeholder image", malProducerID)
	}

	// Sanitize description: collapse newlines/carriage returns to spaces
	desc := strings.TrimSpace(prodResp.Data.About)
	desc = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(desc)

	return ProducerData{ImageURL: imageURL, Description: desc}, nil
}

// fetchStudioFromAnime fetches a Jikan anime page, finds the best matching studio,
// then fetches that studio's image from the producer endpoint (two API calls)
func fetchStudioFromAnime(worker *ProxyWorker, studioName string, malAnimeID int) (*StudioResult, error) {
	// Step 1: Get anime details to find studio mal_id
	apiURL := fmt.Sprintf("https://api.jikan.moe/v4/anime/%d", malAnimeID)

	body, err := jikanGet(worker, apiURL)
	if err != nil {
		return nil, fmt.Errorf("anime %d: %w", malAnimeID, err)
	}

	var jikanResp JikanAnimeResponse
	if err := json.Unmarshal(body, &jikanResp); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Search through both studios and producers arrays
	allProducers := append(jikanResp.Data.Studios, jikanResp.Data.Producers...)

	bestMatch, bestScore := findBestMatchingStudio(studioName, allProducers)

	// Require a minimum score of 0.5 to accept
	if bestMatch == nil || bestScore < 0.5 {
		var names []string
		for _, p := range allProducers {
			names = append(names, p.Name)
		}
		return nil, fmt.Errorf("studio '%s' no good match in anime %d (best=%.2f, producers=%v)",
			studioName, malAnimeID, bestScore, names)
	}

	fmt.Printf("   matched '%s' -> '%s' (score=%.2f, mal_id=%d)\n",
		studioName, bestMatch.Name, bestScore, bestMatch.MalID)

	// Step 2: Fetch the actual studio image and description from the producer endpoint
	producerData, err := fetchProducerData(worker, bestMatch.MalID)
	if err != nil {
		return nil, fmt.Errorf("studio '%s' matched '%s' but data fetch failed: %w",
			studioName, bestMatch.Name, err)
	}

	return &StudioResult{
		StudioName:  studioName,
		MalID:       bestMatch.MalID,
		ImageURL:    producerData.ImageURL,
		Description: producerData.Description,
	}, nil
}

// fetchStudioWithFallbacks tries each MAL anime ID in order until one succeeds
func fetchStudioWithFallbacks(worker *ProxyWorker, task StudioTask) (*StudioResult, error) {
	var lastErr error

	for i, malAnimeID := range task.MALAnimeIDs {
		if malAnimeID == 0 {
			continue // unused fallback slot
		}

		// Retry logic with exponential backoff for each attempt
		var result *StudioResult
		var err error

		for attempt := 0; attempt < 3; attempt++ {
			result, err = fetchStudioFromAnime(worker, task.StudioName, malAnimeID)
			if err == nil {
				return result, nil
			}

			// Don't retry if it's a "not found" or "no match" error — move to next fallback
			errMsg := err.Error()
			if strings.Contains(errMsg, "not found (404)") ||
				strings.Contains(errMsg, "no good match") ||
				strings.Contains(errMsg, "placeholder image") ||
				strings.Contains(errMsg, "image fetch failed") {
				break
			}

			// Transient error (network, rate limit) — retry
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			if attempt < 2 {
				time.Sleep(backoff)
			}
		}

		lastErr = err
		if i < len(task.MALAnimeIDs)-1 && task.MALAnimeIDs[i+1] != 0 {
			fmt.Printf("[fallback] %s: anime %d failed (%v), trying next...\n",
				task.StudioName, malAnimeID, err)
		}
	}

	return nil, lastErr
}

func main() {
	inputFile := flag.String("input", "", "Input CSV file (studio_name,mal_anime_id_1,mal_anime_id_2,mal_anime_id_3)")
	outputFile := flag.String("output", "", "Output CSV file (studio_name,mal_id,image_url)")
	flag.Parse()

	if *inputFile == "" || *outputFile == "" {
		fmt.Println("Usage: fetch_studio_images -input <file> -output <file>")
		os.Exit(1)
	}

	// Read input CSV
	inFile, err := os.Open(*inputFile)
	if err != nil {
		fmt.Printf("ERROR: Cannot open input file: %v\n", err)
		os.Exit(1)
	}
	defer inFile.Close()

	reader := csv.NewReader(inFile)
	reader.FieldsPerRecord = -1

	var tasks []StudioTask
	header := true
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if header {
			header = false
			continue
		}

		if len(record) < 2 {
			continue
		}

		studioName := record[0]
		var malIDs [3]int

		// Read up to 3 MAL anime IDs
		for i := 0; i < 3 && i+1 < len(record); i++ {
			id, err := strconv.Atoi(record[i+1])
			if err == nil && id > 0 {
				malIDs[i] = id
			}
		}

		// Skip if no valid MAL IDs at all
		if malIDs[0] == 0 && malIDs[1] == 0 && malIDs[2] == 0 {
			continue
		}

		tasks = append(tasks, StudioTask{
			StudioName:  studioName,
			MALAnimeIDs: malIDs,
		})
	}

	fmt.Printf("[init] Loaded %d studio tasks\n", len(tasks))

	if len(tasks) == 0 {
		// Write empty output
		outFile, _ := os.Create(*outputFile)
		writer := csv.NewWriter(outFile)
		writer.Write([]string{"studio_name", "mal_id", "image_url", "description"})
		writer.Flush()
		outFile.Close()
		fmt.Println("[done] No tasks to process")
		return
	}

	// Build proxy workers
	proxies := buildProxies(EXAMPLE_PROXY, PROXY_PORT_START, PROXY_PORT_END)
	fmt.Printf("[init] Using %d proxies\n", len(proxies))

	var workers []*ProxyWorker
	for _, p := range proxies {
		workers = append(workers, NewProxyWorker(p))
	}

	// Process tasks in parallel across workers
	// Each worker tries up to 3 fallback anime IDs per studio sequentially
	taskChan := make(chan StudioTask, len(tasks))
	resultChan := make(chan StudioResult, len(tasks))
	var wg sync.WaitGroup

	for i, worker := range workers {
		wg.Add(1)
		go func(id int, w *ProxyWorker) {
			defer wg.Done()
			for task := range taskChan {
				result, err := fetchStudioWithFallbacks(w, task)

				if err != nil {
					fmt.Printf("[worker-%d] %s FAILED all fallbacks: %v\n", id, task.StudioName, err)
					continue
				}

				if result != nil {
					fmt.Printf("[worker-%d] %s -> mal_id=%d image=%s\n",
						id, result.StudioName, result.MalID, result.ImageURL)
					resultChan <- *result
				}
			}
		}(i, worker)
	}

	// Feed tasks
	go func() {
		for _, task := range tasks {
			taskChan <- task
		}
		close(taskChan)
	}()

	// Wait for workers, then close results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and write output CSV
	outFile, err := os.Create(*outputFile)
	if err != nil {
		fmt.Printf("ERROR: Cannot create output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	writer.Write([]string{"studio_name", "mal_id", "image_url", "description"})

	count := 0
	for result := range resultChan {
		writer.Write([]string{
			result.StudioName,
			strconv.Itoa(result.MalID),
			result.ImageURL,
			result.Description,
		})
		count++
	}

	writer.Flush()
	fmt.Printf("[done] Wrote %d results to %s\n", count, *outputFile)
}
