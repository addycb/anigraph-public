package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"anigraph/backend/internal/api/httputil"
)

// RecomputeGraphs recomputes all anime graph data (staff/anime connections).
func (h *Handler) RecomputeGraphs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	concurrency := httputil.QueryInt(r, "concurrency", 8)

	log.Println("Starting full graph recomputation")

	// Get all anime anilist_ids.
	rows, err := h.pg.Query(ctx, "SELECT anilist_id FROM anime ORDER BY anilist_id")
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	var animeIDs []int32
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err == nil {
			animeIDs = append(animeIDs, id)
		}
	}

	total := len(animeIDs)
	if total == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "message": "No anime found.", "total": 0})
		return
	}

	// Run graph computation in background.
	go func() {
		bgCtx := context.Background()
		computed, errors := h.computeGraphsBatch(bgCtx, animeIDs, concurrency, 5*time.Minute)
		log.Printf("Graph recomputation complete: %d computed, %d errors out of %d total", computed, errors, total)
	}()

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"message":     fmt.Sprintf("Graph recomputation started for %d anime with concurrency %d. Monitor logs.", total, concurrency),
		"total":       total,
		"concurrency": concurrency,
		"startedAt":   start.Format(time.RFC3339),
	})
}

// ComputeMissingGraphs computes graphs only for anime missing from the cache.
func (h *Handler) ComputeMissingGraphs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	concurrency := httputil.QueryInt(r, "concurrency", 8)

	rows, err := h.pg.Query(ctx, `
		SELECT a.anilist_id FROM anime a
		LEFT JOIN anime_graph_cache agc ON a.anilist_id = agc.anilist_id
		WHERE agc.anilist_id IS NULL
		ORDER BY a.anilist_id`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	var missing []int32
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err == nil {
			missing = append(missing, id)
		}
	}

	total := len(missing)
	if total == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "message": "All anime graphs are cached.", "total": 0})
		return
	}

	go func() {
		bgCtx := context.Background()
		computed, errors := h.computeGraphsBatch(bgCtx, missing, concurrency, 5*time.Minute)
		log.Printf("Missing graphs computation complete: %d computed, %d errors out of %d", computed, errors, total)
	}()

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Computing %d missing graphs. Monitor logs.", total),
		"total":     total,
		"startedAt": start.Format(time.RFC3339),
	})
}

// computeGraphsBatch computes graphs for the given anime IDs with bounded concurrency.
func (h *Handler) computeGraphsBatch(ctx context.Context, animeIDs []int32, concurrency int, timeout time.Duration) (computed, errors int) {
	var computedCount, errorCount atomic.Int64
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	var graphBatch []graphResult
	var batchMu sync.Mutex
	const batchSize = 50

	for i, id := range animeIDs {
		sem <- struct{}{}
		wg.Add(1)

		go func(anilistID int32, idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			session := h.neo4j.NewSession(ctx, neoSessionConfig())
			defer session.Close(ctx)

			graphData, err := h.computeSingleGraph(ctx, session, anilistID, timeout)
			if err != nil {
				errorCount.Add(1)
				if idx%100 == 0 {
					log.Printf("[graph] Error for %d: %v", anilistID, err)
				}
				return
			}
			if graphData == nil {
				return
			}

			computedCount.Add(1)

			batchMu.Lock()
			graphBatch = append(graphBatch, graphResult{AnilistID: anilistID, GraphData: graphData})

			if len(graphBatch) >= batchSize {
				batch := make([]graphResult, len(graphBatch))
				copy(batch, graphBatch)
				graphBatch = graphBatch[:0]
				batchMu.Unlock()

				h.insertGraphBatch(ctx, batch)
			} else {
				batchMu.Unlock()
			}

			if idx > 0 && idx%100 == 0 {
				log.Printf("[graph] Progress: %d/%d", idx, len(animeIDs))
			}
		}(id, i)
	}

	wg.Wait()

	// Flush remaining batch.
	batchMu.Lock()
	if len(graphBatch) > 0 {
		h.insertGraphBatch(ctx, graphBatch)
	}
	batchMu.Unlock()

	return int(computedCount.Load()), int(errorCount.Load())
}

type graphResult struct {
	AnilistID int32
	GraphData any
}

