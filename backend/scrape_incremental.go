package main

import (
	"bytes"
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
)

// Configuration
const (
	APIURL                = "https://graphql.anilist.co"
	PER_PAGE              = 50
	STAFF_BATCH_SIZE      = 20
	PROXY_PORT_START      = 10001
	PROXY_PORT_END        = 10100
	MIN_SECONDS_PER_PROXY = 3.8

	EXAMPLE_PROXY = "dc.decodo.com:10001:spawee4ylf:yIzsp7~aeb7Yrz87RQ"

)

// Output file names (will be prefixed with output directory)
var (
	outputDir              = "."
	MEDIA_DELTA_CSV        = "media_delta.csv"
	STAFF_DELTA_CSV        = "staff_delta.csv"
	EDGE_DELTA_CSV         = "media_staff_edges_delta.csv"
	RELATIONS_DELTA_CSV    = "media_relations_delta.csv"
	CHANGED_STAFF_IDS_FILE = "changed_staff_ids.txt"
	FAILED_PAGES_FILE      = "failed_pages.txt"
)

// Failure logger
type FailureLogger struct {
	file *os.File
	mu   sync.Mutex
}

func NewFailureLogger(filename string) (*FailureLogger, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &FailureLogger{file: f}, nil
}

func (fl *FailureLogger) Log(format string, args ...interface{}) {
	if fl == nil || fl.file == nil {
		return
	}
	fl.mu.Lock()
	defer fl.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(fl.file, "[%s] ", timestamp)
	fmt.Fprintf(fl.file, format, args...)
	fmt.Fprintln(fl.file)
}

func (fl *FailureLogger) Close() {
	if fl != nil && fl.file != nil {
		fl.file.Close()
	}
}

var failureLogger *FailureLogger

// Track failed pages for retry
var failedPages struct {
	mu    sync.Mutex
	pages []int
}

func recordFailedPage(page int) {
	failedPages.mu.Lock()
	failedPages.pages = append(failedPages.pages, page)
	failedPages.mu.Unlock()
}

// Track failed staff pages for retry
var failedStaffPages struct {
	mu    sync.Mutex
	pages []StaffPageTask
}

func recordFailedStaffPage(mediaID, page int) {
	failedStaffPages.mu.Lock()
	failedStaffPages.pages = append(failedStaffPages.pages, StaffPageTask{MediaID: mediaID, Page: page})
	failedStaffPages.mu.Unlock()
}

// GraphQL Queries - paginate descending by ID
const MEDIA_PAGE_QUERY_DESC = `
query ($page: Int, $perPage: Int) {
  Page(page: $page, perPage: $perPage) {
    pageInfo { hasNextPage lastPage }
    media(sort: ID_DESC) {
      id
      title { romaji english native }
      type
      format
      status
      description
      season
      seasonYear
      startDate { year month day }
      endDate { year month day }
      episodes
      duration
      chapters
      volumes
      countryOfOrigin
      source
      updatedAt
      coverImage { extraLarge large medium color }
      bannerImage
      trailer { id site thumbnail }
      averageScore
      meanScore
      popularity
      favourites
      isLicensed
      siteUrl
      isAdult
      rankings { rank type context season year allTime }
      stats {
        scoreDistribution { score amount }
      }
      staff(page: 1, perPage: 25) {
        pageInfo { hasNextPage total }
        edges { role node { id name { full native } } }
      }
      studios { edges { isMain node { id name } } }
      genres
      synonyms
      tags { name category rank }
      relations {
        edges {
          id
          relationType
          node {
            id
            type
            format
            title { romaji english native }
          }
        }
      }
    }
  }
}
`

const STAFF_PAGE_QUERY = `
query ($id: Int, $page: Int, $perPage: Int) {
  Media(id: $id) {
    id
    staff(page: $page, perPage: $perPage) {
      pageInfo { hasNextPage }
      edges { role node { id name { full native } } }
    }
  }
}
`

// Stats tracker
type Stats struct {
	mu              sync.Mutex
	totalRequests   int64
	successRequests int64
	failedRequests  int64
	startTime       time.Time
	lastReportTime  time.Time
	lastReportCount int64
}

func NewStats() *Stats {
	now := time.Now()
	return &Stats{
		startTime:      now,
		lastReportTime: now,
	}
}

func (s *Stats) RecordSuccess() {
	s.mu.Lock()
	s.totalRequests++
	s.successRequests++
	s.mu.Unlock()
}

func (s *Stats) RecordFailure() {
	s.mu.Lock()
	s.totalRequests++
	s.failedRequests++
	s.mu.Unlock()
}

func (s *Stats) GetStats() (total, success, failed int64, reqPerSec float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	elapsed := time.Since(s.startTime).Seconds()
	if elapsed > 0 {
		reqPerSec = float64(s.totalRequests) / elapsed
	}
	return s.totalRequests, s.successRequests, s.failedRequests, reqPerSec
}

func (s *Stats) GetIntervalStats() (intervalReqs int64, intervalReqPerSec float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(s.lastReportTime).Seconds()
	intervalReqs = s.totalRequests - s.lastReportCount

	if elapsed > 0 {
		intervalReqPerSec = float64(intervalReqs) / elapsed
	}

	s.lastReportTime = now
	s.lastReportCount = s.totalRequests

	return intervalReqs, intervalReqPerSec
}

var globalStats *Stats

// Types
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []interface{}   `json:"errors,omitempty"`
}

type MediaPageTask struct {
	Page int
}

type StaffPageTask struct {
	MediaID int
	Page    int
}

type StaffBatchTask struct {
	StaffIDs []int
}

type EdgeRow struct {
	MediaID int
	StaffID int
	Role    string
}

