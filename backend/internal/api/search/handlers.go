package search

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"

	"anigraph/backend/internal/api/httputil"
)

type Handler struct {
	es *elasticsearch.Client
}

func NewHandler(es *elasticsearch.Client) *Handler {
	return &Handler{es: es}
}

// Search handles GET /api/search — generic multi-index search.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	index := httputil.QueryString(r, "index", "_all")

	if q == "" {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": `Query parameter "q" is required`,
			"hits":    []any{},
		})
		return
	}

	body := map[string]any{
		"query": map[string]any{
			"multi_match": map[string]any{
				"query":     q,
				"fields":    []string{"*"},
				"type":      "best_fields",
				"fuzziness": "AUTO",
				"operator":  "or",
			},
		},
		"size": 10,
	}

	hits, total, err := h.doSearch(index, body)
	if err != nil {
		log.Printf("search error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to search in Elasticsearch")
		return
	}

	results := make([]map[string]any, len(hits))
	for i, hit := range hits {
		results[i] = map[string]any{
			"index":  hit.Index,
			"id":     hit.ID,
			"score":  hit.Score,
			"source": hit.Source,
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"total":   total,
		"hits":    results,
	})
}

// AnimeSearch handles GET /api/anime/search — anime-specific search.
func (h *Handler) AnimeSearch(w http.ResponseWriter, r *http.Request) {
	q := httputil.QueryString(r, "q", "*")
	limit := httputil.QueryInt(r, "limit", 20)

	exists, err := h.indexExists("anime")
	if err != nil || !exists {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": true,
			"total":   0,
			"data":    []any{},
			"message": "No anime data indexed yet. Please run data ingestion first.",
		})
		return
	}

	body := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"should": []any{
					map[string]any{
						"multi_match": map[string]any{
							"query":     q,
							"fields":    []string{"title^4", "title_english^3", "title_romaji^3", "title_native^2"},
							"type":      "best_fields",
							"fuzziness": "AUTO",
						},
					},
					map[string]any{
						"match": map[string]any{
							"title": map[string]any{
								"query":    q,
								"operator": "and",
								"boost":    2,
							},
						},
					},
				},
			},
		},
		"size": limit,
		"highlight": map[string]any{
			"fields": map[string]any{
				"title":         map[string]any{},
				"title_english": map[string]any{},
				"title_romaji":  map[string]any{},
			},
		},
	}

	hits, total, err := h.doSearch("anime", body)
	if err != nil {
		log.Printf("anime search error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to search anime")
		return
	}

	data := make([]map[string]any, len(hits))
	for i, hit := range hits {
		src := hit.Source
		data[i] = map[string]any{
			"id":           src["anilist_id"],
			"anilistId":    src["anilist_id"],
			"title":        src["title"],
			"titleEnglish": src["title_english"],
			"titleRomaji":  src["title_romaji"],
			"titleNative":  src["title_native"],
			"coverImage":   src["cover_image"],
			"format":       src["format"],
			"seasonYear":   src["season_year"],
			"season":       src["season"],
			"averageScore": src["average_score"],
			"score":        hit.Score,
			"highlights":   hit.Highlight,
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"total":   total,
		"data":    data,
	})
}

