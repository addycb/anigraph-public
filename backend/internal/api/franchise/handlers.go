package franchise

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"anigraph/backend/internal/api/httputil"
)

type Handler struct {
	pg *pgxpool.Pool
	es *elasticsearch.Client
}

func NewHandler(pg *pgxpool.Pool, es *elasticsearch.Client) *Handler {
	return &Handler{pg: pg, es: es}
}

// Search handles GET /api/franchise/search.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := httputil.QueryInt(r, "limit", 20)

	if q == "" || len(q) < 2 {
		httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": []any{}})
		return
	}

	exists, _ := h.indexExists("franchises")
	if !exists {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": true, "data": []any{},
			"message": "Franchises index does not exist. Please run the sync script first.",
		})
		return
	}

	body := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"should": []any{
					map[string]any{"multi_match": map[string]any{
						"query": q, "fields": []string{"title^3"},
						"type": "best_fields", "fuzziness": "AUTO",
					}},
					map[string]any{"match": map[string]any{
						"title": map[string]any{"query": q, "operator": "and", "boost": 2},
					}},
				},
			},
		},
		"size":    limit,
		"_source": []string{"id", "title", "anime_count"},
		"sort":    []any{"_score", map[string]any{"anime_count": "desc"}},
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)

	res, err := h.es.Search(h.es.Search.WithIndex("franchises"), h.es.Search.WithBody(&buf))
	if err != nil {
		log.Printf("franchise search error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to search franchises")
		return
	}
	defer res.Body.Close()

	var result struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	json.NewDecoder(res.Body).Decode(&result)

	data := make([]map[string]any, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		animeCount := 0
		if v, ok := hit.Source["anime_count"].(float64); ok {
			animeCount = int(v)
		}
		data[i] = map[string]any{
			"id":         hit.Source["id"],
			"title":      hit.Source["title"],
			"animeCount": animeCount,
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

// GetByID handles GET /api/franchise/{id}.
func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	franchiseID, err := strconv.Atoi(idStr)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid franchise ID")
		return
	}

	ctx := r.Context()

	// Franchise info.
	var id int
	var title string
	err = h.pg.QueryRow(ctx, "SELECT id, title FROM franchise WHERE id = $1", franchiseID).Scan(&id, &title)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "Franchise not found")
		return
	}
	if err != nil {
		log.Printf("franchise detail error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch franchise")
		return
	}

	// Anime entries.
	rows, err := h.pg.Query(ctx, `
		SELECT anilist_id, title, title_romaji, title_english, title_native,
			cover_image, cover_image_extra_large, cover_image_large, cover_image_medium,
			banner_image, format, type, status, description,
			average_score::integer, season_year, season, episodes
		FROM anime WHERE franchise_id = $1
		ORDER BY season_year ASC NULLS LAST,
			CASE season WHEN 'WINTER' THEN 1 WHEN 'SPRING' THEN 2 WHEN 'SUMMER' THEN 3 WHEN 'FALL' THEN 4 ELSE 0 END ASC NULLS LAST`, franchiseID)
	if err != nil {
		log.Printf("franchise anime error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch franchise")
		return
	}
	defer rows.Close()

	var entries []map[string]any
	var anilistIDs []int
	for rows.Next() {
		var anilistID int
		var titleVal *string
		var titleRomaji, titleEn, titleNative *string
		var coverImage, coverImageXL, coverImageL, coverImageM, bannerImage *string
		var formatVal, typeVal, status, description *string
		var averageScore, seasonYear, episodes *int
		var season *string

		rows.Scan(&anilistID, &titleVal, &titleRomaji, &titleEn, &titleNative,
			&coverImage, &coverImageXL, &coverImageL, &coverImageM,
			&bannerImage, &formatVal, &typeVal, &status, &description,
			&averageScore, &seasonYear, &season, &episodes)

		anilistIDs = append(anilistIDs, anilistID)
		entries = append(entries, map[string]any{
			"anime": map[string]any{
				"anilistId": anilistID, "title": titleVal,
				"title_romaji": titleRomaji, "title_english": titleEn, "title_native": titleNative,
				"coverImage": coverImage, "coverImage_extraLarge": coverImageXL,
				"coverImage_large": coverImageL, "coverImage_medium": coverImageM,
				"bannerImage": bannerImage, "format": formatVal, "type": typeVal,
				"status": status, "description": description,
				"averageScore": averageScore, "seasonYear": seasonYear,
				"season": season, "episodes": episodes,
			},
		})
	}
	if entries == nil {
		entries = []map[string]any{}
	}

	// Relations within franchise.
	var relations []map[string]any
	if len(anilistIDs) > 0 {
		relRows, err := h.pg.Query(ctx, `
			SELECT a1.anilist_id, a2.anilist_id, ar.relation_type
			FROM anime_relation ar
			JOIN anime a1 ON ar.anime_id = a1.id
			JOIN anime a2 ON ar.related_anime_id = a2.id
			WHERE a1.franchise_id = $1 AND a2.franchise_id = $1`, franchiseID)
		if err == nil {
			defer relRows.Close()
			for relRows.Next() {
				var src, tgt int
				var relType string
				relRows.Scan(&src, &tgt, &relType)
				relations = append(relations, map[string]any{
					"source": src, "target": tgt, "relationType": relType,
				})
			}
		}
	}
	if relations == nil {
		relations = []map[string]any{}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"id": id, "title": title, "entries": entries, "relations": relations,
		},
	})
}

func (h *Handler) indexExists(index string) (bool, error) {
	res, err := h.es.Indices.Exists([]string{index})
	if err != nil {
		return false, err
	}
	res.Body.Close()
	return res.StatusCode == 200, nil
}