type RelationRow struct {
	MediaID        int
	RelatedMediaID int
	RelationType   string
	RelatedType    string
	RelatedFormat  string
	RelatedTitle   string
}

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

// GraphQL request helper
func graphqlPost(worker *ProxyWorker, payload GraphQLRequest) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		globalStats.RecordFailure()
		return nil, err
	}

	req, err := http.NewRequest("POST", APIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		globalStats.RecordFailure()
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := worker.Do(req)
	if err != nil {
		globalStats.RecordFailure()
		return nil, err
	}
	defer resp.Body.Close()

	// Handle rate limiting (429) with Retry-After header
	if resp.StatusCode == 429 {
		retryAfter := resp.Header.Get("Retry-After")
		waitSeconds := 60 // default to 60 seconds if header missing
		if retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				waitSeconds = seconds
			}
		}
		fmt.Printf("[rate-limit] 429 received, waiting %d seconds before retry...\n", waitSeconds)
		time.Sleep(time.Duration(waitSeconds) * time.Second)
		globalStats.RecordFailure()
		return nil, fmt.Errorf("rate limited (429), waited %ds", waitSeconds)
	}

	// Handle other non-200 status codes
	if resp.StatusCode == 404 {
		globalStats.RecordFailure()
		return nil, fmt.Errorf("not found (404)")
	}

	if resp.StatusCode != 200 {
		globalStats.RecordFailure()
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		globalStats.RecordFailure()
		return nil, err
	}

	globalStats.RecordSuccess()
	return body, nil
}

// Signal to stop pagination when we hit our max ID
var stopSignal = make(chan struct{})
var stopOnce sync.Once

func signalStop() {
	stopOnce.Do(func() {
		close(stopSignal)
	})
}

func isStopped() bool {
	select {
	case <-stopSignal:
		return true
	default:
		return false
	}
}

// Media page worker - processes pages from work queue
func mediaPageWorker(
	id int,
	worker *ProxyWorker,
	tasks <-chan MediaPageTask,
	mediaRows chan<- []string,
	edgeRows chan<- EdgeRow,
	relationRows chan<- RelationRow,
	staffPageTasks chan<- StaffPageTask,
	staffIDsFound chan<- int,
	dbMaxID int,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for task := range tasks {
		if isStopped() {
			continue // drain the channel
		}

		var data []byte
		var err error

		for attempt := 0; attempt < 4; attempt++ {
			payload := GraphQLRequest{
				Query: MEDIA_PAGE_QUERY_DESC,
				Variables: map[string]interface{}{
					"page":    task.Page,
					"perPage": PER_PAGE,
				},
			}

			data, err = graphqlPost(worker, payload)
			if err == nil {
				break
			}

			backoff := time.Duration(1<<uint(attempt)) * time.Second
			fmt.Printf("[worker-%d] page %d attempt %d failed: %v, retry in %.1fs\n", id, task.Page, attempt+1, err, backoff.Seconds())
			time.Sleep(backoff)
		}

		if err != nil {
			fmt.Printf("[worker-%d] page %d FAILED after retries\n", id, task.Page)
			failureLogger.Log("MEDIA_PAGE page=%d error=%v", task.Page, err)
			recordFailedPage(task.Page)
			continue
		}

		var resp GraphQLResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			fmt.Printf("[worker-%d] page %d parse error: %v\n", id, task.Page, err)
			continue
		}

		var result struct {
			Page struct {
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
					LastPage    int  `json:"lastPage"`
				} `json:"pageInfo"`
				Media []map[string]interface{} `json:"media"`
			} `json:"Page"`
		}

		if err := json.Unmarshal(resp.Data, &result); err != nil {
			fmt.Printf("[worker-%d] page %d parse error: %v\n", id, task.Page, err)
			continue
		}

		fmt.Printf("[worker-%d] fetched page %d (%d media)\n", id, task.Page, len(result.Page.Media))

		// Process each media item
		foundOurMax := false
		for _, m := range result.Page.Media {
			mediaID := 0
			if idVal, ok := m["id"].(float64); ok {
				mediaID = int(idVal)
			}

			// Check if we've hit our database max - stop if so
			if mediaID <= dbMaxID {
				fmt.Printf("[worker-%d] Hit DB max ID %d (found media ID %d), signaling stop\n", id, dbMaxID, mediaID)
				foundOurMax = true
				signalStop()
				break
			}

			// Build media row
			row := buildMediaRow(m)
			mediaRows <- row

			// Extract staff from first page
			if staff, ok := m["staff"].(map[string]interface{}); ok {
				if edges, ok := staff["edges"].([]interface{}); ok {
					for _, e := range edges {
						if edge, ok := e.(map[string]interface{}); ok {
							if node, ok := edge["node"].(map[string]interface{}); ok {
								if staffID, ok := node["id"].(float64); ok {
									sid := int(staffID)
									role := ""
									if r, ok := edge["role"].(string); ok {
										role = r
									}
									edgeRows <- EdgeRow{MediaID: mediaID, StaffID: sid, Role: role}
									if staffIDsFound != nil {
										staffIDsFound <- sid
									}
								}
							}
						}
					}
				}

				// Check if we need more staff pages
				if pageInfo, ok := staff["pageInfo"].(map[string]interface{}); ok {
					if total, ok := pageInfo["total"].(float64); ok && total > 25 {
						totalPages := int((int(total) + 24) / 25)
						for page := 2; page <= totalPages; page++ {
							staffPageTasks <- StaffPageTask{MediaID: mediaID, Page: page}
						}
					}
				}
			}

			// Extract relations
			if relations, ok := m["relations"].(map[string]interface{}); ok {
				if edges, ok := relations["edges"].([]interface{}); ok {
					for _, e := range edges {
						if edge, ok := e.(map[string]interface{}); ok {
							relationType := ""
							if rt, ok := edge["relationType"].(string); ok {
								relationType = rt
							}

							if node, ok := edge["node"].(map[string]interface{}); ok {
								relatedMediaID := 0
								if idVal, ok := node["id"].(float64); ok {
									relatedMediaID = int(idVal)
								}

								relatedType := ""
								if t, ok := node["type"].(string); ok {
									relatedType = t
								}

								relatedFormat := ""
								if f, ok := node["format"].(string); ok {
									relatedFormat = f
								}

								relatedTitle := ""
								if title, ok := node["title"].(map[string]interface{}); ok {
									if romaji, ok := title["romaji"].(string); ok {
										relatedTitle = romaji
									} else if english, ok := title["english"].(string); ok {
										relatedTitle = english
									}
								}

								relationRows <- RelationRow{
									MediaID:        mediaID,
									RelatedMediaID: relatedMediaID,
									RelationType:   relationType,
									RelatedType:    relatedType,
									RelatedFormat:  relatedFormat,
									RelatedTitle:   relatedTitle,
								}
							}
						}
					}
				}
			}
		}

		if foundOurMax {
			break
		}

		// Check if we've reached the last page (for full scrape when max-id=0)
		if !result.Page.PageInfo.HasNextPage || len(result.Page.Media) == 0 {
			fmt.Printf("[worker-%d] Reached last page (page %d, hasNextPage=%v, mediaCount=%d), signaling stop\n",
				id, task.Page, result.Page.PageInfo.HasNextPage, len(result.Page.Media))
			signalStop()
			break
		}
	}
}

