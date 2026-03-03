package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
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
	APIURL              = "https://graphql.anilist.co"
	PER_PAGE            = 50
	STAFF_BATCH_SIZE    = 20
	PROXY_PORT_START    = 10001
	PROXY_PORT_END      = 10100
	MIN_SECONDS_PER_PROXY = 2.0 // 30 req/min

	// Proxy template: host:port:username:password
	EXAMPLE_PROXY = "dc.decodo.com:10001:spawee4ylf:yIzsp7~aeb7Yrz87RQ"

	// Output files
	ANIME_CSV = "media.csv"
	STAFF_CSV = "staff.csv"
	EDGE_CSV  = "media_staff_edges.csv"
	RELATIONS_CSV = "media_relations.csv"
	STATE_FILE = "scraper_state.json"
	FAILED_PAGES_FILE = "failed_pages.txt"
)

// GraphQL Queries
const ANIME_PAGE_QUERY = `
query ($page:Int, $perPage:Int) {
  Page(page:$page, perPage:$perPage) {
    pageInfo { hasNextPage }
    media {
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

const ANIME_STAFF_PAGE_QUERY = `
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

// Tracking for handover data
type ScrapeTracker struct {
	scrapedAnimeIDs sync.Map // map[int]bool
	scrapedStaffIDs sync.Map // map[int]bool
	maxAnimeID      int64
	maxStaffID      int64
	mu              sync.Mutex
}

func (st *ScrapeTracker) RecordAnime(id int) {
	st.scrapedAnimeIDs.Store(id, true)
	st.mu.Lock()
	if int64(id) > st.maxAnimeID {
		st.maxAnimeID = int64(id)
	}
	st.mu.Unlock()
}

func (st *ScrapeTracker) RecordStaff(id int) {
	st.scrapedStaffIDs.Store(id, true)
	st.mu.Lock()
	if int64(id) > st.maxStaffID {
		st.maxStaffID = int64(id)
	}
	st.mu.Unlock()
}