// Unified handles GET /api/search/unified — multi-index search across anime, staff, studios.
func (h *Handler) Unified(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": `Query parameter "q" is required`,
		})
		return
	}

	limit := httputil.QueryInt(r, "limit", 10)
	typesParam := httputil.QueryString(r, "types", "anime,staff,studios")
	includeAdult := httputil.QueryBool(r, "includeAdult")
	mediaType := r.URL.Query().Get("type")
	format := r.URL.Query().Get("format")
	yearMin := httputil.QueryIntPtr(r, "yearMin")
	yearMax := httputil.QueryIntPtr(r, "yearMax")
	eras := httputil.QueryStringSlice(r, "eras")
	sort := r.URL.Query().Get("sort")
	sortOrder := "desc"
	if strings.ToLower(r.URL.Query().Get("order")) == "asc" {
		sortOrder = "asc"
	}

	requestedTypes := strings.Split(typesParam, ",")
	for i := range requestedTypes {
		requestedTypes[i] = strings.TrimSpace(requestedTypes[i])
	}

	// Check if indices exist.
	for _, t := range requestedTypes {
		idx := t
		if idx == "anime" || idx == "staff" || idx == "studios" {
			exists, _ := h.indexExists(idx)
			if !exists {
				httputil.JSON(w, http.StatusOK, map[string]any{
					"success": true,
					"results": map[string]any{"anime": []any{}, "staff": []any{}, "studios": []any{}},
					"total":   map[string]any{"anime": 0, "staff": 0, "studios": 0},
					"message": "One or more indices do not exist. Please run the ingestion script first.",
				})
				return
			}
		}
	}

	// Build msearch body.
	var buf bytes.Buffer
	typeSet := make(map[string]bool)
	for _, t := range requestedTypes {
		typeSet[t] = true
	}

	if typeSet["anime"] {
		writeNDJSON(&buf, map[string]any{"index": "anime"})
		animeQuery := buildAnimeSearchQuery(q, limit, includeAdult, mediaType, format, yearMin, yearMax, eras, sort, sortOrder)
		writeNDJSON(&buf, animeQuery)
	}

	if typeSet["staff"] {
		writeNDJSON(&buf, map[string]any{"index": "staff"})
		writeNDJSON(&buf, map[string]any{
			"query": map[string]any{
				"bool": map[string]any{
					"should": []any{
						map[string]any{"multi_match": map[string]any{
							"query": q, "fields": []string{"name^4", "name_en^3", "name_ja^2"},
							"type": "best_fields", "fuzziness": "AUTO",
						}},
						map[string]any{"match": map[string]any{
							"name": map[string]any{"query": q, "operator": "and", "boost": 2},
						}},
					},
				},
			},
			"size":    limit,
			"_source": []string{"id", "staff_id", "name", "name_en", "name_ja", "image"},
			"highlight": map[string]any{
				"fields": map[string]any{"name": map[string]any{}, "name_en": map[string]any{}},
			},
		})
	}

	if typeSet["studios"] {
		writeNDJSON(&buf, map[string]any{"index": "studios"})
		writeNDJSON(&buf, map[string]any{
			"query": map[string]any{
				"bool": map[string]any{
					"should": []any{
						map[string]any{"multi_match": map[string]any{
							"query": q, "fields": []string{"name^3"},
							"type": "best_fields", "fuzziness": "AUTO",
						}},
						map[string]any{"match": map[string]any{
							"name": map[string]any{"query": q, "operator": "and", "boost": 2},
						}},
					},
				},
			},
			"size":    limit,
			"_source": []string{"id", "name", "image_url"},
			"highlight": map[string]any{
				"fields": map[string]any{"name": map[string]any{}},
			},
		})
	}

	// Execute msearch.
	res, err := h.es.Msearch(bytes.NewReader(buf.Bytes()))
	if err != nil {
		log.Printf("unified search error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to execute unified search")
		return
	}
	defer res.Body.Close()

	var msResult struct {
		Responses []searchResponse `json:"responses"`
	}
	if err := json.NewDecoder(res.Body).Decode(&msResult); err != nil {
		log.Printf("unified search decode error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to decode search results")
		return
	}

	results := map[string]any{"anime": []any{}, "staff": []any{}, "studios": []any{}}
	totals := map[string]int{"anime": 0, "staff": 0, "studios": 0}

	idx := 0
	if typeSet["anime"] && idx < len(msResult.Responses) {
		resp := msResult.Responses[idx]
		idx++
		totals["anime"] = resp.totalValue()
		items := make([]map[string]any, len(resp.Hits.Hits))
		for i, hit := range resp.Hits.Hits {
			displayName := strOr(hit.Source["title_english"], hit.Source["title_romaji"], hit.Source["title"])
			items[i] = map[string]any{
				"id":           orVal(hit.Source["anilist_id"], hit.Source["id"]),
				"title":        hit.Source["title"],
				"titleEnglish": hit.Source["title_english"],
				"titleRomaji":  hit.Source["title_romaji"],
				"titleNative":  hit.Source["title_native"],
				"coverImage":   hit.Source["cover_image"],
				"format":       hit.Source["format"],
				"seasonYear":   hit.Source["season_year"],
				"season":       hit.Source["season"],
				"averageScore": hit.Source["average_score"],
				"score":        matchQuality(displayName, q),
				"esScore":      hit.Score,
				"highlights":   hit.Highlight,
			}
		}
		results["anime"] = items
	}

	if typeSet["staff"] && idx < len(msResult.Responses) {
		resp := msResult.Responses[idx]
		idx++
		totals["staff"] = resp.totalValue()
		items := make([]map[string]any, len(resp.Hits.Hits))
		for i, hit := range resp.Hits.Hits {
			displayName := strOr(hit.Source["name_en"], hit.Source["name"])
			items[i] = map[string]any{
				"id":         orVal(hit.Source["staff_id"], hit.ID),
				"name":       hit.Source["name"],
				"nameEn":     hit.Source["name_en"],
				"nameJa":     hit.Source["name_ja"],
				"picture":    hit.Source["image"],
				"score":      matchQuality(displayName, q),
				"esScore":    hit.Score,
				"highlights": hit.Highlight,
			}
		}
		results["staff"] = items
	}

	if typeSet["studios"] && idx < len(msResult.Responses) {
		resp := msResult.Responses[idx]
		idx++
		totals["studios"] = resp.totalValue()
		items := make([]map[string]any, len(resp.Hits.Hits))
		for i, hit := range resp.Hits.Hits {
			displayName := strOr(hit.Source["name"])
			items[i] = map[string]any{
				"studioId":   orVal(hit.Source["id"], hit.ID),
				"name":       hit.Source["name"],
				"imageUrl":   hit.Source["image_url"],
				"score":      matchQuality(displayName, q),
				"esScore":    hit.Score,
				"highlights": hit.Highlight,
			}
		}
		results["studios"] = items
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"query":    q,
		"results":  results,
		"total":    totals,
		"totalAll": totals["anime"] + totals["staff"] + totals["studios"],
	})
}

