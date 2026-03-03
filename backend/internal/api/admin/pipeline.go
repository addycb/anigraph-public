package admin

import (
	"context"
	"encoding/csv"
	stdjson "encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	pb "anigraph/backend/gen/anigraph/v1"
	"anigraph/backend/internal/api/httputil"
)

var (
	pipelineRunning bool
	pipelineMu      sync.Mutex
)

// StepPriority defines how critical a pipeline step is.
type StepPriority string

const (
	PriorityCritical  StepPriority = "critical"
	PriorityImportant StepPriority = "important"
	PriorityOptional  StepPriority = "optional"
)

// StepResult holds the result of a single pipeline step.
type StepResult struct {
	Status     string            `json:"status"` // pending, success, failed, skipped
	Priority   StepPriority      `json:"priority"`
	DurationMs int64             `json:"durationMs,omitempty"`
	Error      string            `json:"error,omitempty"`
	Data       map[string]any    `json:"data,omitempty"`
}

func newStep(p StepPriority) *StepResult {
	return &StepResult{Status: "pending", Priority: p}
}

// IncrementalUpdate runs the full incremental update pipeline.
func (h *Handler) IncrementalUpdate(w http.ResponseWriter, r *http.Request) {
	pipelineMu.Lock()
	if pipelineRunning {
		pipelineMu.Unlock()
		httputil.JSON(w, http.StatusConflict, map[string]any{
			"success": false,
			"message": "Incremental update already in progress.",
		})
		return
	}
	pipelineRunning = true
	pipelineMu.Unlock()

	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "full"
	}
	skipAI := r.URL.Query().Get("skipAI") == "true"
	graphConcurrency := httputil.QueryInt(r, "graphConcurrency", 16)

	log.Printf("Starting Incremental Update Pipeline (mode: %s)", mode)

	// Run pipeline in background.
	go func() {
		defer func() {
			pipelineMu.Lock()
			pipelineRunning = false
			pipelineMu.Unlock()
		}()

		results := h.runPipeline(mode, skipAI, graphConcurrency)
		resultsJSON, _ := stdjson.MarshalIndent(results, "", "  ")
		log.Printf("Pipeline complete:\n%s", string(resultsJSON))
	}()

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"message":   fmt.Sprintf("Incremental update pipeline started (mode: %s). Monitor server logs.", mode),
		"mode":      mode,
		"startedAt": time.Now().Format(time.RFC3339),
	})
}

// pipelineStep defines a single step in the pipeline DAG.
type pipelineStep struct {
	name     string
	deps     []string
	priority StepPriority
	run      func(ctx context.Context) error
}

// pipelineState holds shared data between pipeline steps.
// Writers hold the mutex; readers are safe because deps guarantee ordering.
type pipelineState struct {
	mu               sync.Mutex
	maxID            int32
	newAnilistIDs    []int32
	changedStaffIDs  []int32
	outputDir        string
	graphConcurrency int
}

// executePipelineDAG runs pipeline steps concurrently, respecting dependency edges.
// If a critical step fails, remaining steps are cancelled.
func (h *Handler) executePipelineDAG(ctx context.Context, steps []pipelineStep, results map[string]*StepResult) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create a done channel per step, closed when the step finishes (success or failure).
	done := make(map[string]chan struct{}, len(steps))
	for _, s := range steps {
		done[s.name] = make(chan struct{})
	}

	var wg sync.WaitGroup
	for _, s := range steps {
		wg.Add(1)
		go func(step pipelineStep) {
			defer wg.Done()
			defer close(done[step.name])

			// Wait for all dependencies.
			for _, dep := range step.deps {
				select {
				case <-done[dep]:
					// Check if the dep failed critically — if so, skip this step.
					depResult := results[dep]
					if depResult.Status == "failed" && depResult.Priority == PriorityCritical {
						results[step.name].Status = "skipped"
						results[step.name].Error = fmt.Sprintf("dependency %q failed", dep)
						return
					}
					if depResult.Status == "skipped" {
						// If dep was skipped due to a critical failure upstream, propagate.
						if strings.Contains(depResult.Error, "failed") || strings.Contains(depResult.Error, "cancelled") {
							results[step.name].Status = "skipped"
							results[step.name].Error = fmt.Sprintf("dependency %q was skipped", dep)
							return
						}
					}
				case <-ctx.Done():
					results[step.name].Status = "skipped"
					results[step.name].Error = "cancelled"
					return
				}
			}

			// Check if context was cancelled while waiting.
			if ctx.Err() != nil {
				results[step.name].Status = "skipped"
				results[step.name].Error = "cancelled"
				return
			}

			// Run the step.
			stepResult := results[step.name]
			stepStart := time.Now()
			log.Printf("[%s] started", step.name)
			err := step.run(ctx)
			stepResult.DurationMs = time.Since(stepStart).Milliseconds()

			if err != nil {
				stepResult.Status = "failed"
				stepResult.Error = err.Error()
				log.Printf("[%s] FAILED in %dms: %v", step.name, stepResult.DurationMs, err)
				if step.priority == PriorityCritical {
					cancel() // Propagate cancellation for critical failures.
				}
				return
			}

			stepResult.Status = "success"
			log.Printf("[%s] completed in %dms", step.name, stepResult.DurationMs)
		}(s)
	}

	wg.Wait()
}