// Staff page worker - fetches additional staff pages for media with >25 staff
func staffPageWorker(
	id int,
	worker *ProxyWorker,
	tasks <-chan StaffPageTask,
	edgeRows chan<- EdgeRow,
	staffIDsFound chan<- int,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for task := range tasks {
		var data []byte
		var err error

		for attempt := 0; attempt < 4; attempt++ {
			payload := GraphQLRequest{
				Query: STAFF_PAGE_QUERY,
				Variables: map[string]interface{}{
					"id":      task.MediaID,
					"page":    task.Page,
					"perPage": 25,
				},
			}

			data, err = graphqlPost(worker, payload)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
		}

		if err != nil {
			fmt.Printf("[staff-worker-%d] media %d page %d FAILED\n", id, task.MediaID, task.Page)
			failureLogger.Log("STAFF_PAGE media=%d page=%d error=%v", task.MediaID, task.Page, err)
			recordFailedStaffPage(task.MediaID, task.Page)
			continue
		}

		var resp GraphQLResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}

		var result struct {
			Media struct {
				Staff struct {
					Edges []map[string]interface{} `json:"edges"`
				} `json:"staff"`
			} `json:"Media"`
		}

		if err := json.Unmarshal(resp.Data, &result); err != nil {
			continue
		}

		for _, edge := range result.Media.Staff.Edges {
			if node, ok := edge["node"].(map[string]interface{}); ok {
				if staffID, ok := node["id"].(float64); ok {
					sid := int(staffID)
					role := ""
					if r, ok := edge["role"].(string); ok {
						role = r
					}
					edgeRows <- EdgeRow{MediaID: task.MediaID, StaffID: sid, Role: role}
					if staffIDsFound != nil {
						staffIDsFound <- sid
					}
				}
			}
		}
	}
}

// Staff batch worker - fetches staff details in batches
func staffBatchWorker(
	id int,
	worker *ProxyWorker,
	tasks <-chan StaffBatchTask,
	staffRows chan<- []string,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for task := range tasks {
		query := buildStaffBatchQuery(task.StaffIDs)
		payload := GraphQLRequest{Query: query}

		var data []byte
		var err error

		for attempt := 0; attempt < 4; attempt++ {
			data, err = graphqlPost(worker, payload)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
		}

		if err != nil {
			fmt.Printf("[staff-batch-%d] batch FAILED, trying individually\n", id)
			failureLogger.Log("STAFF_BATCH ids=%v failed, trying individually", task.StaffIDs)
			// Fallback to individual queries
			for _, staffID := range task.StaffIDs {
				individualQuery := fmt.Sprintf("{ Staff(id: %d) { id name { full native alternative } languageV2 image { large medium } description primaryOccupations gender dateOfBirth { year month day } dateOfDeath { year month day } age yearsActive homeTown bloodType } }", staffID)
				payload := GraphQLRequest{Query: individualQuery}

				for attempt := 0; attempt < 2; attempt++ {
					data, err = graphqlPost(worker, payload)
					if err == nil {
						break
					}
					time.Sleep(time.Second)
				}

				if err != nil {
					failureLogger.Log("STAFF_INDIVIDUAL id=%d error=%v", staffID, err)
					continue
				}

				var resp GraphQLResponse
				if err := json.Unmarshal(data, &resp); err != nil {
					continue
				}

				var result struct {
					Staff map[string]interface{} `json:"Staff"`
				}
				if err := json.Unmarshal(resp.Data, &result); err != nil {
					continue
				}

				if result.Staff != nil {
					staffRows <- buildStaffRow(result.Staff)
				} else {
					failureLogger.Log("STAFF_INDIVIDUAL id=%d not found (likely deleted)", staffID)
				}
			}
			continue
		}

		var resp GraphQLResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}

		var result map[string]interface{}
		if err := json.Unmarshal(resp.Data, &result); err != nil {
			continue
		}

		count := 0
		for _, s := range result {
			if staff, ok := s.(map[string]interface{}); ok && staff != nil {
				staffRows <- buildStaffRow(staff)
				count++
			}
		}
		fmt.Printf("[staff-batch-%d] fetched %d/%d staff\n", id, count, len(task.StaffIDs))
	}
}