// --- helpers ---

type searchHit struct {
	Index     string         `json:"_index"`
	ID        string         `json:"_id"`
	Score     float64        `json:"_score"`
	Source    map[string]any `json:"_source"`
	Highlight map[string]any `json:"highlight,omitempty"`
}

type searchResponse struct {
	Hits struct {
		Total json.RawMessage `json:"total"`
		Hits  []searchHit     `json:"hits"`
	} `json:"hits"`
	Error *json.RawMessage `json:"error,omitempty"`
}

func (sr searchResponse) totalValue() int {
	// total can be {"value":N,"relation":"eq"} or just N.
	var obj struct {
		Value int `json:"value"`
	}
	if err := json.Unmarshal(sr.Hits.Total, &obj); err == nil {
		return obj.Value
	}
	var n int
	json.Unmarshal(sr.Hits.Total, &n)
	return n
}

func (h *Handler) doSearch(index string, body map[string]any) ([]searchHit, int, error) {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	res, err := h.es.Search(
		h.es.Search.WithIndex(index),
		h.es.Search.WithBody(&buf),
	)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	var result searchResponse
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, 0, err
	}

	return result.Hits.Hits, result.totalValue(), nil
}

func (h *Handler) indexExists(index string) (bool, error) {
	res, err := h.es.Indices.Exists([]string{index})
	if err != nil {
		return false, err
	}
	res.Body.Close()
	return res.StatusCode == 200, nil
}