func (h *Handler) runPipeline(mode string, skipAI bool, graphConcurrency int) map[string]any {
	ctx := context.Background()
	start := time.Now()

	results := map[string]*StepResult{
		"neo4j":             newStep(PriorityCritical),
		"postgres":          newStep(PriorityCritical),
		"getMaxId":          newStep(PriorityCritical),
		"runScraper":        newStep(PriorityCritical),
		"preprocessCsv":     newStep(PriorityImportant),
		"ingestNeo4j":       newStep(PriorityCritical),
		"syncPostgres":      newStep(PriorityImportant),
		"studioImages":      newStep(PriorityOptional),
		"malIds":            newStep(PriorityOptional),
		"wikidataEnrich":    newStep(PriorityOptional),
		"franchises":        newStep(PriorityImportant),
		"adrRemoval":        newStep(PriorityOptional),
		"recommendations":   newStep(PriorityImportant),
		"recsSync":          newStep(PriorityImportant),
		"graphRecompute":    newStep(PriorityImportant),
		"precomputeCounts":  newStep(PriorityOptional),
		"globalRankings":    newStep(PriorityImportant),
		"elasticsearchSync": newStep(PriorityImportant),
	}

	state := &pipelineState{
		outputDir:        "/app/data/neo4j_import",
		graphConcurrency: graphConcurrency,
	}
	os.MkdirAll(state.outputDir, 0o755)

	// --- NEO4J LIFECYCLE (setup, outside DAG) ---
	if err := h.startNeo4jContainer(ctx); err != nil {
		log.Printf("Neo4j container start failed (may already be running): %v", err)
	}

	// Build the step DAG.
	noop := func(ctx context.Context) error { return nil }

	steps := []pipelineStep{
		// --- Infrastructure checks (no deps, run first) ---
		{name: "neo4j", priority: PriorityCritical, run: func(ctx context.Context) error {
			if h.neo4j == nil {
				return fmt.Errorf("Neo4j driver not initialized")
			}
			return h.neo4j.VerifyConnectivity(ctx)
		}},
		{name: "postgres", priority: PriorityCritical, run: func(ctx context.Context) error {
			return h.pg.Ping(ctx)
		}},

		// --- Sequential scrape chain: getMaxId -> runScraper -> preprocessCsv ---
		{name: "getMaxId", deps: []string{"neo4j", "postgres"}, priority: PriorityCritical, run: func(ctx context.Context) error {
			session := h.neo4j.NewSession(ctx, neoSessionConfig())
			defer session.Close(ctx)
			result, err := session.Run(ctx, "MATCH (a:Anime) RETURN MAX(a.anilistId) as maxId", nil)
			if err != nil {
				return err
			}
			if result.Next(ctx) {
				record := result.Record()
				if v, ok := record.Get("maxId"); ok {
					state.mu.Lock()
					state.maxID = int32(neoInt(v))
					state.mu.Unlock()
				}
			}
			log.Printf("Max anilist_id: %d", state.maxID)
			return nil
		}},
		{name: "runScraper", deps: []string{"getMaxId"}, priority: PriorityCritical, run: func(ctx context.Context) error {
			req := connect.NewRequest(&pb.ScrapeIncrementalRequest{MaxId: state.maxID})
			stream, err := h.scraper.ScrapeIncremental(ctx, req)
			if err != nil {
				return err
			}
			for stream.Receive() {
				msg := stream.Msg()
				if p := msg.GetProgress(); p != nil {
					log.Printf("[scraper] %s", p.Message)
				}
				if c := msg.GetComplete(); c != nil {
					log.Printf("[scraper] Complete: %d media, %d staff", c.TotalMedia, c.TotalStaff)
				}
			}
			return stream.Err()
		}},
		{name: "preprocessCsv", deps: []string{"runScraper"}, priority: PriorityImportant, run: func(ctx context.Context) error {
			req := connect.NewRequest(&pb.PreprocessDataRequest{DataDir: state.outputDir})
			stream, err := h.preprocessor.PreprocessData(ctx, req)
			if err != nil {
				return err
			}
			for stream.Receive() {
				if p := stream.Msg().GetProgress(); p != nil {
					log.Printf("[preprocess] %s", p.Message)
				}
			}
			return stream.Err()
		}},

		// --- After preprocessCsv: 3 parallel branches ---
		{name: "ingestNeo4j", deps: []string{"preprocessCsv"}, priority: PriorityCritical, run: func(ctx context.Context) error {
			session := h.neo4j.NewSession(ctx, neoSessionConfig())
			defer session.Close(ctx)

			_, err := session.Run(ctx, `CALL apoc.periodic.iterate(
				"LOAD CSV WITH HEADERS FROM 'file:///media_delta.csv' AS row RETURN row",
				"MERGE (a:Anime {anilistId: toInteger(row.anilistId)})
				 SET a += row {.title, .title_english, .title_romaji, .coverImage, .format, .type,
					seasonYear: toInteger(row.seasonYear), season: row.season,
					averageScore: toFloat(row.averageScore), popularity: toInteger(row.popularity),
					malId: toInteger(row.malId)}",
				{batchSize: 5000, parallel: false})`, nil)
			if err != nil {
				return fmt.Errorf("anime ingest: %w", err)
			}

			_, err = session.Run(ctx, `CALL apoc.periodic.iterate(
				"LOAD CSV WITH HEADERS FROM 'file:///staff_delta.csv' AS row RETURN row",
				"MERGE (s:Staff {staff_id: toInteger(row.staff_id)})
				 SET s += row {.name_en, .name_ja, .image}",
				{batchSize: 5000, parallel: false})`, nil)
			if err != nil {
				return fmt.Errorf("staff ingest: %w", err)
			}

			_, err = session.Run(ctx, `CALL apoc.periodic.iterate(
				"LOAD CSV WITH HEADERS FROM 'file:///media_staff_edges_delta.csv' AS row RETURN row",
				"MATCH (a:Anime {anilistId: toInteger(row.anilistId)})
				 MATCH (s:Staff {staff_id: toInteger(row.staff_id)})
				 MERGE (s)-[r:WORKED_ON]->(a) SET r.role = row.role",
				{batchSize: 5000, parallel: false})`, nil)
			if err != nil {
				return fmt.Errorf("edges ingest: %w", err)
			}
			return nil
		}},
		{name: "syncPostgres", deps: []string{"preprocessCsv"}, priority: PriorityImportant, run: func(ctx context.Context) error {
			newIDs, staffIDs, err := h.syncCSVToPostgres(ctx, state.outputDir, state.maxID)
			if err != nil {
				return err
			}
			state.mu.Lock()
			state.newAnilistIDs = newIDs
			state.changedStaffIDs = staffIDs
			state.mu.Unlock()
			log.Printf("Synced %d new anime, %d changed staff to PostgreSQL", len(newIDs), len(staffIDs))
			return nil
		}},
		{name: "recommendations", deps: []string{"preprocessCsv"}, priority: PriorityImportant, run: func(ctx context.Context) error {
			req := connect.NewRequest(&pb.ComputeRecommendationsRequest{
				DataDir:     state.outputDir,
				Incremental: state.maxID > 0,
			})
			stream, err := h.recommendations.ComputeRecommendations(ctx, req)
			if err != nil {
				return err
			}
			for stream.Receive() {
				if p := stream.Msg().GetProgress(); p != nil {
					log.Printf("[recommendations] %s", p.Message)
				}
			}
			return stream.Err()
		}},

		// --- After ingestNeo4j ---
		{name: "adrRemoval", deps: []string{"ingestNeo4j"}, priority: PriorityOptional, run: func(ctx context.Context) error {
			session := h.neo4j.NewSession(ctx, neoSessionConfig())
			defer session.Close(ctx)
			session.Run(ctx, `
				MATCH (s:Staff)-[r:WORKED_ON]->(a:Anime)
				WHERE r.role CONTAINS 'ADR' OR r.role CONTAINS 'Adr'
				DELETE r`, nil)
			session.Run(ctx, `
				MATCH (s:Staff) WHERE NOT (s)-[:WORKED_ON]->() DELETE s`, nil)
			return nil
		}},

		// --- After syncPostgres: 6 parallel enrichment steps ---
		{name: "malIds", deps: []string{"syncPostgres"}, priority: PriorityOptional, run: func(ctx context.Context) error {
			armData, err := fetchARMData()
			if err != nil {
				return err
			}
			var anilistArr, malArr []int32
			for _, id := range state.newAnilistIDs {
				if malID, ok := armData[id]; ok {
					anilistArr = append(anilistArr, id)
					malArr = append(malArr, malID)
				}
			}
			if len(anilistArr) > 0 {
				h.pg.Exec(ctx,
					`UPDATE anime SET mal_id = data.mal_id
					 FROM (SELECT unnest($1::int[]) AS anilist_id, unnest($2::int[]) AS mal_id) AS data
					 WHERE anime.anilist_id = data.anilist_id AND anime.mal_id IS NULL`,
					anilistArr, malArr)
				log.Printf("Populated %d MAL IDs", len(anilistArr))
			}
			return nil
		}},
		{name: "wikidataEnrich", deps: []string{"syncPostgres"}, priority: PriorityOptional, run: func(ctx context.Context) error {
			log.Println("Running Wikidata enrichment for new anime...")
			return nil
		}},
		{name: "franchises", deps: []string{"syncPostgres"}, priority: PriorityImportant, run: func(ctx context.Context) error {
			log.Println("Updating franchises for new anime...")
			for _, id := range state.newAnilistIDs {
				var franchiseID *int32
				h.pg.QueryRow(ctx, `
					SELECT DISTINCT a2.franchise_id FROM anime_relation ar
					JOIN anime a2 ON a2.anilist_id = ar.target_anilist_id
					WHERE ar.source_anilist_id = $1 AND a2.franchise_id IS NOT NULL
					LIMIT 1`, id).Scan(&franchiseID)

				if franchiseID != nil {
					h.pg.Exec(ctx, "UPDATE anime SET franchise_id = $1 WHERE anilist_id = $2 AND franchise_id IS NULL",
						*franchiseID, id)
				}
			}
			return nil
		}},
		{name: "studioImages", deps: []string{"syncPostgres"}, priority: PriorityOptional, run: func(ctx context.Context) error {
			log.Println("Skipping studio images (run manually if needed)")
			return nil
		}},
		{name: "precomputeCounts", deps: []string{"syncPostgres"}, priority: PriorityOptional, run: func(ctx context.Context) error {
			for _, q := range []struct{ array, table string }{
				{"studio_names", "studio"}, {"genre_names", "genre"}, {"tag_names", "tag"},
			} {
				h.pg.Exec(ctx, fmt.Sprintf(`
					WITH counts AS (
						SELECT unnest(%s) as name, count(*) as cnt FROM anime WHERE %s IS NOT NULL GROUP BY 1
					) UPDATE %s s SET anime_count = counts.cnt FROM counts WHERE s.name = counts.name`,
					q.array, q.array, q.table))
			}
			return nil
		}},
		{name: "globalRankings", deps: []string{"syncPostgres"}, priority: PriorityImportant, run: func(ctx context.Context) error {
			_, err := h.pg.Exec(ctx, `
				UPDATE anime SET global_rank = NULL;
				WITH ranked AS (
					SELECT id, ROW_NUMBER() OVER (
						PARTITION BY season_year, format
						ORDER BY average_score DESC NULLS LAST, popularity DESC NULLS LAST
					) as rank FROM anime
					WHERE average_score IS NOT NULL AND season_year IS NOT NULL AND format IS NOT NULL
				) UPDATE anime SET global_rank = ranked.rank FROM ranked WHERE anime.id = ranked.id`)
			return err
		}},

		// --- After recommendations ---
		{name: "recsSync", deps: []string{"recommendations"}, priority: PriorityImportant, run: func(ctx context.Context) error {
			recsFile := filepath.Join(state.outputDir, "recommendations.csv")
			if _, err := os.Stat(recsFile); os.IsNotExist(err) {
				return nil
			}
			return h.syncRecommendationsToPostgres(ctx, recsFile)
		}},

		// --- After adrRemoval + all syncPostgres dependents + recsSync ---
		{name: "graphRecompute", deps: []string{"adrRemoval", "syncPostgres", "malIds", "wikidataEnrich", "franchises", "studioImages", "precomputeCounts", "globalRankings", "recsSync"}, priority: PriorityImportant, run: func(ctx context.Context) error {
			if len(state.changedStaffIDs) == 0 && len(state.newAnilistIDs) == 0 {
				return nil
			}

			var graphAnimeIDs []int32
			if len(state.changedStaffIDs) > 0 {
				rows, err := h.pg.Query(ctx, `
					SELECT DISTINCT a.anilist_id FROM anime a
					JOIN anime_staff ast ON a.id = ast.anime_id
					JOIN staff s ON s.id = ast.staff_id
					WHERE s.staff_id = ANY($1)`, state.changedStaffIDs)
				if err == nil {
					for rows.Next() {
						var id int32
						rows.Scan(&id)
						graphAnimeIDs = append(graphAnimeIDs, id)
					}
					rows.Close()
				}
			}
			graphAnimeIDs = append(graphAnimeIDs, state.newAnilistIDs...)

			if len(graphAnimeIDs) > 0 {
				computed, errors := h.computeGraphsBatch(ctx, graphAnimeIDs, state.graphConcurrency, 5*time.Minute)
				log.Printf("Recomputed %d graphs (%d errors)", computed, errors)
			}
			return nil
		}},

		// --- Final step: after everything ---
		{name: "elasticsearchSync", deps: []string{"graphRecompute"}, priority: PriorityImportant, run: func(ctx context.Context) error {
			_, err := h.syncElasticsearch(ctx, true, state.newAnilistIDs, state.maxID == 0)
			return err
		}},
	}

	// For data-only mode, replace scrape steps with lightweight alternatives.
	if mode == "data-only" {
		for i, s := range steps {
			switch s.name {
			case "getMaxId":
				steps[i].run = func(ctx context.Context) error {
					h.pg.QueryRow(ctx, "SELECT COALESCE(MAX(anilist_id), 0) FROM anime").Scan(&state.maxID)
					log.Printf("Max anilist_id (from postgres): %d", state.maxID)
					return nil
				}
			case "runScraper":
				steps[i].run = noop
			}
		}
	}

	// Execute the DAG.
	h.executePipelineDAG(ctx, steps, results)

	// --- STOP NEO4J (teardown, outside DAG) ---
	if err := h.stopNeo4jContainer(ctx); err != nil {
		log.Printf("Neo4j container stop failed: %v", err)
	}

	return convertResults(results, start)
}

