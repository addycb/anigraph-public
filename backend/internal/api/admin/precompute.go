package admin

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"anigraph/backend/internal/api/httputil"
)

// PrecomputeBitmaps ensures all studios, genres, and tags exist in PostgreSQL lookup tables.
func (h *Handler) PrecomputeBitmaps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	log.Println("Starting Filter Bitmap Precomputation")

	type lookupResult struct {
		Synced   int `json:"synced"`
		Inserted int `json:"inserted"`
	}

	results := map[string]*lookupResult{
		"studios": {},
		"genres":  {},
		"tags":    {},
	}

	// Extract and insert unique values from denormalized arrays.
	lookups := []struct {
		key    string
		table  string
		column string
	}{
		{"studios", "studio", "studio_names"},
		{"genres", "genre", "genre_names"},
		{"tags", "tag", "tag_names"},
	}

	for _, lk := range lookups {
		rows, err := h.pg.Query(ctx, fmt.Sprintf(
			"SELECT DISTINCT unnest(%s) as name FROM anime WHERE %s IS NOT NULL ORDER BY name",
			lk.column, lk.column))
		if err != nil {
			httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to extract %s: %v", lk.key, err))
			return
		}

		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				continue
			}
			results[lk.key].Synced++

			tag, err := h.pg.Exec(ctx, fmt.Sprintf(
				"INSERT INTO %s (name) VALUES ($1) ON CONFLICT (name) DO NOTHING RETURNING id", lk.table),
				name)
			if err == nil && tag.RowsAffected() > 0 {
				results[lk.key].Inserted++
			}
		}
		rows.Close()
	}

	// Get total counts.
	totals := map[string]int64{}
	for _, lk := range lookups {
		var count int64
		h.pg.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", lk.table)).Scan(&count)
		totals[lk.key] = count
	}

	duration := time.Since(start)
	log.Printf("Bitmap Precomputation Complete! Duration: %v", duration)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "Filter bitmaps synced successfully from PostgreSQL arrays",
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
		"lookups": map[string]any{
			"studios": map[string]any{
				"synced":          results["studios"].Synced,
				"inserted":        results["studios"].Inserted,
				"totalInPostgres": totals["studios"],
			},
			"genres": map[string]any{
				"synced":          results["genres"].Synced,
				"inserted":        results["genres"].Inserted,
				"totalInPostgres": totals["genres"],
			},
			"tags": map[string]any{
				"synced":          results["tags"].Synced,
				"inserted":        results["tags"].Inserted,
				"totalInPostgres": totals["tags"],
			},
		},
	})
}

// PrecomputeCounts computes anime counts for filter lookup tables.
func (h *Handler) PrecomputeCounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	log.Println("Starting Filter Counts Precomputation")

	type countResult struct {
		RowsUpdated int64 `json:"rowsUpdated"`
		TotalAnime  int64 `json:"totalAnime"`
	}
	results := map[string]*countResult{}

	queries := []struct {
		key   string
		table string
		array string
	}{
		{"studios", "studio", "studio_names"},
		{"genres", "genre", "genre_names"},
		{"tags", "tag", "tag_names"},
	}

	for _, q := range queries {
		sql := fmt.Sprintf(`WITH counts AS (
			SELECT unnest(%s) as name, count(*) as cnt
			FROM anime WHERE %s IS NOT NULL GROUP BY 1
		)
		UPDATE %s s SET anime_count = counts.cnt
		FROM counts WHERE s.name = counts.name
		RETURNING s.name, s.anime_count`, q.array, q.array, q.table)

		rows, err := h.pg.Query(ctx, sql)
		if err != nil {
			httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to compute %s counts: %v", q.key, err))
			return
		}

		cr := &countResult{}
		for rows.Next() {
			var name string
			var count int64
			if err := rows.Scan(&name, &count); err != nil {
				continue
			}
			cr.RowsUpdated++
			cr.TotalAnime += count
		}
		rows.Close()
		results[q.key] = cr
		log.Printf("Updated %d %s", cr.RowsUpdated, q.key)
	}

	duration := time.Since(start)
	log.Printf("Precomputation Complete! Duration: %v", duration)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "Filter counts precomputed successfully from PostgreSQL",
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
		"results":  results,
	})
}