func buildStaffBatchQuery(staffIDs []int) string {
	fields := `
		id
		name { full native alternative }
		languageV2
		image { large medium }
		description
		primaryOccupations
		gender
		dateOfBirth { year month day }
		dateOfDeath { year month day }
		age
		yearsActive
		homeTown
		bloodType
	`

	var aliases []string
	for i, sid := range staffIDs {
		aliases = append(aliases, fmt.Sprintf("s%d: Staff(id: %d) { %s }", i, sid, fields))
	}

	return "{ " + strings.Join(aliases, " ") + " }"
}

func buildMediaRow(a map[string]interface{}) []string {
	getString := func(key string) string {
		if v, ok := a[key].(string); ok {
			return v
		}
		return ""
	}

	getNestedString := func(parent, child string) string {
		if p, ok := a[parent].(map[string]interface{}); ok {
			if v, ok := p[child].(string); ok {
				return v
			}
		}
		return ""
	}

	getInt := func(key string) string {
		if v, ok := a[key].(float64); ok {
			return strconv.Itoa(int(v))
		}
		return ""
	}

	getBool := func(key string) string {
		if v, ok := a[key].(bool); ok {
			if v {
				return "1"
			}
			return "0"
		}
		return "0"
	}

	joinArray := func(key string) string {
		if arr, ok := a[key].([]interface{}); ok {
			var strs []string
			for _, v := range arr {
				if s, ok := v.(string); ok {
					strs = append(strs, s)
				}
			}
			return strings.Join(strs, "|")
		}
		return ""
	}

	// Tags as JSON
	tagsJSON := "[]"
	if tags, ok := a["tags"].([]interface{}); ok {
		var tagList []map[string]interface{}
		for _, t := range tags {
			if tag, ok := t.(map[string]interface{}); ok {
				if name, ok := tag["name"].(string); ok && name != "" {
					tagList = append(tagList, tag)
				}
			}
		}
		if len(tagList) > 0 {
			if b, err := json.Marshal(tagList); err == nil {
				tagsJSON = string(b)
			}
		}
	}

	// Studios
	studiosStr := ""
	if studios, ok := a["studios"].(map[string]interface{}); ok {
		if edges, ok := studios["edges"].([]interface{}); ok {
			var parts []string
			for _, e := range edges {
				if edge, ok := e.(map[string]interface{}); ok {
					if node, ok := edge["node"].(map[string]interface{}); ok {
						studioID := ""
						studioName := ""
						isMain := "0"

						if idVal, ok := node["id"].(float64); ok {
							studioID = strconv.Itoa(int(idVal))
						}
						if name, ok := node["name"].(string); ok {
							studioName = name
						}
						if main, ok := edge["isMain"].(bool); ok && main {
							isMain = "1"
						}

						parts = append(parts, fmt.Sprintf("%s:%s:%s", studioID, studioName, isMain))
					}
				}
			}
			studiosStr = strings.Join(parts, "|")
		}
	}

	// Rankings as JSON
	rankingsJSON := "[]"
	if rankings, ok := a["rankings"].([]interface{}); ok && len(rankings) > 0 {
		if b, err := json.Marshal(rankings); err == nil {
			rankingsJSON = string(b)
		}
	}

	// Score distribution as JSON
	scoreDistJSON := "[]"
	if stats, ok := a["stats"].(map[string]interface{}); ok {
		if scoreDist, ok := stats["scoreDistribution"].([]interface{}); ok && len(scoreDist) > 0 {
			if b, err := json.Marshal(scoreDist); err == nil {
				scoreDistJSON = string(b)
			}
		}
	}

	getNestedInt := func(parent, child string) string {
		if p, ok := a[parent].(map[string]interface{}); ok {
			if v, ok := p[child].(float64); ok {
				return strconv.Itoa(int(v))
			}
		}
		return ""
	}

	return []string{
		getInt("id"),
		getNestedString("title", "romaji"),
		getNestedString("title", "english"),
		getNestedString("title", "native"),
		getString("type"),
		getString("format"),
		getString("status"),
		getString("description"),
		getString("season"),
		getInt("seasonYear"),
		getNestedInt("startDate", "year"),
		getNestedInt("startDate", "month"),
		getNestedInt("startDate", "day"),
		getNestedInt("endDate", "year"),
		getNestedInt("endDate", "month"),
		getNestedInt("endDate", "day"),
		getInt("episodes"),
		getInt("duration"),
		getInt("chapters"),
		getInt("volumes"),
		getString("countryOfOrigin"),
		getString("source"),
		getInt("updatedAt"),
		getNestedString("coverImage", "extraLarge"),
		getNestedString("coverImage", "large"),
		getNestedString("coverImage", "medium"),
		getNestedString("coverImage", "color"),
		getString("bannerImage"),
		getNestedString("trailer", "id"),
		getNestedString("trailer", "site"),
		getNestedString("trailer", "thumbnail"),
		joinArray("genres"),
		joinArray("synonyms"),
		tagsJSON,
		getInt("averageScore"),
		getInt("meanScore"),
		getInt("popularity"),
		getInt("favourites"),
		getBool("isLicensed"),
		getString("siteUrl"),
		rankingsJSON,
		scoreDistJSON,
		getBool("isAdult"),
		studiosStr,
	}
}

