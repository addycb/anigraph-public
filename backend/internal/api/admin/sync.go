package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"anigraph/backend/internal/api/httputil"
)

const esBatchSize = 1000

// ES index settings with autocomplete analyzers.
var esIndexSettings = map[string]any{
	"analysis": map[string]any{
		"analyzer": map[string]any{
			"autocomplete_analyzer": map[string]any{
				"type": "custom", "tokenizer": "autocomplete_tokenizer",
				"filter": []string{"lowercase", "asciifolding"},
			},
			"autocomplete_search_analyzer": map[string]any{
				"type": "custom", "tokenizer": "standard",
				"filter": []string{"lowercase", "asciifolding"},
			},
		},
		"tokenizer": map[string]any{
			"autocomplete_tokenizer": map[string]any{
				"type": "edge_ngram", "min_gram": 2, "max_gram": 20,
				"token_chars": []string{"letter", "digit"},
			},
		},
	},
}

func esAutocompleteText(hasStandard bool) map[string]any {
	fields := map[string]any{"keyword": map[string]any{"type": "keyword"}}
	if hasStandard {
		fields["standard"] = map[string]any{"type": "text"}
	}
	return map[string]any{
		"type": "text", "analyzer": "autocomplete_analyzer",
		"search_analyzer": "autocomplete_search_analyzer", "fields": fields,
	}
}

var esAnimeMappings = map[string]any{"properties": map[string]any{
	"anilist_id": map[string]any{"type": "keyword"},
	"title": esAutocompleteText(true), "title_english": esAutocompleteText(false),
	"title_romaji": esAutocompleteText(false),
	"title_native": map[string]any{"type": "text", "fields": map[string]any{"keyword": map[string]any{"type": "keyword"}}},
	"synonyms":      esAutocompleteText(false),
	"cover_image":   map[string]any{"type": "keyword"},
	"format":        map[string]any{"type": "keyword"},
	"season_year":   map[string]any{"type": "integer"},
	"season":        map[string]any{"type": "keyword"},
	"average_score": map[string]any{"type": "float"},
	"is_adult":      map[string]any{"type": "boolean"},
}}

var esStaffMappings = map[string]any{"properties": map[string]any{
	"staff_id": map[string]any{"type": "keyword"},
	"name": esAutocompleteText(true), "name_en": esAutocompleteText(false),
	"name_ja": map[string]any{"type": "text", "fields": map[string]any{"keyword": map[string]any{"type": "keyword"}}},
	"image":   map[string]any{"type": "keyword"},
}}

var esStudioMappings = map[string]any{"properties": map[string]any{
	"id": map[string]any{"type": "keyword"}, "name": esAutocompleteText(true),
	"image_url": map[string]any{"type": "keyword"}, "description": map[string]any{"type": "text"},
}}

var esFranchiseMappings = map[string]any{"properties": map[string]any{
	"id": map[string]any{"type": "keyword"}, "title": esAutocompleteText(true),
	"anime_count": map[string]any{"type": "integer"},
}}

// SyncElasticsearchResult holds the result of an ES sync operation.
type SyncElasticsearchResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Summary struct {
		TotalIndexed int `json:"totalIndexed"`
		TotalFailed  int `json:"totalFailed"`
	} `json:"summary"`
}

// SyncElasticsearch syncs PostgreSQL data to Elasticsearch.
func (h *Handler) SyncElasticsearch(w http.ResponseWriter, r *http.Request) {
	recreate := r.URL.Query().Get("recreate") != "false"

	result, err := h.syncElasticsearch(r.Context(), recreate, nil, false)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Sync failed: %v", err))
		return
	}
	httputil.JSON(w, http.StatusOK, result)
}

