package studio

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sort"

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

// Search handles GET /api/studio/search.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := httputil.QueryString(r, "q", "*")
	limit := httputil.QueryInt(r, "limit", 20)

	exists, _ := h.indexExists("studios")
	if !exists {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": true, "total": 0, "data": []any{},
			"message": "No studios data indexed yet. Please run data ingestion first.",
		})
		return
	}

	body := map[string]any{
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
		"size": limit,
		"highlight": map[string]any{
			"fields": map[string]any{"name": map[string]any{}},
		},
	}

	hits, total, err := h.doSearch("studios", body)
	if err != nil {
		log.Printf("studio search error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to search studios")
		return
	}

	data := make([]map[string]any, len(hits))
	for i, hit := range hits {
		data[i] = map[string]any{
			"studioId":   orVal(hit.Source["id"], hit.ID),
			"name":       hit.Source["name"],
			"imageUrl":   hit.Source["image_url"],
			"score":      hit.Score,
			"highlights": hit.Highlight,
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "total": total, "data": data})
}

// GetByName handles GET /api/studio/{name} — full studio detail.
func (h *Handler) GetByName(w http.ResponseWriter, r *http.Request) {
	studioName := chi.URLParam(r, "name")
	if studioName == "" {
		httputil.Error(w, http.StatusBadRequest, "Studio name is required")
		return
	}

	ctx := r.Context()

	// Fetch studio.
	var studioID int
	var name string
	var animeCount *int
	var imageURL, description, wikipediaEn, wikipediaJa *string
	var websiteURL, twitterHandle, youtubeChannelID, wikipediaContentHTML *string

	err := h.pg.QueryRow(ctx, `
		SELECT id, name, anime_count, image_url, description,
			wikipedia_en, wikipedia_ja, website_url, twitter_handle, youtube_channel_id,
			wikipedia_content_html
		FROM studio WHERE name = $1`, studioName).Scan(
		&studioID, &name, &animeCount, &imageURL, &description,
		&wikipediaEn, &wikipediaJa, &websiteURL, &twitterHandle, &youtubeChannelID,
		&wikipediaContentHTML)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "Studio not found")
		return
	}
	if err != nil {
		log.Printf("studio detail error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch studio details")
		return
	}

	// Productions.
	prodRows, err := h.pg.Query(ctx, `
		SELECT a.anilist_id, a.title, a.cover_image, a.cover_image_extra_large,
			a.cover_image_large, a.cover_image_medium, a.episodes, a.season_year, a.season,
			a.average_score::integer, a.format, a.is_adult, a.description, ast.is_main,
			(SELECT array_agg(DISTINCT g.name) FROM anime_genre ag JOIN genre g ON ag.genre_id = g.id WHERE ag.anime_id = a.id) as genres,
			(SELECT json_agg(json_build_object('name', t.name, 'rank', at.rank))
			 FROM anime_tag at JOIN tag t ON at.tag_id = t.id WHERE at.anime_id = a.id) as tags
		FROM anime_studio ast
		JOIN anime a ON ast.anime_id = a.id
		WHERE ast.studio_id = $1
		ORDER BY ast.is_main DESC,
			a.season_year DESC NULLS LAST,
			CASE a.season WHEN 'FALL' THEN 4 WHEN 'SUMMER' THEN 3 WHEN 'SPRING' THEN 2 WHEN 'WINTER' THEN 1 ELSE 0 END DESC NULLS LAST`, studioID)
	if err != nil {
		log.Printf("studio productions error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch studio details")
		return
	}
	defer prodRows.Close()

	var allProductions, mainProductions, supportingProductions []map[string]any
	genreCounts := map[string]int{}
	tagScores := map[string]*tagAcc{}

	for prodRows.Next() {
		var anilistID int
		var title string
		var coverImage, coverImageXL, coverImageL, coverImageM *string
		var episodes, seasonYear, averageScore *int
		var season, formatVal, desc *string
		var isAdult *bool
		var isMain bool
		var genres []string
		var tagsJSON []byte

		prodRows.Scan(&anilistID, &title, &coverImage, &coverImageXL,
			&coverImageL, &coverImageM, &episodes, &seasonYear, &season,
			&averageScore, &formatVal, &isAdult, &desc, &isMain, &genres, &tagsJSON)

		if title == "" {
			continue
		}

		var tags []map[string]any
		if tagsJSON != nil {
			json.Unmarshal(tagsJSON, &tags)
		}
		if tags == nil {
			tags = []map[string]any{}
		}
		if genres == nil {
			genres = []string{}
		}

		entry := map[string]any{
			"anime": map[string]any{
				"anilistId": anilistID, "title": title,
				"coverImage": coverImage, "coverImage_extraLarge": coverImageXL,
				"coverImage_large": coverImageL, "coverImage_medium": coverImageM,
				"episodes": episodes, "seasonYear": seasonYear, "season": season,
				"averageScore": averageScore, "format": formatVal,
				"isAdult": isAdult, "description": desc,
				"genres": genres, "tags": tags,
			},
			"isMain": isMain,
		}
		allProductions = append(allProductions, entry)
		if isMain {
			mainProductions = append(mainProductions, entry)
		} else {
			supportingProductions = append(supportingProductions, entry)
		}

		for _, g := range genres {
			genreCounts[g]++
		}
		for _, t := range tags {
			n, _ := t["name"].(string)
			rank, _ := t["rank"].(float64)
			if n == "" {
				continue
			}
			acc := tagScores[n]
			if acc == nil {
				acc = &tagAcc{}
				tagScores[n] = acc
			}
			acc.count++
			acc.totalRank += rank
		}
	}

	if allProductions == nil {
		allProductions = []map[string]any{}
	}
	if mainProductions == nil {
		mainProductions = []map[string]any{}
	}
	if supportingProductions == nil {
		supportingProductions = []map[string]any{}
	}

	// Collaborators.
	collabRows, err := h.pg.Query(ctx, `
		WITH studio_anime AS (SELECT anime_id FROM anime_studio WHERE studio_id = $1)
		SELECT s.name as studio_name, COUNT(DISTINCT ast.anime_id) as collaboration_count,
			(SELECT json_agg(json_build_object(
				'anilistId', a.anilist_id, 'title', a.title,
				'coverImage_large', a.cover_image_large, 'seasonYear', a.season_year,
				'season', a.season, 'isAdult', a.is_adult))
			FROM (SELECT DISTINCT a.anilist_id, a.title, a.cover_image_large, a.season_year, a.season, a.is_adult
				FROM anime_studio ast2 JOIN anime a ON ast2.anime_id = a.id
				WHERE ast2.studio_id = s.id AND ast2.anime_id IN (SELECT anime_id FROM studio_anime)
				LIMIT 5) a) as shared_anime
		FROM anime_studio ast
		JOIN studio s ON ast.studio_id = s.id
		WHERE ast.anime_id IN (SELECT anime_id FROM studio_anime) AND s.id <> $1
		GROUP BY s.id, s.name ORDER BY collaboration_count DESC LIMIT 20`, studioID)
	var collaborators []map[string]any
	if err == nil {
		defer collabRows.Close()
		for collabRows.Next() {
			var studioNameC string
			var collabCount int
			var sharedJSON []byte
			collabRows.Scan(&studioNameC, &collabCount, &sharedJSON)
			var shared []map[string]any
			if sharedJSON != nil {
				json.Unmarshal(sharedJSON, &shared)
			}
			if shared == nil {
				shared = []map[string]any{}
			}
			collaborators = append(collaborators, map[string]any{
				"studioName": studioNameC, "collaborationCount": collabCount, "sharedAnime": shared,
			})
		}
	}
	if collaborators == nil {
		collaborators = []map[string]any{}
	}

	// Genre stats.
	genreStats := make([]map[string]any, 0, len(genreCounts))
	for n, c := range genreCounts {
		genreStats = append(genreStats, map[string]any{"name": n, "count": c})
	}
	sort.Slice(genreStats, func(i, j int) bool {
		return genreStats[i]["count"].(int) > genreStats[j]["count"].(int)
	})

	// Tag stats (top 20).
	tagStats := make([]map[string]any, 0, len(tagScores))
	for n, acc := range tagScores {
		avg := 0.0
		if acc.count > 0 {
			avg = acc.totalRank / float64(acc.count)
		}
		tagStats = append(tagStats, map[string]any{"name": n, "count": acc.count, "avgRank": avg})
	}
	sort.Slice(tagStats, func(i, j int) bool {
		ci := tagStats[i]["count"].(int)
		cj := tagStats[j]["count"].(int)
		if ci != cj {
			return ci > cj
		}
		return tagStats[i]["avgRank"].(float64) > tagStats[j]["avgRank"].(float64)
	})
	if len(tagStats) > 20 {
		tagStats = tagStats[:20]
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"name": name, "imageUrl": imageURL, "description": description,
			"wikipediaEn": wikipediaEn, "wikipediaJa": wikipediaJa,
			"websiteUrl": websiteURL, "twitterHandle": twitterHandle,
			"youtubeChannelId": youtubeChannelID, "wikipediaContentHtml": wikipediaContentHTML,
			"productions": allProductions, "mainProductions": mainProductions,
			"supportingProductions": supportingProductions,
			"genreStats": genreStats, "tagStats": tagStats,
			"collaborators": collaborators,
			"totalProductions": len(allProductions),
			"mainProductionsCount": len(mainProductions),
			"supportingProductionsCount": len(supportingProductions),
		},
	})
}

// --- helpers ---

type tagAcc struct {
	count     int
	totalRank float64
}

type searchHit struct {
	ID        string         `json:"_id"`
	Score     float64        `json:"_score"`
	Source    map[string]any `json:"_source"`
	Highlight map[string]any `json:"highlight,omitempty"`
}

func (h *Handler) doSearch(index string, body map[string]any) ([]searchHit, int, error) {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body)
	res, err := h.es.Search(h.es.Search.WithIndex(index), h.es.Search.WithBody(&buf))
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	var result struct {
		Hits struct {
			Total json.RawMessage `json:"total"`
			Hits  []searchHit     `json:"hits"`
		} `json:"hits"`
	}
	json.NewDecoder(res.Body).Decode(&result)
	var total int
	var obj struct{ Value int }
	if json.Unmarshal(result.Hits.Total, &obj) == nil {
		total = obj.Value
	} else {
		json.Unmarshal(result.Hits.Total, &total)
	}
	return result.Hits.Hits, total, nil
}

func (h *Handler) indexExists(index string) (bool, error) {
	res, err := h.es.Indices.Exists([]string{index})
	if err != nil {
		return false, err
	}
	res.Body.Close()
	return res.StatusCode == 200, nil
}

func orVal(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}
