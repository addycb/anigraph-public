package admin

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	pb "anigraph/backend/gen/anigraph/v1"
	"anigraph/backend/internal/api/httputil"
)

// ImportCSVToPostgres imports CSV files from Neo4j export to PostgreSQL using COPY.
func (h *Handler) ImportCSVToPostgres(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	skipRelationships := r.URL.Query().Get("skipRelationships") == "true"
	skipClear := r.URL.Query().Get("skipClear") == "true"
	only := r.URL.Query().Get("only")

	log.Println("Starting CSV import to PostgreSQL")

	dataDir := "/app/data/neo4j_import"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		dataDir = ".runtime/neo4j_import"
	}

	results := map[string]any{}

	// Clear tables if requested.
	if !skipClear && only == "" {
		log.Println("Clearing existing data...")
		tables := []string{"anime_recommendation", "anime_relation", "anime_staff",
			"anime_genre", "anime_tag", "anime_studio", "anime_graph_cache",
			"anime", "staff", "genre", "tag", "studio", "franchise"}
		for _, t := range tables {
			h.pg.Exec(ctx, fmt.Sprintf("TRUNCATE %s CASCADE", t))
		}
		results["cleared"] = true
	}

	// Import entities.
	entities := []struct {
		name, file, table string
		columns           []string
	}{
		{"genres", "genres.csv", "genre", []string{"name"}},
		{"tags", "tags.csv", "tag", []string{"name"}},
		{"studios", "studios.csv", "studio", []string{"name"}},
		{"franchises", "franchises.csv", "franchise", []string{"title"}},
		{"anime", "anime.csv", "anime", nil},
		{"staff", "staff.csv", "staff", nil},
	}

	for _, e := range entities {
		if only != "" && only != e.name {
			continue
		}
		csvPath := filepath.Join(dataDir, e.file)
		if _, err := os.Stat(csvPath); os.IsNotExist(err) {
			results[e.name] = map[string]any{"skipped": true, "reason": "file not found"}
			continue
		}

		count, err := importCSVFile(ctx, h, csvPath, e.table, e.columns)
		if err != nil {
			results[e.name] = map[string]any{"error": err.Error()}
			log.Printf("Error importing %s: %v", e.name, err)
		} else {
			results[e.name] = map[string]any{"imported": count}
			log.Printf("Imported %d %s", count, e.name)
		}
	}

	// Import junctions.
	if !skipRelationships && only == "" {
		junctions := []struct {
			name, file, table string
		}{
			{"anime_genre", "anime_genre.csv", "anime_genre"},
			{"anime_tag", "anime_tag.csv", "anime_tag"},
			{"anime_studio", "anime_studio.csv", "anime_studio"},
			{"anime_staff", "anime_staff.csv", "anime_staff"},
			{"anime_relation", "media_relations_delta.csv", "anime_relation"},
		}
		for _, j := range junctions {
			csvPath := filepath.Join(dataDir, j.file)
			if _, err := os.Stat(csvPath); os.IsNotExist(err) {
				results[j.name] = map[string]any{"skipped": true}
				continue
			}
			count, err := importCSVFile(ctx, h, csvPath, j.table, nil)
			if err != nil {
				results[j.name] = map[string]any{"error": err.Error()}
			} else {
				results[j.name] = map[string]any{"imported": count}
			}
		}
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "CSV import complete",
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
		"results":  results,
	})
}

func importCSVFile(ctx context.Context, h *Handler, csvPath, table string, columns []string) (int, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	// Read header.
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read header: %w", err)
	}

	if columns == nil {
		columns = header
	}

	count := 0
	const batchSize = 5000
	var values [][]any

	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		row := make([]any, len(record))
		for i, v := range record {
			if v == "" || v == "\\N" {
				row[i] = nil
			} else {
				row[i] = v
			}
		}
		values = append(values, row)
		count++

		if len(values) >= batchSize {
			if err := batchInsert(ctx, h, table, columns, values); err != nil {
				return count, err
			}
			values = values[:0]
		}
	}

	if len(values) > 0 {
		if err := batchInsert(ctx, h, table, columns, values); err != nil {
			return count, err
		}
	}

	return count, nil
}

func batchInsert(ctx context.Context, h *Handler, table string, columns []string, values [][]any) error {
	if len(values) == 0 {
		return nil
	}

	placeholders := make([]string, len(values))
	args := make([]any, 0, len(values)*len(columns))
	for i, row := range values {
		ph := make([]string, len(columns))
		for j := range columns {
			args = append(args, row[j])
			ph[j] = fmt.Sprintf("$%d", i*len(columns)+j+1)
		}
		placeholders[i] = "(" + strings.Join(ph, ",") + ")"
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s ON CONFLICT DO NOTHING",
		table, strings.Join(columns, ","), strings.Join(placeholders, ","))

	_, err := h.pg.Exec(ctx, sql, args...)
	return err
}