func buildStaffRow(s map[string]interface{}) []string {
	getString := func(key string) string {
		if v, ok := s[key].(string); ok {
			return v
		}
		return ""
	}

	getNestedString := func(parent, child string) string {
		if p, ok := s[parent].(map[string]interface{}); ok {
			if v, ok := p[child].(string); ok {
				return v
			}
		}
		return ""
	}

	getInt := func(key string) string {
		if v, ok := s[key].(float64); ok {
			return strconv.Itoa(int(v))
		}
		return ""
	}

	getNestedInt := func(parent, child string) string {
		if p, ok := s[parent].(map[string]interface{}); ok {
			if v, ok := p[child].(float64); ok {
				return strconv.Itoa(int(v))
			}
		}
		return ""
	}

	joinArray := func(key string) string {
		if arr, ok := s[key].([]interface{}); ok {
			var strs []string
			for _, v := range arr {
				if str, ok := v.(string); ok {
					strs = append(strs, str)
				} else if num, ok := v.(float64); ok {
					strs = append(strs, strconv.Itoa(int(num)))
				}
			}
			return strings.Join(strs, "|")
		}
		return ""
	}

	joinNestedArray := func(parent, child string) string {
		if p, ok := s[parent].(map[string]interface{}); ok {
			if arr, ok := p[child].([]interface{}); ok {
				var strs []string
				for _, v := range arr {
					if str, ok := v.(string); ok {
						strs = append(strs, str)
					}
				}
				return strings.Join(strs, "|")
			}
		}
		return ""
	}

	return []string{
		getInt("id"),
		getNestedString("name", "full"),
		getNestedString("name", "native"),
		getString("languageV2"),
		getNestedString("image", "large"),
		getNestedString("image", "medium"),
		getString("description"),
		joinArray("primaryOccupations"),
		getString("gender"),
		getNestedInt("dateOfBirth", "year"),
		getNestedInt("dateOfBirth", "month"),
		getNestedInt("dateOfBirth", "day"),
		getNestedInt("dateOfDeath", "year"),
		getNestedInt("dateOfDeath", "month"),
		getNestedInt("dateOfDeath", "day"),
		getInt("age"),
		joinArray("yearsActive"),
		getString("homeTown"),
		getString("bloodType"),
		joinNestedArray("name", "alternative"),
	}
}

// CSV writer - streams data directly to file (memory efficient)
func csvWriter(filename string, header []string, dataChan <-chan []string, wg *sync.WaitGroup) {
	defer wg.Done()

	file, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write(header)

	rowCount := 0
	for row := range dataChan {
		writer.Write(row)
		rowCount++
	}
	fmt.Printf("[csv-writer] %s: Wrote %d rows\n", filename, rowCount)
}