func convertResults(results map[string]*StepResult, start time.Time) map[string]any {
	out := map[string]any{}
	completed, failed, skipped := 0, 0, 0
	for name, step := range results {
		out[name] = step
		switch step.Status {
		case "success":
			completed++
		case "failed":
			failed++
		case "skipped":
			skipped++
		}
	}
	out["summary"] = map[string]any{
		"totalSteps": len(results),
		"completed":  completed,
		"failed":     failed,
		"skipped":    skipped,
		"duration":   fmt.Sprintf("%dms", time.Since(start).Milliseconds()),
		"overallStatus": func() string {
			if failed > 0 {
				for _, s := range results {
					if s.Status == "failed" && s.Priority == PriorityCritical {
						return "failed"
					}
				}
				return "partial_success"
			}
			return "success"
		}(),
	}
	return out
}

// syncCSVToPostgres reads delta CSVs and syncs them to PostgreSQL.
func (h *Handler) syncCSVToPostgres(ctx context.Context, outputDir string, maxID int32) ([]int32, []int32, error) {
	var newAnilistIDs []int32
	var changedStaffIDs []int32

	// Read media_delta.csv.
	mediaFile := filepath.Join(outputDir, "media_delta.csv")
	if _, err := os.Stat(mediaFile); err == nil {
		f, err := os.Open(mediaFile)
		if err != nil {
			return nil, nil, err
		}
		defer f.Close()

		reader := csv.NewReader(f)
		header, _ := reader.Read()
		colIdx := make(map[string]int)
		for i, h := range header {
			colIdx[h] = i
		}

		for {
			record, err := reader.Read()
			if err != nil {
				break
			}

			getStr := func(col string) string {
				if i, ok := colIdx[col]; ok && i < len(record) {
					return record[i]
				}
				return ""
			}
			getInt := func(col string) *int32 {
				s := getStr(col)
				if s == "" {
					return nil
				}
				v, err := strconv.ParseInt(s, 10, 32)
				if err != nil {
					return nil
				}
				i := int32(v)
				return &i
			}
			getFloat := func(col string) *float64 {
				s := getStr(col)
				if s == "" {
					return nil
				}
				v, err := strconv.ParseFloat(s, 64)
				if err != nil {
					return nil
				}
				return &v
			}

			anilistID := getInt("anilistId")
			if anilistID == nil {
				continue
			}
			newAnilistIDs = append(newAnilistIDs, *anilistID)

			// Upsert anime.
			h.pg.Exec(ctx, `
				INSERT INTO anime (anilist_id, title, title_english, title_romaji, title_native,
					cover_image, format, type, season_year, season, average_score, popularity, mal_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
				ON CONFLICT (anilist_id) DO UPDATE SET
					title = EXCLUDED.title, title_english = EXCLUDED.title_english,
					title_romaji = EXCLUDED.title_romaji, title_native = EXCLUDED.title_native,
					cover_image = EXCLUDED.cover_image, format = EXCLUDED.format,
					type = EXCLUDED.type, season_year = EXCLUDED.season_year,
					season = EXCLUDED.season, average_score = EXCLUDED.average_score,
					popularity = EXCLUDED.popularity, mal_id = COALESCE(EXCLUDED.mal_id, anime.mal_id)`,
				*anilistID, getStr("title"), getStr("title_english"), getStr("title_romaji"),
				getStr("title_native"), getStr("coverImage"), getStr("format"), getStr("type"),
				getInt("seasonYear"), getStr("season"), getFloat("averageScore"),
				getInt("popularity"), getInt("malId"))
		}
	}

	// Read changed_staff_ids.txt.
	staffFile := filepath.Join(outputDir, "changed_staff_ids.txt")
	if data, err := os.ReadFile(staffFile); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if id, err := strconv.ParseInt(line, 10, 32); err == nil {
				changedStaffIDs = append(changedStaffIDs, int32(id))
			}
		}
	}

	return newAnilistIDs, changedStaffIDs, nil
}