// syncElasticsearch is the internal implementation callable from both the endpoint and the pipeline.
func (h *Handler) syncElasticsearch(ctx interface{ Done() <-chan struct{} }, recreate bool, animeIds []int32, fullSync bool) (map[string]any, error) {
	isIncremental := len(animeIds) > 0 && !fullSync
	start := time.Now()

	results := map[string]map[string]any{
		"anime":      {"indexed": 0, "failed": 0},
		"staff":      {"indexed": 0, "failed": 0},
		"studios":    {"indexed": 0, "failed": 0},
		"franchises": {"indexed": 0, "failed": 0},
	}

	type indexDef struct {
		name    string
		mapping map[string]any
	}
	indices := []indexDef{
		{"anime", esAnimeMappings}, {"staff", esStaffMappings},
		{"studios", esStudioMappings}, {"franchises", esFranchiseMappings},
	}

	// Create/recreate indices (skip for incremental).
	if !isIncremental {
		for _, idx := range indices {
			h.createOrRecreateESIndex(idx.name, idx.mapping, recreate)
		}
	}

	pgCtx := h.pg.Config().ConnConfig.ConnectTimeout
	_ = pgCtx

	// Index anime.
	{
		query := `SELECT anilist_id, COALESCE(NULLIF(title_english,''), title_romaji, title) as title,
			title_english, title_romaji, title_native, synonyms, cover_image, format,
			season_year, season, average_score, COALESCE(is_adult, false) as is_adult
			FROM anime`
		if isIncremental {
			query += fmt.Sprintf(" WHERE anilist_id = ANY('{%s}'::int[])", joinInts(animeIds))
		}
		query += " ORDER BY anilist_id"

		rows, err := h.pg.Query(staticCtx(), query)
		if err == nil {
			var records []map[string]any
			for rows.Next() {
				var id int32
				var title, titleEng, titleRom, titleNat, covImg, format, season *string
				var synonyms []string
				var seasonYear *int32
				var avgScore *float64
				var isAdult bool
				if err := rows.Scan(&id, &title, &titleEng, &titleRom, &titleNat, &synonyms, &covImg,
					&format, &seasonYear, &season, &avgScore, &isAdult); err != nil {
					continue
				}
				records = append(records, map[string]any{
					"anilist_id": id, "title": deref(title), "title_english": deref(titleEng),
					"title_romaji": deref(titleRom), "title_native": deref(titleNat),
					"synonyms": synonyms, "cover_image": deref(covImg),
					"format": deref(format), "season_year": derefInt(seasonYear),
					"season": deref(season), "average_score": derefFloat(avgScore),
					"is_adult": isAdult,
				})
			}
			rows.Close()

			if len(records) > 0 {
				indexed, failed := h.bulkIndexES("anime", records, func(r map[string]any) string {
					return fmt.Sprintf("%v", r["anilist_id"])
				})
				results["anime"]["indexed"] = indexed
				results["anime"]["failed"] = failed
				log.Printf("Indexed %d anime (%d failed)", indexed, failed)
			}
		}
	}

	// Index staff.
	if !isIncremental || len(animeIds) > 0 {
		query := `SELECT staff_id, COALESCE(NULLIF(name_en,''), name_ja) as name,
			name_en, name_ja, image_large as image FROM staff`
		if isIncremental {
			query = fmt.Sprintf(`SELECT DISTINCT s.staff_id, COALESCE(NULLIF(s.name_en,''), s.name_ja) as name,
				s.name_en, s.name_ja, s.image_large as image
				FROM staff s JOIN anime_staff ast ON s.id=ast.staff_id
				JOIN anime a ON a.id=ast.anime_id WHERE a.anilist_id = ANY('{%s}'::int[])`, joinInts(animeIds))
		}
		query += " ORDER BY staff_id"

		rows, err := h.pg.Query(staticCtx(), query)
		if err == nil {
			var records []map[string]any
			for rows.Next() {
				var id int32
				var name, nameEn, nameJa, image *string
				if err := rows.Scan(&id, &name, &nameEn, &nameJa, &image); err != nil {
					continue
				}
				records = append(records, map[string]any{
					"staff_id": id, "name": deref(name), "name_en": deref(nameEn),
					"name_ja": deref(nameJa), "image": deref(image),
				})
			}
			rows.Close()

			if len(records) > 0 {
				indexed, failed := h.bulkIndexES("staff", records, func(r map[string]any) string {
					return fmt.Sprintf("%v", r["staff_id"])
				})
				results["staff"]["indexed"] = indexed
				results["staff"]["failed"] = failed
				log.Printf("Indexed %d staff (%d failed)", indexed, failed)
			}
		}
	}

	// Index studios (full sync only).
	if !isIncremental {
		rows, err := h.pg.Query(staticCtx(), "SELECT id, name, image_url, description FROM studio ORDER BY id")
		if err == nil {
			var records []map[string]any
			for rows.Next() {
				var id int32
				var name string
				var imgURL, desc *string
				if err := rows.Scan(&id, &name, &imgURL, &desc); err != nil {
					continue
				}
				records = append(records, map[string]any{
					"id": id, "name": name, "image_url": deref(imgURL), "description": deref(desc),
				})
			}
			rows.Close()

			if len(records) > 0 {
				indexed, failed := h.bulkIndexES("studios", records, func(r map[string]any) string {
					return fmt.Sprintf("%v", r["id"])
				})
				results["studios"]["indexed"] = indexed
				results["studios"]["failed"] = failed
			}
		}
	}

	// Index franchises (full sync only).
	if !isIncremental {
		rows, err := h.pg.Query(staticCtx(), `
			SELECT f.id, f.title, COUNT(a.id) as anime_count
			FROM franchise f LEFT JOIN anime a ON a.franchise_id=f.id
			GROUP BY f.id, f.title ORDER BY anime_count DESC`)
		if err == nil {
			var records []map[string]any
			for rows.Next() {
				var id int32
				var title string
				var count int64
				if err := rows.Scan(&id, &title, &count); err != nil {
					continue
				}
				records = append(records, map[string]any{
					"id": id, "title": title, "anime_count": count,
				})
			}
			rows.Close()

			if len(records) > 0 {
				indexed, failed := h.bulkIndexES("franchises", records, func(r map[string]any) string {
					return fmt.Sprintf("%v", r["id"])
				})
				results["franchises"]["indexed"] = indexed
				results["franchises"]["failed"] = failed
			}
		}
	}

	totalIndexed := results["anime"]["indexed"].(int) + results["staff"]["indexed"].(int) +
		results["studios"]["indexed"].(int) + results["franchises"]["indexed"].(int)
	totalFailed := results["anime"]["failed"].(int) + results["staff"]["failed"].(int) +
		results["studios"]["failed"].(int) + results["franchises"]["failed"].(int)

	duration := time.Since(start)
	msg := fmt.Sprintf("Successfully synced %d documents from PostgreSQL to Elasticsearch", totalIndexed)
	if totalFailed > 0 {
		msg = fmt.Sprintf("Sync completed with some errors (%d indexed, %d failed)", totalIndexed, totalFailed)
	}

	return map[string]any{
		"success":  totalFailed == 0 && totalIndexed > 0,
		"message":  msg,
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
		"summary":  map[string]any{"totalIndexed": totalIndexed, "totalFailed": totalFailed},
		"details":  results,
	}, nil
}