// IngestES ingests CSV files into Elasticsearch.
func (h *Handler) IngestES(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Println("Starting CSV-to-ES ingestion")

	dataDir := "/app/data/neo4j_import"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		dataDir = ".runtime/neo4j_import"
	}

	results := map[string]any{}

	// Ingest anime CSV.
	animeCSV := filepath.Join(dataDir, "anime.csv")
	if _, err := os.Stat(animeCSV); err == nil {
		records, err := readCSVAsRecords(animeCSV, func(header []string, fields []string) map[string]any {
			rec := csvFieldsToMap(header, fields)
			return map[string]any{
				"anilist_id":    rec["anilist_id"],
				"title":        coalesce(rec["title_english"], rec["title_romaji"], rec["title"]),
				"title_english": rec["title_english"],
				"title_romaji":  rec["title_romaji"],
				"cover_image":   rec["cover_image"],
				"format":        rec["format"],
				"season_year":   parseIntOrNil(rec["season_year"]),
				"season":        rec["season"],
				"average_score": parseFloatOrNil(rec["average_score"]),
			}
		})
		if err == nil && len(records) > 0 {
			indexed, failed := h.bulkIndexES("anime", records, func(r map[string]any) string {
				return fmt.Sprintf("%v", r["anilist_id"])
			})
			results["anime"] = map[string]any{"indexed": indexed, "failed": failed}
		}
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "CSV-to-ES ingestion complete",
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
		"results":  results,
	})
}

// PreprocessCSV preprocesses scraped CSV files using the preprocessor service.
func (h *Handler) PreprocessCSV(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting CSV preprocessing")

	dataDir := "/app/data/neo4j_import"
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		dataDir = ".runtime/neo4j_import"
	}

	req := connect.NewRequest(&pb.PreprocessDataRequest{DataDir: dataDir})
	stream, err := h.preprocessor.PreprocessData(r.Context(), req)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Preprocessing failed: %v", err))
		return
	}

	var lastMsg string
	for stream.Receive() {
		msg := stream.Msg()
		if p := msg.GetProgress(); p != nil {
			lastMsg = p.Message
			log.Printf("[preprocess] %s", p.Message)
		}
	}
	if err := stream.Err(); err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Preprocessing failed: %v", err))
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "CSV preprocessing complete",
		"lastMsg": lastMsg,
	})
}

// ExportNeo4jToCSV exports all Neo4j data to CSV format.
func (h *Handler) ExportNeo4jToCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	outputDir := "/app/data/neo4j_import"
	os.MkdirAll(outputDir, 0o755)

	log.Println("Starting Neo4j-to-CSV export")

	session := h.neo4j.NewSession(ctx, neo4jReadSessionConfig())
	defer session.Close(ctx)

	results := map[string]any{}

	// Export queries.
	exports := []struct {
		name    string
		query   string
		columns []string
	}{
		{"genres", "MATCH (g:Genre) RETURN g.name as name", []string{"name"}},
		{"tags", "MATCH (t:Tag) RETURN t.name as name", []string{"name"}},
		{"studios", "MATCH (s:Studio) RETURN s.name as name", []string{"name"}},
		{"anime", `MATCH (a:Anime)
			RETURN a.anilistId as anilist_id, a.title as title,
				a.title_english as title_english, a.title_romaji as title_romaji,
				a.coverImage as cover_image, a.format as format,
				a.seasonYear as season_year, a.season as season,
				a.averageScore as average_score, a.type as type,
				a.malId as mal_id`,
			[]string{"anilist_id", "title", "title_english", "title_romaji",
				"cover_image", "format", "season_year", "season",
				"average_score", "type", "mal_id"}},
		{"staff", `MATCH (s:Staff)
			RETURN s.staff_id as staff_id, s.name_en as name_en,
				s.name_ja as name_ja, s.image as image_large`,
			[]string{"staff_id", "name_en", "name_ja", "image_large"}},
	}

	for _, exp := range exports {
		result, err := session.Run(ctx, exp.query, nil)
		if err != nil {
			results[exp.name] = map[string]any{"error": err.Error()}
			continue
		}

		csvPath := filepath.Join(outputDir, exp.name+".csv")
		f, err := os.Create(csvPath)
		if err != nil {
			results[exp.name] = map[string]any{"error": err.Error()}
			continue
		}

		writer := csv.NewWriter(f)
		writer.Write(exp.columns)

		count := 0
		for result.Next(ctx) {
			record := result.Record()
			row := make([]string, len(exp.columns))
			for i, col := range exp.columns {
				val, _ := record.Get(col)
				row[i] = neoToString(val)
			}
			writer.Write(row)
			count++
		}

		writer.Flush()
		f.Close()
		results[exp.name] = map[string]any{"exported": count, "file": csvPath}
		log.Printf("Exported %d %s", count, exp.name)
	}

	// Export relationships.
	relExports := []struct {
		name, query string
		columns     []string
	}{
		{"anime_genre", `MATCH (a:Anime)-[:HAS_GENRE]->(g:Genre) RETURN a.anilistId as anilist_id, g.name as genre_name`,
			[]string{"anilist_id", "genre_name"}},
		{"anime_tag", `MATCH (a:Anime)-[:HAS_TAG]->(t:Tag) RETURN a.anilistId as anilist_id, t.name as tag_name`,
			[]string{"anilist_id", "tag_name"}},
		{"anime_studio", `MATCH (a:Anime)-[:PRODUCED_BY]->(s:Studio) RETURN a.anilistId as anilist_id, s.name as studio_name`,
			[]string{"anilist_id", "studio_name"}},
		{"anime_staff", `MATCH (s:Staff)-[r:WORKED_ON]->(a:Anime) RETURN a.anilistId as anilist_id, s.staff_id as staff_id, r.role as role`,
			[]string{"anilist_id", "staff_id", "role"}},
	}

	for _, exp := range relExports {
		result, err := session.Run(ctx, exp.query, nil)
		if err != nil {
			results[exp.name] = map[string]any{"error": err.Error()}
			continue
		}

		csvPath := filepath.Join(outputDir, exp.name+".csv")
		f, _ := os.Create(csvPath)
		writer := csv.NewWriter(f)
		writer.Write(exp.columns)

		count := 0
		for result.Next(ctx) {
			record := result.Record()
			row := make([]string, len(exp.columns))
			for i, col := range exp.columns {
				val, _ := record.Get(col)
				row[i] = neoToString(val)
			}
			writer.Write(row)
			count++
		}

		writer.Flush()
		f.Close()
		results[exp.name] = map[string]any{"exported": count}
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "Neo4j-to-CSV export complete",
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
		"results":  results,
	})
}