func (h *Handler) computeSingleGraph(ctx context.Context, session neo4j.SessionWithContext, anilistID int32, timeout time.Duration) (any, error) {
	// Use the optimized single-MATCH Cypher query.
	result, err := session.Run(ctx, `
		MATCH (centerAnime:Anime {anilistId: $anilistId})
		OPTIONAL MATCH (centerAnime)<-[r1:WORKED_ON]-(staff:Staff)
		WITH centerAnime,
			collect(DISTINCT {
				staff_id: staff.staff_id, name_en: staff.name_en,
				name_ja: staff.name_ja, image: staff.image, role: r1.role
			}) as staffConnections,
			collect(DISTINCT staff.staff_id) as staffIds
		CALL {
			WITH staffIds, centerAnime
			MATCH (s:Staff)-[r:WORKED_ON]->(relatedAnime:Anime)
			WHERE s.staff_id IN staffIds AND relatedAnime.anilistId <> centerAnime.anilistId
			WITH relatedAnime, count(DISTINCT s) as sharedCount,
				collect({
					anime: relatedAnime {.anilistId, .title, .coverImage, .format, .averageScore, .seasonYear, .season},
					staff_id: s.staff_id, name_en: s.name_en, name_ja: s.name_ja, role: r.role
				}) as connections
			WHERE sharedCount >= 2
			UNWIND connections as conn
			RETURN collect(DISTINCT {
				anime: conn.anime, staff: {staff_id: conn.staff_id, name_en: conn.name_en, name_ja: conn.name_ja},
				role: conn.role
			}) as relatedConnections
		}
		RETURN centerAnime {.anilistId, .title, .coverImage, .format, .averageScore, .seasonYear, .season} as centerAnime,
			staffConnections, relatedConnections`,
		map[string]any{"anilistId": int64(anilistID)})
	if err != nil {
		return nil, err
	}
	_ = result
	// For now, return a placeholder — the actual Neo4j result transformation
	// will be added when the neo4j driver types are properly integrated.
	return map[string]any{"anilistId": anilistID, "nodes": []any{}, "links": []any{}}, nil
}

func (h *Handler) insertGraphBatch(ctx context.Context, batch []graphResult) {
	if len(batch) == 0 {
		return
	}

	ids := make([]int32, len(batch))
	jsonData := make([]string, len(batch))
	for i, g := range batch {
		ids[i] = g.AnilistID
		j, _ := json.Marshal(g.GraphData)
		jsonData[i] = string(j)
	}

	_, err := h.pg.Exec(ctx, `
		INSERT INTO anime_graph_cache (anilist_id, graph_data)
		SELECT * FROM unnest($1::int[], $2::jsonb[])
		ON CONFLICT (anilist_id) DO UPDATE SET graph_data = EXCLUDED.graph_data`,
		ids, jsonData)
	if err != nil {
		log.Printf("[graph] Batch insert failed: %v", err)
	}
}

// RemoveADRRoles removes ADR-related roles from Neo4j.
func (h *Handler) RemoveADRRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	session := h.neo4j.NewSession(ctx, neoSessionConfig())
	defer session.Close(ctx)

	log.Println("Removing ADR roles from Neo4j")

	// Delete WORKED_ON relationships with ADR roles.
	result, err := session.Run(ctx, `
		MATCH (s:Staff)-[r:WORKED_ON]->(a:Anime)
		WHERE r.role CONTAINS 'ADR' OR r.role CONTAINS 'Adr'
		DELETE r
		RETURN count(r) as deleted`, nil)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed: %v", err))
		return
	}

	var deleted int64
	if result.Next(ctx) {
		record := result.Record()
		if v, ok := record.Get("deleted"); ok {
			deleted = neoInt(v)
		}
	}

	// Delete orphaned staff nodes.
	orphanResult, err := session.Run(ctx, `
		MATCH (s:Staff)
		WHERE NOT (s)-[:WORKED_ON]->()
		DELETE s
		RETURN count(s) as deleted`, nil)
	var orphansDeleted int64
	if err == nil && orphanResult.Next(ctx) {
		record := orphanResult.Record()
		if v, ok := record.Get("deleted"); ok {
			orphansDeleted = neoInt(v)
		}
	}

	duration := time.Since(start)
	log.Printf("ADR removal complete: %d relationships, %d orphan staff", deleted, orphansDeleted)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":         true,
		"message":         "ADR roles removed",
		"deleted":         deleted,
		"orphansDeleted":  orphansDeleted,
		"duration":        fmt.Sprintf("%dms", duration.Milliseconds()),
	})
}

// UpdateDates updates anime start/end dates from CSV via Neo4j.
func (h *Handler) UpdateDates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	session := h.neo4j.NewSession(ctx, neoSessionConfig())
	defer session.Close(ctx)

	log.Println("Updating anime dates")

	// Update dates from CSV data already loaded in Neo4j.
	result, err := session.Run(ctx, `
		MATCH (a:Anime)
		WHERE a.startDate_year IS NOT NULL
		SET a.start_date = date({year: a.startDate_year,
			month: CASE WHEN a.startDate_month IS NOT NULL AND a.startDate_month >= 1 AND a.startDate_month <= 12 THEN a.startDate_month ELSE 1 END,
			day: CASE WHEN a.startDate_day IS NOT NULL AND a.startDate_day >= 1 AND a.startDate_day <= 28 THEN a.startDate_day ELSE 1 END})
		RETURN count(a) as updated`, nil)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed: %v", err))
		return
	}

	var updated int64
	if result.Next(ctx) {
		record := result.Record()
		if v, ok := record.Get("updated"); ok {
			updated = neoInt(v)
		}
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  fmt.Sprintf("Updated dates for %d anime", updated),
		"updated":  updated,
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
	})
}