// PrecomputeGlobalRankings precomputes global anime rankings by year/format/genre.
func (h *Handler) PrecomputeGlobalRankings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	log.Println("Starting Global Rankings Precomputation")

	// This runs the PostgreSQL-based ranking computation.
	// Uses window functions to rank anime within year+format groups.
	_, err := h.pg.Exec(ctx, `
		-- Clear previous rankings
		UPDATE anime SET global_rank = NULL;

		-- Compute rankings per year+format
		WITH ranked AS (
			SELECT id,
				ROW_NUMBER() OVER (
					PARTITION BY season_year, format
					ORDER BY average_score DESC NULLS LAST, popularity DESC NULLS LAST
				) as rank
			FROM anime
			WHERE average_score IS NOT NULL
				AND season_year IS NOT NULL
				AND format IS NOT NULL
		)
		UPDATE anime SET global_rank = ranked.rank
		FROM ranked WHERE anime.id = ranked.id
	`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to precompute global rankings: %v", err))
		return
	}

	duration := time.Since(start)
	log.Printf("Global Rankings Precomputation Complete! Duration: %v", duration)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "Global rankings precomputed successfully",
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
	})
}

// PopulateMalIDs populates anime.mal_id from the ARM static mapping dataset.
func (h *Handler) PopulateMalIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log.Println("Starting MAL ID population")

	// Download ARM dataset and build mapping.
	// The ARM dataset maps anilist_id → mal_id.
	// Since we can't use the npm package in Go, we fetch the JSON directly.
	armData, err := fetchARMData()
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch ARM data: %v", err))
		return
	}

	// Get candidates: anime without mal_id.
	rows, err := h.pg.Query(ctx, "SELECT anilist_id FROM anime WHERE mal_id IS NULL ORDER BY anilist_id")
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to query candidates: %v", err))
		return
	}
	defer rows.Close()

	var candidates []int32
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err == nil {
			candidates = append(candidates, id)
		}
	}

	// Build update arrays.
	var anilistArr, malArr []int32
	for _, id := range candidates {
		if malID, ok := armData[id]; ok {
			anilistArr = append(anilistArr, id)
			malArr = append(malArr, malID)
		}
	}

	if len(anilistArr) == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "No anime to update",
			"updated": 0,
			"total":   len(candidates),
		})
		return
	}

	// Bulk update via unnest.
	tag, err := h.pg.Exec(ctx,
		`UPDATE anime SET mal_id = data.mal_id
		 FROM (SELECT unnest($1::int[]) AS anilist_id, unnest($2::int[]) AS mal_id) AS data
		 WHERE anime.anilist_id = data.anilist_id AND anime.mal_id IS NULL`,
		anilistArr, malArr)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update MAL IDs: %v", err))
		return
	}

	updated := tag.RowsAffected()
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("Populated MAL IDs for %d anime (%d already had one or no mapping found).", updated, int64(len(candidates))-updated),
		"updated": updated,
		"total":   len(candidates),
	})
}

// fetchARMData downloads the ARM (Anime Relations Mapping) dataset.
func fetchARMData() (map[int32]int32, error) {
	// Use net/http to fetch the ARM JSON from the kawaiioverflow CDN.
	resp, err := http.Get("https://raw.githubusercontent.com/kawaiioverflow/arm/master/arm.json")
	if err != nil {
		return nil, fmt.Errorf("fetch ARM data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ARM data HTTP %d", resp.StatusCode)
	}

	var entries []struct {
		AnilistID *int32 `json:"anilist_id"`
		MalID     *int32 `json:"mal_id"`
	}

	if err := decodeJSON(resp.Body, &entries); err != nil {
		return nil, fmt.Errorf("decode ARM data: %w", err)
	}

	m := make(map[int32]int32, len(entries))
	for _, e := range entries {
		if e.AnilistID != nil && e.MalID != nil {
			m[*e.AnilistID] = *e.MalID
		}
	}
	return m, nil
}