func (h *Handler) createOrRecreateESIndex(name string, mapping map[string]any, recreate bool) {
	existsRes, err := h.es.Indices.Exists([]string{name})
	if err != nil {
		return
	}
	existsRes.Body.Close()

	if existsRes.StatusCode == 200 {
		if recreate {
			delRes, _ := h.es.Indices.Delete([]string{name})
			if delRes != nil {
				delRes.Body.Close()
			}
			log.Printf("Deleted existing index: %s", name)
		} else {
			return
		}
	}

	body := map[string]any{"settings": esIndexSettings, "mappings": mapping}
	bodyBytes, _ := json.Marshal(body)
	createRes, err := h.es.Indices.Create(name, h.es.Indices.Create.WithBody(strings.NewReader(string(bodyBytes))))
	if err == nil {
		createRes.Body.Close()
		log.Printf("Created index: %s", name)
	}
}

func (h *Handler) bulkIndexES(indexName string, records []map[string]any, getID func(map[string]any) string) (indexed, failed int) {
	for i := 0; i < len(records); i += esBatchSize {
		end := i + esBatchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]

		var buf bytes.Buffer
		for _, rec := range batch {
			meta := map[string]any{"index": map[string]any{"_index": indexName, "_id": getID(rec)}}
			metaBytes, _ := json.Marshal(meta)
			buf.Write(metaBytes)
			buf.WriteByte('\n')
			docBytes, _ := json.Marshal(rec)
			buf.Write(docBytes)
			buf.WriteByte('\n')
		}

		res, err := h.es.Bulk(strings.NewReader(buf.String()))
		if err != nil {
			failed += len(batch)
			continue
		}

		var result struct {
			Errors bool `json:"errors"`
			Items  []struct {
				Index struct {
					Error *struct {
						Type   string `json:"type"`
						Reason string `json:"reason"`
					} `json:"error"`
				} `json:"index"`
			} `json:"items"`
		}
		json.NewDecoder(res.Body).Decode(&result)
		res.Body.Close()

		if result.Errors {
			for _, item := range result.Items {
				if item.Index.Error != nil {
					failed++
				} else {
					indexed++
				}
			}
		} else {
			indexed += len(batch)
		}
	}
	return
}

func deref(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func derefInt(i *int32) any {
	if i == nil {
		return nil
	}
	return *i
}

func derefFloat(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}

func joinInts(ids []int32) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = fmt.Sprintf("%d", id)
	}
	return strings.Join(s, ",")
}