// ExportBackup exports Neo4j database using APOC.
func (h *Handler) ExportBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "cypher"
	}

	session := h.neo4j.NewSession(ctx, neo4jReadSessionConfig())
	defer session.Close(ctx)

	filename := fmt.Sprintf("backup_%s.%s", time.Now().Format("20060102_150405"), format)

	var query string
	switch format {
	case "graphml":
		query = fmt.Sprintf("CALL apoc.export.graphml.all('%s', {})", filename)
	case "csv":
		query = fmt.Sprintf("CALL apoc.export.csv.all('%s', {})", filename)
	default:
		query = fmt.Sprintf("CALL apoc.export.cypher.all('%s', {})", filename)
	}

	result, err := session.Run(ctx, query, nil)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Backup failed: %v", err))
		return
	}

	var nodes, rels int64
	if result.Next(ctx) {
		record := result.Record()
		if v, ok := record.Get("nodes"); ok {
			nodes = neoInt(v)
		}
		if v, ok := record.Get("relationships"); ok {
			rels = neoInt(v)
		}
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"message":       "Backup exported",
		"file":          filename,
		"format":        format,
		"nodes":         nodes,
		"relationships": rels,
		"duration":      fmt.Sprintf("%dms", duration.Milliseconds()),
	})
}

// Helper functions.

func neoToString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func readCSVAsRecords(path string, transform func([]string, []string) map[string]any) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Handle BOM.
	reader := bufio.NewReader(f)
	bom := make([]byte, 3)
	n, _ := reader.Read(bom)
	if n < 3 || bom[0] != 0xEF || bom[1] != 0xBB || bom[2] != 0xBF {
		// No BOM, reopen.
		f.Seek(0, 0)
		reader = bufio.NewReader(f)
	}

	csvReader := csv.NewReader(reader)
	header, err := csvReader.Read()
	if err != nil {
		return nil, err
	}

	var records []map[string]any
	for {
		fields, err := csvReader.Read()
		if err != nil {
			break
		}
		rec := transform(header, fields)
		if rec != nil {
			records = append(records, rec)
		}
	}
	return records, nil
}

func csvFieldsToMap(header, fields []string) map[string]string {
	m := make(map[string]string, len(header))
	for i, h := range header {
		if i < len(fields) {
			m[h] = fields[i]
		}
	}
	return m
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseIntOrNil(s string) any {
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return v
}

func parseFloatOrNil(s string) any {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return v
}

func staticCtx() context.Context {
	return context.Background()
}

// unused but kept for type compatibility
var _ = json.Marshal