func (st *ScrapeTracker) GetAnimeCount() int {
	count := 0
	st.scrapedAnimeIDs.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func (st *ScrapeTracker) GetStaffCount() int {
	count := 0
	st.scrapedStaffIDs.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

var scrapeTracker *ScrapeTracker

// Types
type AnimePageTask struct {
	Page int
}

// Signal channel for stopping pagination when no more pages
var stopPagination chan bool

type StaffPageTask struct {
	AnimeID int
	Page    int
}

type StaffBatchTask struct {
	StaffIDs []int
}

type AnimeRow struct {
	Data []string
}

type EdgeRow struct {
	AnimeID int
	StaffID int
	Role    string
}

type RelationRow struct {
	MediaID         int
	RelatedMediaID  int
	RelationType    string
	RelatedType     string
	RelatedFormat   string
	RelatedTitle    string
}

type StaffRow struct {
	Data []string
}

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []interface{}   `json:"errors,omitempty"`
}

// ProxyWorker manages rate limiting for a single proxy
type ProxyWorker struct {
	proxyURL     string
	client       *http.Client
	lastUsed     time.Time
	mu           sync.Mutex
	minInterval  time.Duration
}

func NewProxyWorker(proxyURL string) *ProxyWorker {
	proxy, _ := url.Parse(proxyURL)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxy),
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
	// Rate limit: ensure min interval between requests
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

// Worker pool - one goroutine per proxy
func animePageWorker(id int, worker *ProxyWorker, tasks <-chan AnimePageTask, animeChan chan<- []string, edgeChan chan<- EdgeRow, relationsChan chan<- RelationRow, staffPageTasks chan<- StaffPageTask, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range tasks {
		// Fetch anime page with retry
		var data []byte
		var err error

		for attempt := 0; attempt < 4; attempt++ {
			payload := GraphQLRequest{
				Query: ANIME_PAGE_QUERY,
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
			failureLogger.Log("ANIME_PAGE page=%d error=%v", task.Page, err)
			continue
		}

		// Parse response
		var resp GraphQLResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			fmt.Printf("[worker-%d] page %d parse error: %v\n", id, task.Page, err)
			continue
		}

		var result struct {
			Page struct {
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
				} `json:"pageInfo"`
				Media []map[string]interface{} `json:"media"`
			} `json:"Page"`
		}

		if err := json.Unmarshal(resp.Data, &result); err != nil {
			fmt.Printf("[worker-%d] page %d parse error: %v\n", id, task.Page, err)
			continue
		}

		fmt.Printf("[worker-%d] fetched page %d (%d media)\n", id, task.Page, len(result.Page.Media))

		// Check if this is the last page
		if !result.Page.PageInfo.HasNextPage || len(result.Page.Media) == 0 {
			fmt.Printf("[worker-%d] Reached last page (page %d, hasNextPage=%v, mediaCount=%d)\n", id, task.Page, result.Page.PageInfo.HasNextPage, len(result.Page.Media))
			select {
			case stopPagination <- true:
			default:
			}
		}

		// Process anime
		for _, a := range result.Page.Media {
			// Track anime ID for handover data
			animeID := 0
			if id, ok := a["id"].(float64); ok {
				animeID = int(id)
				scrapeTracker.RecordAnime(animeID)
			}

			// Build anime row
			animeRow := buildAnimeRow(a)
			animeChan <- animeRow

			// Extract staff edges from page 1
			if staff, ok := a["staff"].(map[string]interface{}); ok {
				if edges, ok := staff["edges"].([]interface{}); ok {
					for _, e := range edges {
						if edge, ok := e.(map[string]interface{}); ok {
							if node, ok := edge["node"].(map[string]interface{}); ok {
								if staffID, ok := node["id"].(float64); ok {
									animeID := int(a["id"].(float64))
									role := ""
									if r, ok := edge["role"].(string); ok {
										role = r
									}
									edgeChan <- EdgeRow{AnimeID: animeID, StaffID: int(staffID), Role: role}
								}
							}
						}
					}
				}

				// Queue all remaining staff pages using total count
				if pageInfo, ok := staff["pageInfo"].(map[string]interface{}); ok {
					if total, ok := pageInfo["total"].(float64); ok && total > 25 {
						animeID := int(a["id"].(float64))
						totalPages := int((int(total) + 24) / 25) // ceil(total / 25)
						for page := 2; page <= totalPages; page++ {
							staffPageTasks <- StaffPageTask{AnimeID: animeID, Page: page}
						}
					}
				}
			}

			// Extract relations
			if relations, ok := a["relations"].(map[string]interface{}); ok {
				if edges, ok := relations["edges"].([]interface{}); ok {
					for _, e := range edges {
						if edge, ok := e.(map[string]interface{}); ok {
							mediaID := int(a["id"].(float64))
							relationType := ""
							if rt, ok := edge["relationType"].(string); ok {
								relationType = rt
							}

							if node, ok := edge["node"].(map[string]interface{}); ok {
								relatedMediaID := 0
								if id, ok := node["id"].(float64); ok {
									relatedMediaID = int(id)
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

								relationsChan <- RelationRow{
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
}

// Staff page worker
func staffPageWorker(id int, worker *ProxyWorker, tasks <-chan StaffPageTask, edgeChan chan<- EdgeRow, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range tasks {
		page := task.Page
		animeID := task.AnimeID

		var data []byte
		var err error

		for attempt := 0; attempt < 4; attempt++ {
			payload := GraphQLRequest{
				Query: ANIME_STAFF_PAGE_QUERY,
				Variables: map[string]interface{}{
					"id":      animeID,
					"page":    page,
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
			fmt.Printf("[staff-worker-%d] anime %d page %d FAILED\n", id, animeID, page)
			failureLogger.Log("STAFF_PAGE anime=%d page=%d error=%v", animeID, page, err)
			continue
		}

		var resp GraphQLResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}

		var result struct {
			Media struct {
				ID    int `json:"id"`
				Staff struct {
					PageInfo struct {
						HasNextPage bool `json:"hasNextPage"`
					} `json:"pageInfo"`
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
					role := ""
					if r, ok := edge["role"].(string); ok {
						role = r
					}
					edgeChan <- EdgeRow{AnimeID: animeID, StaffID: int(staffID), Role: role}
				}
			}
		}

		fmt.Printf("[staff-worker-%d] anime %d page %d: %d edges\n", id, animeID, page, len(result.Media.Staff.Edges))
	}
}

// Staff batch worker with individual fallback
func staffBatchWorker(id int, worker *ProxyWorker, tasks <-chan StaffBatchTask, staffChan chan<- []string, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range tasks {
		// Try batch query first
		query := buildStaffBatchQuery(task.StaffIDs)

		var data []byte
		var err error

		for attempt := 0; attempt < 4; attempt++ {
			payload := GraphQLRequest{Query: query}
			data, err = graphqlPost(worker, payload)
			if err == nil {
				break
			}

			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}

		batchFailed := false
		if err != nil {
			batchFailed = true
		} else {
			var resp GraphQLResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				batchFailed = true
			} else {
				var result map[string]interface{}
				if err := json.Unmarshal(resp.Data, &result); err != nil {
					batchFailed = true
				} else {
					// Check if we got actual data (not all nulls due to invalid ID)
					validCount := 0
					for _, s := range result {
						if staff, ok := s.(map[string]interface{}); ok && staff != nil {
							staffRow := buildStaffRow(staff)
							staffChan <- staffRow
							validCount++

							// Track staff ID for handover data
							if staffID, ok := staff["id"].(float64); ok {
								scrapeTracker.RecordStaff(int(staffID))
							}
						}
					}

					if validCount == 0 && len(task.StaffIDs) > 0 {
						batchFailed = true
					} else {
						fmt.Printf("[staff-batch-%d] fetched %d/%d staff\n", id, validCount, len(task.StaffIDs))
					}
				}
			}
		}

		// Fallback to individual queries if batch failed (goes through same rate-limited worker)
		if batchFailed {
			fmt.Printf("[staff-batch-%d] batch FAILED, falling back to individual queries for %d staff\n", id, len(task.StaffIDs))
			failureLogger.Log("STAFF_BATCH ids=%v failed, trying individually", task.StaffIDs)

			successCount := 0
			failCount := 0

			for _, staffID := range task.StaffIDs {
				individualQuery := fmt.Sprintf("{ Staff(id: %d) { id name { full native } languageV2 image { large medium } description primaryOccupations gender dateOfBirth { year month day } dateOfDeath { year month day } age yearsActive homeTown bloodType } }", staffID)

				var individualData []byte
				var individualErr error

				for attempt := 0; attempt < 2; attempt++ {
					payload := GraphQLRequest{Query: individualQuery}
					individualData, individualErr = graphqlPost(worker, payload) // Same rate-limited pipeline
					if individualErr == nil {
						break
					}
					time.Sleep(time.Second)
				}

				if individualErr != nil {
					failCount++
					failureLogger.Log("STAFF_INDIVIDUAL id=%d error=%v", staffID, individualErr)
					continue
				}

				var individualResp GraphQLResponse
				if err := json.Unmarshal(individualData, &individualResp); err != nil {
					failCount++
					continue
				}

				var individualResult struct {
					Staff map[string]interface{} `json:"Staff"`
				}

				if err := json.Unmarshal(individualResp.Data, &individualResult); err != nil {
					failCount++
					continue
				}

				if individualResult.Staff != nil {
					staffRow := buildStaffRow(individualResult.Staff)
					staffChan <- staffRow
					successCount++

					// Track staff ID for handover data
					if id, ok := individualResult.Staff["id"].(float64); ok {
						scrapeTracker.RecordStaff(int(id))
					}
				} else {
					failCount++
					failureLogger.Log("STAFF_INDIVIDUAL id=%d not found (likely deleted)", staffID)
				}
			}

			fmt.Printf("[staff-batch-%d] individual fallback: %d success, %d failed\n", id, successCount, failCount)
		}
	}
}

func buildStaffBatchQuery(staffIDs []int) string {
	fields := `
		id
		name { full native }
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

func buildAnimeRow(a map[string]interface{}) []string {
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

						if id, ok := node["id"].(float64); ok {
							studioID = strconv.Itoa(int(id))
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
	}
}

// CSV writer
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

func main() {
	rand.Seed(42)

	// Initialize stats tracker
	globalStats = NewStats()

	// Initialize scrape tracker for handover data
	scrapeTracker = &ScrapeTracker{}

	fmt.Println("[init] Running in FULL SCRAPE mode")

	// Initialize failure logger
	var err error
	failureLogger, err = NewFailureLogger(FAILED_PAGES_FILE)
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
	animePageTasks := make(chan AnimePageTask, 1000)
	staffPageTasks := make(chan StaffPageTask, 5000)
	staffBatchTasks := make(chan StaffBatchTask, 500)

	animeDataChan := make(chan []string, 1000)
	edgeDataChan := make(chan []string, 2000)
	relationsDataChan := make(chan []string, 2000)
	staffDataChan := make(chan []string, 1000)

	edgeChan := make(chan EdgeRow, 2000)
	relationsChan := make(chan RelationRow, 2000)

	// Start CSV writers
	var writerWg sync.WaitGroup
	writerWg.Add(4)

	animeHeader := []string{"id", "title_romaji", "title_english", "title_native", "type", "format", "status", "description", "season", "seasonYear", "startDate_year", "startDate_month", "startDate_day", "endDate_year", "endDate_month", "endDate_day", "episodes", "duration", "chapters", "volumes", "countryOfOrigin", "source", "updatedAt", "coverImage_extraLarge", "coverImage_large", "coverImage_medium", "coverImage_color", "bannerImage", "trailer_id", "trailer_site", "trailer_thumbnail", "genres", "synonyms", "tags", "averageScore", "meanScore", "popularity", "favourites", "isLicensed", "siteUrl", "rankings", "scoreDistribution", "isAdult", "studios"}
	staffHeader := []string{"id", "name_full", "name_native", "languageV2", "image_large", "image_medium", "description", "primaryOccupations", "gender", "dateOfBirth_year", "dateOfBirth_month", "dateOfBirth_day", "dateOfDeath_year", "dateOfDeath_month", "dateOfDeath_day", "age", "yearsActive", "homeTown", "bloodType"}
	edgeHeader := []string{"mediaId", "staffId", "role"}
	relationsHeader := []string{"mediaId", "relatedMediaId", "relationType", "relatedType", "relatedFormat", "relatedTitle"}

	go csvWriter(ANIME_CSV, animeHeader, animeDataChan, &writerWg)
	go csvWriter(STAFF_CSV, staffHeader, staffDataChan, &writerWg)
	go csvWriter(EDGE_CSV, edgeHeader, edgeDataChan, &writerWg)
	go csvWriter(RELATIONS_CSV, relationsHeader, relationsDataChan, &writerWg)

	// Edge collector
	go func() {
		edgeCount := 0
		for edge := range edgeChan {
			edgeDataChan <- []string{strconv.Itoa(edge.AnimeID), strconv.Itoa(edge.StaffID), edge.Role}
			edgeCount++
		}
		fmt.Printf("[edge-collector] Transformed %d edges total\n", edgeCount)
		close(edgeDataChan)
	}()

	// Relations collector
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
	var animeWg sync.WaitGroup
	var staffPageWg sync.WaitGroup
	var staffBatchWg sync.WaitGroup

	for i, worker := range workers {
		animeWg.Add(1)
		go animePageWorker(i, worker, animePageTasks, animeDataChan, edgeChan, relationsChan, staffPageTasks, &animeWg)

		staffPageWg.Add(1)
		go staffPageWorker(i, worker, staffPageTasks, edgeChan, &staffPageWg)

		staffBatchWg.Add(1)
		go staffBatchWorker(i, worker, staffBatchTasks, staffDataChan, &staffBatchWg)
	}

	// Initialize stop signal for pagination
	stopPagination = make(chan bool, 1)

	// Phase 1: Fetch all media pages
	fmt.Println("[main] Starting media fetch...")
	startTime := time.Now()

	// Queue media pages dynamically until we hit the end
	go func() {
		page := 1
		lookAhead := 50 // Queue pages in batches for efficiency
		fmt.Printf("[main] Starting dynamic pagination (will stop when no more pages)\n")

		for {
			// Queue a batch of pages
			for i := 0; i < lookAhead; i++ {
				select {
				case <-stopPagination:
					fmt.Printf("[main] Stop signal received, queued up to page %d\n", page-1)
					close(animePageTasks)
					return
				default:
					animePageTasks <- AnimePageTask{Page: page}
					page++
				}
			}
			// Small delay to check stop signal periodically
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Wait for anime workers
	animeWg.Wait()
	close(staffPageTasks)
	close(animeDataChan)
	close(relationsChan)

	// Wait for staff page workers
	staffPageWg.Wait()
	close(edgeChan)

	elapsed := time.Since(startTime)
	fmt.Printf("[main] Media fetch complete in %.1fs\n", elapsed.Seconds())

	// Phase 2: Read unique staff IDs from edges
	fmt.Println("[main] Extracting unique staff IDs...")
	staffIDs := readUniqueStaffIDs(EDGE_CSV)
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

	// Wait for CSV writers
	writerWg.Wait()

	// Stop stats reporter
	stopStats <- true

	totalElapsed := time.Since(startTime)

	// Print final stats
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
	fmt.Printf(strings.Repeat("=", 70) + "\n\n")

	fmt.Printf("[main] Complete! Total time: %.1fs\n", totalElapsed.Seconds())
	fmt.Printf("  - %s\n", ANIME_CSV)
	fmt.Printf("  - %s\n", EDGE_CSV)
	fmt.Printf("  - %s\n", RELATIONS_CSV)
	fmt.Printf("  - %s\n", STAFF_CSV)

	// Save scrape state
	mediaCount := scrapeTracker.GetAnimeCount()
	staffCount := scrapeTracker.GetStaffCount()

	stateData := map[string]interface{}{
		"last_scrape_timestamp": time.Now().UTC().Format(time.RFC3339),
		"max_media_id":          scrapeTracker.maxAnimeID,
		"max_staff_id":          scrapeTracker.maxStaffID,
		"total_media_scraped":   mediaCount,
		"total_staff_scraped":   staffCount,
		"total_requests":        total,
		"successful_requests":   success,
		"failed_requests":       failed,
		"scrape_duration_seconds": totalElapsed.Seconds(),
		"csv_files": map[string]string{
			"media":     ANIME_CSV,
			"staff":     STAFF_CSV,
			"edges":     EDGE_CSV,
			"relations": RELATIONS_CSV,
		},
		"notes": "Full scrape of all media types on AniList.",
	}

	stateJSON, err := json.MarshalIndent(stateData, "", "  ")
	if err == nil {
		if err := os.WriteFile(STATE_FILE, stateJSON, 0644); err == nil {
			fmt.Printf("\n[main] Scrape state saved to %s\n", STATE_FILE)
			fmt.Printf("  Max media ID: %d\n", scrapeTracker.maxAnimeID)
			fmt.Printf("  Max staff ID: %d\n", scrapeTracker.maxStaffID)
			fmt.Printf("  Total media:  %d\n", mediaCount)
			fmt.Printf("  Total staff:  %d\n", staffCount)
			fmt.Printf("  Timestamp:    %s\n", stateData["last_scrape_timestamp"])
		} else {
			fmt.Printf("[ERROR] Failed to save state file: %v\n", err)
		}
	} else {
		fmt.Printf("[ERROR] Failed to marshal state data: %v\n", err)
	}
}

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