func buildAnimeSearchQuery(q string, limit int, includeAdult bool, mediaType, format string, yearMin, yearMax *int, eras []string, sort, sortOrder string) map[string]any {
	filters := []any{}

	if mediaType == "manga" {
		filters = append(filters, map[string]any{"terms": map[string]any{"format": []string{"MANGA", "NOVEL", "ONE_SHOT"}}})
	} else if mediaType == "anime" {
		filters = append(filters, map[string]any{"terms": map[string]any{"format": []string{"TV", "MOVIE", "OVA", "ONA", "SPECIAL"}}})
	}

	if format != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"format": format}})
	}

	if yearMin != nil || yearMax != nil {
		rangeQ := map[string]any{}
		if yearMin != nil {
			rangeQ["gte"] = *yearMin
		}
		if yearMax != nil {
			rangeQ["lte"] = *yearMax
		}
		filters = append(filters, map[string]any{"range": map[string]any{"season_year": rangeQ}})
	}

	if len(eras) > 0 {
		eraConditions := []any{}
		for _, era := range eras {
			switch era {
			case "pre-1960":
				eraConditions = append(eraConditions, map[string]any{"range": map[string]any{"season_year": map[string]any{"lt": 1960}}})
			case "1960s-1980s":
				eraConditions = append(eraConditions, map[string]any{"range": map[string]any{"season_year": map[string]any{"gte": 1960, "lt": 1990}}})
			case "1990s-2000s":
				eraConditions = append(eraConditions, map[string]any{"range": map[string]any{"season_year": map[string]any{"gte": 1990, "lt": 2010}}})
			case "2010s":
				eraConditions = append(eraConditions, map[string]any{"range": map[string]any{"season_year": map[string]any{"gte": 2010, "lt": 2020}}})
			case "2020s":
				eraConditions = append(eraConditions, map[string]any{"range": map[string]any{"season_year": map[string]any{"gte": 2020, "lt": 2030}}})
			}
		}
		if len(eraConditions) == 1 {
			filters = append(filters, eraConditions[0])
		} else if len(eraConditions) > 1 {
			filters = append(filters, map[string]any{"bool": map[string]any{"should": eraConditions, "minimum_should_match": 1}})
		}
	}

	boolQ := map[string]any{
		"should": []any{
			map[string]any{"multi_match": map[string]any{
				"query": q, "fields": []string{"title^4", "title_english^3", "title_romaji^3", "title_native^2"},
				"type": "best_fields", "fuzziness": "AUTO",
			}},
			map[string]any{"match": map[string]any{
				"title": map[string]any{"query": q, "operator": "and", "boost": 2},
			}},
		},
		"filter": filters,
	}

	if !includeAdult {
		boolQ["must_not"] = []any{
			map[string]any{"term": map[string]any{"is_adult": true}},
			map[string]any{"term": map[string]any{"genres": "Ecchi"}},
		}
	}

	query := map[string]any{
		"query": map[string]any{"bool": boolQ},
		"size":  limit,
		"_source": []string{
			"id", "anilist_id", "title", "title_english", "title_romaji", "title_native",
			"cover_image", "format", "season_year", "season", "average_score", "is_adult",
		},
		"highlight": map[string]any{
			"fields": map[string]any{
				"title": map[string]any{}, "title_english": map[string]any{}, "title_romaji": map[string]any{},
			},
		},
	}

	if sort == "score" {
		query["sort"] = []any{map[string]any{"average_score": map[string]any{"order": sortOrder, "missing": "_last"}}, "_score"}
	} else if sort == "year" {
		query["sort"] = []any{map[string]any{"season_year": map[string]any{"order": sortOrder, "missing": "_last"}}, "_score"}
	} else if sort == "title" {
		query["sort"] = []any{map[string]any{"title.keyword": map[string]any{"order": sortOrder, "missing": "_last"}}, "_score"}
	}

	return query
}

func writeNDJSON(buf *bytes.Buffer, v any) {
	json.NewEncoder(buf).Encode(v)
}

// matchQuality calculates string containment quality.
func matchQuality(displayName, query string) float64 {
	name := strings.ToLower(displayName)
	q := strings.ToLower(query)

	if name == q {
		return 1.0
	}
	if strings.HasPrefix(name, q) {
		return 0.9
	}
	if strings.Contains(name, q) {
		return 0.8
	}

	words := strings.Fields(q)
	allPresent := true
	for _, w := range words {
		if !strings.Contains(name, w) {
			allPresent = false
			break
		}
	}
	if allPresent && len(words) > 0 {
		return 0.7
	}

	return 0.5
}

// strOr returns the first non-empty string from the values.
func strOr(vals ...any) string {
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// orVal returns the first non-nil value.
func orVal(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}

// round2 rounds to 2 decimal places.
func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