// Read unique staff IDs from already-written edges CSV
func readUniqueStaffIDs(filename string) []int {
	file, err := os.Open(filename)
	if err != nil {
		return nil
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, _ := reader.ReadAll()

	seen := make(map[int]bool)
	var ids []int

	for i, record := range records {
		if i == 0 { // skip header
			continue
		}
		if len(record) >= 2 {
			if id, err := strconv.Atoi(record[1]); err == nil {
				if !seen[id] {
					seen[id] = true
					ids = append(ids, id)
				}
			}
		}
	}

	return ids
}

// Retry failed media pages at the end
func retryFailedMediaPages(
	workers []*ProxyWorker,
	mediaRows chan<- []string,
	edgeRows chan<- EdgeRow,
	relationRows chan<- RelationRow,
	staffPageTasks chan<- StaffPageTask,
	dbMaxID int,
) int {
	failedPages.mu.Lock()
	pagesToRetry := make([]int, len(failedPages.pages))
	copy(pagesToRetry, failedPages.pages)
	failedPages.pages = nil // Clear for tracking permanent failures
	failedPages.mu.Unlock()

	if len(pagesToRetry) == 0 {
		return 0
	}

	fmt.Printf("\n[retry] Retrying %d failed media pages...\n", len(pagesToRetry))
	successCount := 0

	for i, page := range pagesToRetry {
		worker := workers[i%len(workers)]

		var data []byte
		var err error

		for attempt := 0; attempt < 4; attempt++ {
			payload := GraphQLRequest{
				Query: MEDIA_PAGE_QUERY_DESC,
				Variables: map[string]interface{}{
					"page":    page,
					"perPage": PER_PAGE,
				},
			}

			data, err = graphqlPost(worker, payload)
			if err == nil {
				break
			}

			backoff := time.Duration(1<<uint(attempt)) * time.Second
			fmt.Printf("[retry] page %d attempt %d failed: %v, retry in %.1fs\n", page, attempt+1, err, backoff.Seconds())
			time.Sleep(backoff)
		}

		if err != nil {
			fmt.Printf("[retry] page %d FAILED permanently\n", page)
			failureLogger.Log("MEDIA_PAGE_RETRY page=%d error=%v", page, err)
			recordFailedPage(page) // Record as permanently failed
			continue
		}

		var resp GraphQLResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			fmt.Printf("[retry] page %d parse error: %v\n", page, err)
			recordFailedPage(page)
			continue
		}

		var result struct {
			Page struct {
				Media []map[string]interface{} `json:"media"`
			} `json:"Page"`
		}

		if err := json.Unmarshal(resp.Data, &result); err != nil {
			fmt.Printf("[retry] page %d parse error: %v\n", page, err)
			recordFailedPage(page)
			continue
		}

		fmt.Printf("[retry] page %d succeeded (%d media)\n", page, len(result.Page.Media))
		successCount++

		// Process each media item (same logic as mediaPageWorker)
		for _, m := range result.Page.Media {
			mediaID := 0
			if idVal, ok := m["id"].(float64); ok {
				mediaID = int(idVal)
			}

			// Skip if we've already hit our max (shouldn't happen in retry, but be safe)
			if mediaID <= dbMaxID && dbMaxID > 0 {
				continue
			}

			// Build media row
			row := buildMediaRow(m)
			mediaRows <- row

			// Extract staff from first page
			if staff, ok := m["staff"].(map[string]interface{}); ok {
				if edges, ok := staff["edges"].([]interface{}); ok {
					for _, e := range edges {
						if edge, ok := e.(map[string]interface{}); ok {
							if node, ok := edge["node"].(map[string]interface{}); ok {
								if staffID, ok := node["id"].(float64); ok {
									sid := int(staffID)
									role := ""
									if r, ok := edge["role"].(string); ok {
										role = r
									}
									edgeRows <- EdgeRow{MediaID: mediaID, StaffID: sid, Role: role}
								}
							}
						}
					}
				}

				// Check if we need more staff pages
				if pageInfo, ok := staff["pageInfo"].(map[string]interface{}); ok {
					if total, ok := pageInfo["total"].(float64); ok && total > 25 {
						totalPages := int((int(total) + 24) / 25)
						for p := 2; p <= totalPages; p++ {
							staffPageTasks <- StaffPageTask{MediaID: mediaID, Page: p}
						}
					}
				}
			}

			// Extract relations
			if relations, ok := m["relations"].(map[string]interface{}); ok {
				if edges, ok := relations["edges"].([]interface{}); ok {
					for _, e := range edges {
						if edge, ok := e.(map[string]interface{}); ok {
							relationType := ""
							if rt, ok := edge["relationType"].(string); ok {
								relationType = rt
							}

							if node, ok := edge["node"].(map[string]interface{}); ok {
								relatedMediaID := 0
								if idVal, ok := node["id"].(float64); ok {
									relatedMediaID = int(idVal)
								}

								relatedType := ""
								if t, ok := node["type"].(string); ok {
									relatedType = t
								}

								relatedFormat := ""
								if f, ok := node["format"].(string); ok {
									relatedFormat = f
								}

								relatedTitle := ""
								if title, ok := node["title"].(map[string]interface{}); ok {
									if romaji, ok := title["romaji"].(string); ok {
										relatedTitle = romaji
									} else if english, ok := title["english"].(string); ok {
										relatedTitle = english
									}
								}

								relationRows <- RelationRow{
									MediaID:        mediaID,
									RelatedMediaID: relatedMediaID,
									RelationType:   relationType,
									RelatedType:    relatedType,
									RelatedFormat:  relatedFormat,
									RelatedTitle:   relatedTitle,
								}
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("[retry] Media page retry complete: %d/%d succeeded\n", successCount, len(pagesToRetry))
	return successCount
}

// Retry failed staff pages at the end
func retryFailedStaffPages(
	workers []*ProxyWorker,
	edgeRows chan<- EdgeRow,
) int {
	failedStaffPages.mu.Lock()
	pagesToRetry := make([]StaffPageTask, len(failedStaffPages.pages))
	copy(pagesToRetry, failedStaffPages.pages)
	failedStaffPages.pages = nil // Clear for tracking permanent failures
	failedStaffPages.mu.Unlock()

	if len(pagesToRetry) == 0 {
		return 0
	}

	fmt.Printf("\n[retry] Retrying %d failed staff pages...\n", len(pagesToRetry))
	successCount := 0

	for i, task := range pagesToRetry {
		worker := workers[i%len(workers)]

		var data []byte
		var err error

		for attempt := 0; attempt < 4; attempt++ {
			payload := GraphQLRequest{
				Query: STAFF_PAGE_QUERY,
				Variables: map[string]interface{}{
					"id":      task.MediaID,
					"page":    task.Page,
					"perPage": 25,
				},
			}

			data, err = graphqlPost(worker, payload)
			if err == nil {
				break
			}

			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}

		if err != nil {
			fmt.Printf("[retry] staff page media=%d page=%d FAILED permanently\n", task.MediaID, task.Page)
			failureLogger.Log("STAFF_PAGE_RETRY media=%d page=%d error=%v", task.MediaID, task.Page, err)
			recordFailedStaffPage(task.MediaID, task.Page) // Record as permanently failed
			continue
		}

		var resp GraphQLResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			recordFailedStaffPage(task.MediaID, task.Page)
			continue
		}

		var result struct {
			Media struct {
				Staff struct {
					Edges []map[string]interface{} `json:"edges"`
				} `json:"staff"`
			} `json:"Media"`
		}

		if err := json.Unmarshal(resp.Data, &result); err != nil {
			recordFailedStaffPage(task.MediaID, task.Page)
			continue
		}

		edgeCount := 0
		for _, edge := range result.Media.Staff.Edges {
			if node, ok := edge["node"].(map[string]interface{}); ok {
				if staffID, ok := node["id"].(float64); ok {
					sid := int(staffID)
					role := ""
					if r, ok := edge["role"].(string); ok {
						role = r
					}
					edgeRows <- EdgeRow{MediaID: task.MediaID, StaffID: sid, Role: role}
					edgeCount++
				}
			}
		}

		fmt.Printf("[retry] staff page media=%d page=%d succeeded (%d edges)\n", task.MediaID, task.Page, edgeCount)
		successCount++
	}

	fmt.Printf("[retry] Staff page retry complete: %d/%d succeeded\n", successCount, len(pagesToRetry))
	return successCount
}

func main() {
	// Parse command line flags
	maxID := flag.Int("max-id", 0, "Maximum anilist ID currently in database")
	outDir := flag.String("output-dir", ".", "Directory to write output CSV files")
	flag.Parse()

	outputDir = *outDir

	fmt.Printf("[init] Starting incremental scrape\n")
	fmt.Printf("[init] DB max ID: %d\n", *maxID)
	fmt.Printf("[init] Output directory: %s\n", outputDir)

	dbMaxID := *maxID
	if dbMaxID == 0 {
		fmt.Println("[init] max-id=0 detected - will scrape ALL media (fresh database)")
	}

	globalStats = NewStats()

	// Initialize failure logger
	var err error
	failureLogger, err = NewFailureLogger(outputDir + "/" + FAILED_PAGES_FILE)
	if err != nil {
		fmt.Printf("[ERROR] Could not create failure log: %v\n", err)
	}
	defer failureLogger.Close()

	// Build proxy list
	proxies := buildProxies(EXAMPLE_PROXY, PROXY_PORT_START, PROXY_PORT_END)
	fmt.Printf("[init] Using %d proxies\n", len(proxies))

	// Create proxy workers
	var workers []*ProxyWorker
	for _, p := range proxies {
		workers = append(workers, NewProxyWorker(p))
	}

	// Start stats reporter
	stopStats := make(chan bool)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				intervalReqs, intervalRate := globalStats.GetIntervalStats()
				total, success, failed, avgRate := globalStats.GetStats()
				fmt.Printf("\n[STATS] Interval: %d reqs (%.1f req/s) | Total: %d reqs (%.1f req/s avg) | Success: %d | Failed: %d\n\n",
					intervalReqs, intervalRate, total, avgRate, success, failed)
			case <-stopStats:
				return
			}
		}
	}()

	// Channels
	mediaPageTasks := make(chan MediaPageTask, 1000)
	staffPageTasks := make(chan StaffPageTask, 5000)
	staffBatchTasks := make(chan StaffBatchTask, 500)

	mediaDataChan := make(chan []string, 1000)
	edgeDataChan := make(chan []string, 2000)
	relationsDataChan := make(chan []string, 2000)
	staffDataChan := make(chan []string, 1000)

	edgeChan := make(chan EdgeRow, 2000)
	relationsChan := make(chan RelationRow, 2000)

	// Start streaming CSV writers (memory efficient - writes directly to disk)
	var writerWg sync.WaitGroup
	writerWg.Add(4)

	mediaHeader := []string{"id", "title_romaji", "title_english", "title_native", "type", "format", "status", "description", "season", "seasonYear", "startDate_year", "startDate_month", "startDate_day", "endDate_year", "endDate_month", "endDate_day", "episodes", "duration", "chapters", "volumes", "countryOfOrigin", "source", "updatedAt", "coverImage_extraLarge", "coverImage_large", "coverImage_medium", "coverImage_color", "bannerImage", "trailer_id", "trailer_site", "trailer_thumbnail", "genres", "synonyms", "tags", "averageScore", "meanScore", "popularity", "favourites", "isLicensed", "siteUrl", "rankings", "scoreDistribution", "isAdult", "studios"}
	staffHeader := []string{"id", "name_full", "name_native", "languageV2", "image_large", "image_medium", "description", "primaryOccupations", "gender", "dateOfBirth_year", "dateOfBirth_month", "dateOfBirth_day", "dateOfDeath_year", "dateOfDeath_month", "dateOfDeath_day", "age", "yearsActive", "homeTown", "bloodType", "alternativeNames"}
	edgeHeader := []string{"mediaId", "staffId", "role"}
	relationsHeader := []string{"mediaId", "relatedMediaId", "relationType", "relatedType", "relatedFormat", "relatedTitle"}

	go csvWriter(outputDir+"/"+MEDIA_DELTA_CSV, mediaHeader, mediaDataChan, &writerWg)
	go csvWriter(outputDir+"/"+STAFF_DELTA_CSV, staffHeader, staffDataChan, &writerWg)
	go csvWriter(outputDir+"/"+EDGE_DELTA_CSV, edgeHeader, edgeDataChan, &writerWg)
	go csvWriter(outputDir+"/"+RELATIONS_DELTA_CSV, relationsHeader, relationsDataChan, &writerWg)

	// Edge collector - transforms EdgeRow to []string and forwards to CSV writer
	go func() {
		edgeCount := 0
		for edge := range edgeChan {
			edgeDataChan <- []string{strconv.Itoa(edge.MediaID), strconv.Itoa(edge.StaffID), edge.Role}
			edgeCount++
		}
		fmt.Printf("[edge-collector] Transformed %d edges total\n", edgeCount)
		close(edgeDataChan)
	}()

	// Relations collector - transforms RelationRow to []string and forwards to CSV writer
	go func() {
		relationsCount := 0
		for relation := range relationsChan {
			relationsDataChan <- []string{
				strconv.Itoa(relation.MediaID),
				strconv.Itoa(relation.RelatedMediaID),
				relation.RelationType,
				relation.RelatedType,
				relation.RelatedFormat,
				relation.RelatedTitle,
			}
			relationsCount++
		}
		fmt.Printf("[relations-collector] Transformed %d relations total\n", relationsCount)
		close(relationsDataChan)
	}()

	// Start worker pools (one goroutine per proxy)
	var mediaWg sync.WaitGroup
	var staffPageWg sync.WaitGroup
	var staffBatchWg sync.WaitGroup

	for i, worker := range workers {
		mediaWg.Add(1)
		go mediaPageWorker(i, worker, mediaPageTasks, mediaDataChan, edgeChan, relationsChan, staffPageTasks, nil, dbMaxID, &mediaWg)

		staffPageWg.Add(1)
		go staffPageWorker(i, worker, staffPageTasks, edgeChan, nil, &staffPageWg)

		staffBatchWg.Add(1)
		go staffBatchWorker(i, worker, staffBatchTasks, staffDataChan, &staffBatchWg)
	}

	// Phase 1: Fetch all media pages
	fmt.Println("[main] Starting media fetch...")
	startTime := time.Now()

	// Queue media pages dynamically until we hit the end
	go func() {
		page := 1
		lookAhead := 50 // Queue pages in batches for efficiency
		fmt.Printf("[main] Starting dynamic pagination (will stop when we hit DB max ID or last page)\n")

		for {
			// Queue a batch of pages
			for i := 0; i < lookAhead; i++ {
				select {
				case <-stopSignal:
					fmt.Printf("[main] Stop signal received, queued up to page %d\n", page-1)
					close(mediaPageTasks)
					return
				default:
					mediaPageTasks <- MediaPageTask{Page: page}
					page++
				}
			}
			// Small delay to check stop signal periodically
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Wait for media workers
	mediaWg.Wait()

	// Retry failed media pages before closing channels
	retryFailedMediaPages(workers, mediaDataChan, edgeChan, relationsChan, staffPageTasks, dbMaxID)

	close(staffPageTasks)
	close(mediaDataChan)
	close(relationsChan)

	// Wait for staff page workers
	staffPageWg.Wait()

	// Retry failed staff pages before closing edge channel
	retryFailedStaffPages(workers, edgeChan)

	close(edgeChan)

	elapsed := time.Since(startTime)
	fmt.Printf("[main] Media fetch complete in %.1fs\n", elapsed.Seconds())

	// Phase 2: Read unique staff IDs from edges CSV (already written to disk)
	fmt.Println("[main] Extracting unique staff IDs from edges file...")
	edgeFilePath := outputDir + "/" + EDGE_DELTA_CSV
	staffIDs := readUniqueStaffIDs(edgeFilePath)
	fmt.Printf("[main] Found %d unique staff\n", len(staffIDs))

	// Phase 3: Fetch staff in batches
	fmt.Println("[main] Starting staff fetch...")
	go func() {
		for i := 0; i < len(staffIDs); i += STAFF_BATCH_SIZE {
			end := i + STAFF_BATCH_SIZE
			if end > len(staffIDs) {
				end = len(staffIDs)
			}
			staffBatchTasks <- StaffBatchTask{StaffIDs: staffIDs[i:end]}
		}
		close(staffBatchTasks)
	}()

	staffBatchWg.Wait()
	close(staffDataChan)

	// Wait for CSV writers to finish
	writerWg.Wait()

	// Stop stats reporter
	stopStats <- true

	// Write changed staff IDs file
	changedFile, _ := os.Create(outputDir + "/" + CHANGED_STAFF_IDS_FILE)
	for _, sid := range staffIDs {
		fmt.Fprintf(changedFile, "%d\n", sid)
	}
	changedFile.Close()

	totalElapsed := time.Since(startTime)

	// Check for failed pages
	failedPages.mu.Lock()
	numFailedPages := len(failedPages.pages)
	failedPagesList := make([]int, len(failedPages.pages))
	copy(failedPagesList, failedPages.pages)
	failedPages.mu.Unlock()

	// Check for failed staff pages
	failedStaffPages.mu.Lock()
	numFailedStaffPages := len(failedStaffPages.pages)
	failedStaffPages.mu.Unlock()

	// Final stats
	total, success, failed, avgRate := globalStats.GetStats()
	fmt.Printf("\n" + strings.Repeat("=", 70) + "\n")
	fmt.Printf("[FINAL STATS]\n")
	fmt.Printf("  Total runtime:    %.1fs\n", totalElapsed.Seconds())
	fmt.Printf("  Total requests:   %d\n", total)
	fmt.Printf("  Successful:       %d\n", success)
	fmt.Printf("  Failed:           %d\n", failed)
	fmt.Printf("  Average rate:     %.2f req/s\n", avgRate)
	fmt.Printf("  Proxies used:     %d\n", len(proxies))
	fmt.Printf("  Theoretical max:  %.1f req/s (30 req/min per proxy)\n", float64(len(proxies))*0.5)
	fmt.Printf("  Efficiency:       %.1f%%\n", (avgRate/(float64(len(proxies))*0.5))*100)
	if numFailedPages > 0 {
		fmt.Printf("  FAILED MEDIA PAGES:  %d (see %s)\n", numFailedPages, FAILED_PAGES_FILE)
	}
	if numFailedStaffPages > 0 {
		fmt.Printf("  FAILED STAFF PAGES:  %d (see %s)\n", numFailedStaffPages, FAILED_PAGES_FILE)
	}
	fmt.Printf(strings.Repeat("=", 70) + "\n\n")

	if numFailedPages > 0 {
		fmt.Printf("[WARNING] %d media pages failed after retries: %v\n", numFailedPages, failedPagesList)
		fmt.Printf("          These pages are logged in %s\n", outputDir+"/"+FAILED_PAGES_FILE)
		fmt.Printf("          You may want to re-run the scraper or manually check these pages.\n\n")
	}

	if numFailedStaffPages > 0 {
		fmt.Printf("[WARNING] %d staff pages failed after retries (logged in %s)\n\n", numFailedStaffPages, outputDir+"/"+FAILED_PAGES_FILE)
	}

	fmt.Printf("[main] Complete! Total time: %.1fs\n", totalElapsed.Seconds())
	fmt.Printf("  Changed staff IDs: %d\n", len(staffIDs))
	fmt.Printf("\nOutput files:\n")
	fmt.Printf("  - %s\n", MEDIA_DELTA_CSV)
	fmt.Printf("  - %s\n", STAFF_DELTA_CSV)
	fmt.Printf("  - %s\n", EDGE_DELTA_CSV)
	fmt.Printf("  - %s\n", RELATIONS_DELTA_CSV)
	fmt.Printf("  - %s\n", CHANGED_STAFF_IDS_FILE)
	if numFailedPages > 0 {
		fmt.Printf("  - %s (contains %d failed pages)\n", FAILED_PAGES_FILE, numFailedPages)
	}
}