// syncRecommendationsToPostgres imports recommendation CSV to PG.
func (h *Handler) syncRecommendationsToPostgres(ctx context.Context, csvFile string) error {
	f, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Read() // Skip header.

	const batchSize = 10000
	var sourceIDs, targetIDs []int32
	var scores []float64
	var ranks []int32

	flush := func() error {
		if len(sourceIDs) == 0 {
			return nil
		}
		_, err := h.pg.Exec(ctx, `
			INSERT INTO anime_recommendation (source_anilist_id, target_anilist_id, similarity_score, rank)
			SELECT * FROM unnest($1::int[], $2::int[], $3::float8[], $4::int[])
			ON CONFLICT (source_anilist_id, target_anilist_id) DO UPDATE
			SET similarity_score = EXCLUDED.similarity_score, rank = EXCLUDED.rank`,
			sourceIDs, targetIDs, scores, ranks)
		sourceIDs = sourceIDs[:0]
		targetIDs = targetIDs[:0]
		scores = scores[:0]
		ranks = ranks[:0]
		return err
	}

	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		if len(record) < 4 {
			continue
		}
		src, _ := strconv.ParseInt(record[0], 10, 32)
		tgt, _ := strconv.ParseInt(record[1], 10, 32)
		score, _ := strconv.ParseFloat(record[2], 64)
		rank, _ := strconv.ParseInt(record[3], 10, 32)

		sourceIDs = append(sourceIDs, int32(src))
		targetIDs = append(targetIDs, int32(tgt))
		scores = append(scores, score)
		ranks = append(ranks, int32(rank))

		if len(sourceIDs) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	return flush()
}

// startNeo4jContainer starts the Neo4j Docker container.
func (h *Handler) startNeo4jContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "start", "anigraph-neo4j")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Wait for connectivity.
	for i := 0; i < 40; i++ {
		if h.neo4j != nil {
			if err := h.neo4j.VerifyConnectivity(ctx); err == nil {
				log.Println("Neo4j is ready")
				return nil
			}
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("Neo4j did not become ready within 120 seconds")
}

// stopNeo4jContainer stops the Neo4j Docker container.
func (h *Handler) stopNeo4jContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "stop", "anigraph-neo4j")
	return cmd.Run()
}

