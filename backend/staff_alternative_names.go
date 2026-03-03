package main

// staff_alternative_names.go
//
// Backfill alternative names for existing staff from AniList GraphQL API.
//
// Reads staff IDs from stdin CSV (one column: staff_id), batches queries
// (20 per request) via proxied workers, outputs CSV to stdout:
//   staff_id,alternative_names (pipe-delimited)
//
// Usage:
//
//	psql "$DATABASE_URL" -t -A -c "SELECT staff_id FROM staff ORDER BY staff_id" \
//	  | ./staff_alternative_names > staff_alt_names.csv

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	apiURL           = "https://graphql.anilist.co"
	staffBatchSize   = 20
	proxyPortStart   = 10001
	proxyPortEnd     = 10100
	minSecsPerProxy  = 3.8
	exampleProxy     = "dc.decodo.com:10001:spawee4ylf:yIzsp7~aeb7Yrz87RQ"
)

// ---------------------------------------------------------------------------
// Proxy worker
// ---------------------------------------------------------------------------

type proxyWorker struct {
	proxyURL    string
	client      *http.Client
	lastUsed    time.Time
	mu          sync.Mutex
	minInterval time.Duration
}

func newProxyWorker(proxyURL string) *proxyWorker {
	proxy, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxy),
		MaxIdleConnsPerHost: 10,
	}
	return &proxyWorker{
		proxyURL:    proxyURL,
		client:      &http.Client{Transport: transport, Timeout: 30 * time.Second},
		minInterval: time.Duration(minSecsPerProxy * float64(time.Second)),
	}
}

func (pw *proxyWorker) Do(req *http.Request) (*http.Response, error) {
	pw.mu.Lock()
	elapsed := time.Since(pw.lastUsed)
	if elapsed < pw.minInterval {
		time.Sleep(pw.minInterval - elapsed)
	}
	pw.lastUsed = time.Now()
	pw.mu.Unlock()
	return pw.client.Do(req)
}

// ---------------------------------------------------------------------------
// GraphQL helpers
// ---------------------------------------------------------------------------

type gqlRequest struct {
	Query string `json:"query"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []interface{}   `json:"errors,omitempty"`
}

func gqlPost(worker *proxyWorker, payload gqlRequest) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

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
		fmt.Fprintf(os.Stderr, "[rate-limit] 429, waiting %ds\n", waitSeconds)
		time.Sleep(time.Duration(waitSeconds) * time.Second)
		return nil, fmt.Errorf("rate limited (429)")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gqlResp gqlResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, err
	}
	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("graphql errors: %v", gqlResp.Errors)
	}
	return gqlResp.Data, nil
}

func buildBatchQuery(staffIDs []int) string {
	fields := `id name { alternative }`
	var aliases []string
	for i, sid := range staffIDs {
		aliases = append(aliases, fmt.Sprintf("s%d: Staff(id: %d) { %s }", i, sid, fields))
	}
	return "{ " + strings.Join(aliases, " ") + " }"
}

// ---------------------------------------------------------------------------
// Build proxies
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Worker
// ---------------------------------------------------------------------------

type batchTask struct {
	StaffIDs []int
}

type resultRow struct {
	StaffID          int
	AlternativeNames string // pipe-delimited
}

func worker(id int, pw *proxyWorker, tasks <-chan batchTask, results chan<- resultRow, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range tasks {
		query := buildBatchQuery(task.StaffIDs)
		payload := gqlRequest{Query: query}

		var data []byte
		var err error

		for attempt := 0; attempt < 4; attempt++ {
			data, err = gqlPost(pw, payload)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "[worker-%d] batch FAILED for %v: %v\n", id, task.StaffIDs, err)
			// Output empty results for failed IDs
			for _, sid := range task.StaffIDs {
				results <- resultRow{StaffID: sid, AlternativeNames: ""}
			}
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			fmt.Fprintf(os.Stderr, "[worker-%d] JSON parse error: %v\n", id, err)
			for _, sid := range task.StaffIDs {
				results <- resultRow{StaffID: sid, AlternativeNames: ""}
			}
			continue
		}

		// Track which IDs we got results for
		foundIDs := make(map[int]bool)

		for _, v := range result {
			staff, ok := v.(map[string]interface{})
			if !ok || staff == nil {
				continue
			}
			staffID := 0
			if idVal, ok := staff["id"].(float64); ok {
				staffID = int(idVal)
			}
			if staffID == 0 {
				continue
			}
			foundIDs[staffID] = true

			var altNames []string
			if nameObj, ok := staff["name"].(map[string]interface{}); ok {
				if altArr, ok := nameObj["alternative"].([]interface{}); ok {
					for _, a := range altArr {
						if s, ok := a.(string); ok && s != "" {
							altNames = append(altNames, s)
						}
					}
				}
			}

			results <- resultRow{
				StaffID:          staffID,
				AlternativeNames: strings.Join(altNames, "|"),
			}
		}

		// Emit empty rows for IDs not in response (deleted staff, etc.)
		for _, sid := range task.StaffIDs {
			if !foundIDs[sid] {
				results <- resultRow{StaffID: sid, AlternativeNames: ""}
			}
		}

		fmt.Fprintf(os.Stderr, "[worker-%d] fetched %d/%d staff\n", id, len(foundIDs), len(task.StaffIDs))
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	// Read staff IDs from stdin
	r := csv.NewReader(os.Stdin)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var staffIDs []int
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(row) < 1 {
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(row[0]))
		if err != nil {
			continue
		}
		staffIDs = append(staffIDs, id)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d staff IDs from stdin\n", len(staffIDs))

	if len(staffIDs) == 0 {
		fmt.Fprintln(os.Stderr, "No staff IDs to process.")
		os.Exit(0)
	}

	// Build proxies and workers
	proxies := buildProxies(exampleProxy, proxyPortStart, proxyPortEnd)
	workers := make([]*proxyWorker, len(proxies))
	for i, p := range proxies {
		workers[i] = newProxyWorker(p)
	}
	fmt.Fprintf(os.Stderr, "Created %d proxy workers\n", len(workers))

	// Create tasks channel and results channel
	tasks := make(chan batchTask, 500)
	results := make(chan resultRow, 1000)

	// Start workers
	var wg sync.WaitGroup
	for i, w := range workers {
		wg.Add(1)
		go worker(i, w, tasks, results, &wg)
	}

	// Feed tasks
	go func() {
		for i := 0; i < len(staffIDs); i += staffBatchSize {
			end := i + staffBatchSize
			if end > len(staffIDs) {
				end = len(staffIDs)
			}
			tasks <- batchTask{StaffIDs: staffIDs[i:end]}
		}
		close(tasks)
	}()

	// Collect results in background
	go func() {
		wg.Wait()
		close(results)
	}()

	// Write CSV output to stdout
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"staff_id", "alternative_names"})

	count := 0
	withAlts := 0
	for row := range results {
		w.Write([]string{
			strconv.Itoa(row.StaffID),
			row.AlternativeNames,
		})
		count++
		if row.AlternativeNames != "" {
			withAlts++
		}
		if count%1000 == 0 {
			fmt.Fprintf(os.Stderr, "[progress] %d/%d processed, %d with alternatives\n", count, len(staffIDs), withAlts)
			w.Flush()
		}
	}
	w.Flush()

	fmt.Fprintf(os.Stderr, "Done: %d staff processed, %d with alternative names\n", count, withAlts)
}