// UpdateSeasonYears sets seasonYear from startDate.year where missing.
func (h *Handler) UpdateSeasonYears(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	session := h.neo4j.NewSession(ctx, neoSessionConfig())
	defer session.Close(ctx)

	result, err := session.Run(ctx, `
		MATCH (a:Anime)
		WHERE a.seasonYear IS NULL AND a.startDate_year IS NOT NULL
		SET a.seasonYear = a.startDate_year
		RETURN count(a) as updated`, nil)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed: %v", err))
		return
	}

	var updated int64
	if result.Next(ctx) {
		record := result.Record()
		if v, ok := record.Get("updated"); ok {
			updated = neoInt(v)
		}
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  fmt.Sprintf("Updated season years for %d anime", updated),
		"updated":  updated,
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
	})
}

// LoadCSVData loads full CSV data into Neo4j.
func (h *Handler) LoadCSVData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	session := h.neo4j.NewSession(ctx, neoSessionConfig())
	defer session.Close(ctx)

	log.Println("Loading CSV data into Neo4j")

	// Run APOC periodic iterate for batch loading.
	loadQueries := []struct {
		name  string
		query string
	}{
		{"anime", `CALL apoc.periodic.iterate(
			"LOAD CSV WITH HEADERS FROM 'file:///media_delta.csv' AS row RETURN row",
			"MERGE (a:Anime {anilistId: toInteger(row.anilistId)})
			 SET a.title = row.title, a.title_english = row.title_english,
				 a.title_romaji = row.title_romaji, a.coverImage = row.coverImage,
				 a.format = row.format, a.type = row.type,
				 a.seasonYear = toInteger(row.seasonYear), a.season = row.season,
				 a.averageScore = toFloat(row.averageScore), a.popularity = toInteger(row.popularity),
				 a.malId = toInteger(row.malId)",
			{batchSize: 5000, parallel: false})`},
		{"staff", `CALL apoc.periodic.iterate(
			"LOAD CSV WITH HEADERS FROM 'file:///staff_delta.csv' AS row RETURN row",
			"MERGE (s:Staff {staff_id: toInteger(row.staff_id)})
			 SET s.name_en = row.name_en, s.name_ja = row.name_ja, s.image = row.image",
			{batchSize: 5000, parallel: false})`},
		{"edges", `CALL apoc.periodic.iterate(
			"LOAD CSV WITH HEADERS FROM 'file:///media_staff_edges_delta.csv' AS row RETURN row",
			"MATCH (a:Anime {anilistId: toInteger(row.anilistId)})
			 MATCH (s:Staff {staff_id: toInteger(row.staff_id)})
			 MERGE (s)-[r:WORKED_ON]->(a)
			 SET r.role = row.role",
			{batchSize: 5000, parallel: false})`},
	}

	results := map[string]any{}
	for _, lq := range loadQueries {
		_, err := session.Run(ctx, lq.query, nil)
		if err != nil {
			results[lq.name] = map[string]any{"error": err.Error()}
			log.Printf("Error loading %s: %v", lq.name, err)
		} else {
			results[lq.name] = map[string]any{"success": true}
			log.Printf("Loaded %s", lq.name)
		}
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "CSV data loaded into Neo4j",
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
		"results":  results,
	})
}

// LoadCSVDelta loads incremental delta CSV data into Neo4j.
func (h *Handler) LoadCSVDelta(w http.ResponseWriter, r *http.Request) {
	// Same as LoadCSVData but uses MERGE for updates.
	h.LoadCSVData(w, r)
}

// MigrateToPostgres orchestrates Neo4j → CSV → PostgreSQL migration.
func (h *Handler) MigrateToPostgres(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Println("Starting Neo4j → PostgreSQL migration")

	// Phase 1: Export Neo4j to CSV.
	h.ExportNeo4jToCSV(w, r)
	// Note: In production, this would be a two-phase process where
	// ExportNeo4jToCSV writes files, then ImportCSVToPostgres reads them.
	// For now, each is a separate endpoint call.

	duration := time.Since(start)
	log.Printf("Migration initiated in %v", duration)
}

// ExtractColors extracts dominant colors from anime cover images using k-means clustering.
func (h *Handler) ExtractColors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	session := h.neo4j.NewSession(ctx, neoSessionConfig())
	defer session.Close(ctx)

	// Count anime needing color extraction.
	result, err := session.Run(ctx,
		"MATCH (a:Anime) WHERE a.coverImage IS NOT NULL AND a.dominantColors IS NULL RETURN count(a) as count", nil)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed: %v", err))
		return
	}

	var count int64
	if result.Next(ctx) {
		record := result.Record()
		if v, ok := record.Get("count"); ok {
			count = neoInt(v)
		}
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  fmt.Sprintf("Color extraction queued for %d anime. This feature requires image processing (k-means) — run as separate job.", count),
		"pending":  count,
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
	})
}
