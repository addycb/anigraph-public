package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"anigraph/backend/internal/api/httputil"
)

// DBStats returns Neo4j tag count.
func (h *Handler) DBStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.neo4j == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "Neo4j not available")
		return
	}

	session := h.neo4j.NewSession(ctx, neoSessionConfig())
	defer session.Close(ctx)

	result, err := session.Run(ctx, "MATCH (t:Tag) RETURN count(t) as count", nil)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get database stats: %v", err))
		return
	}

	record, err := result.Single(ctx)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get database stats: %v", err))
		return
	}

	count, _ := record.Get("count")
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"tagCount": neoInt(count),
	})
}

// SetupGINIndexes creates GIN indexes for fast genre/tag/studio filtering.
func (h *Handler) SetupGINIndexes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	log.Println("Setting up GIN indexes for fast filtering")

	results := []string{}

	// 1. Add columns if they don't exist.
	columns := []struct{ name, typ string }{
		{"genre_names", "TEXT[]"},
		{"tag_names", "TEXT[]"},
		{"studio_names", "TEXT[]"},
	}
	for _, col := range columns {
		_, err := h.pg.Exec(ctx, fmt.Sprintf("ALTER TABLE anime ADD COLUMN IF NOT EXISTS %s %s", col.name, col.typ))
		if err != nil {
			httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add column %s: %v", col.name, err))
			return
		}
		results = append(results, fmt.Sprintf("Added column %s", col.name))
	}

	// 2. Create GIN indexes.
	indexes := []struct{ name, column string }{
		{"idx_anime_genre_names_gin", "genre_names"},
		{"idx_anime_tag_names_gin", "tag_names"},
		{"idx_anime_studio_names_gin", "studio_names"},
	}
	for _, idx := range indexes {
		_, err := h.pg.Exec(ctx, fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON anime USING gin (%s)", idx.name, idx.column))
		if err != nil {
			httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create index %s: %v", idx.name, err))
			return
		}
		results = append(results, fmt.Sprintf("Created index %s", idx.name))
	}

	// 3. Create trigger functions.
	triggerFunctions := []string{
		`CREATE OR REPLACE FUNCTION refresh_anime_genre_names(p_anime_id INTEGER)
		RETURNS VOID AS $$
		BEGIN
			UPDATE anime SET genre_names = (
				SELECT array_agg(g.name ORDER BY g.name)
				FROM anime_genre ag
				JOIN genre g ON ag.genre_id = g.id
				WHERE ag.anime_id = p_anime_id
			)
			WHERE id = p_anime_id;
		END;
		$$ LANGUAGE plpgsql`,

		`CREATE OR REPLACE FUNCTION sync_anime_genre_names()
		RETURNS TRIGGER AS $$
		BEGIN
			IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
				PERFORM refresh_anime_genre_names(NEW.anime_id);
				RETURN NEW;
			ELSIF TG_OP = 'DELETE' THEN
				PERFORM refresh_anime_genre_names(OLD.anime_id);
				RETURN OLD;
			END IF;
		END;
		$$ LANGUAGE plpgsql`,

		`CREATE OR REPLACE FUNCTION refresh_anime_tag_names(p_anime_id INTEGER)
		RETURNS VOID AS $$
		BEGIN
			UPDATE anime SET tag_names = (
				SELECT array_agg(t.name ORDER BY t.name)
				FROM anime_tag at
				JOIN tag t ON at.tag_id = t.id
				WHERE at.anime_id = p_anime_id
			)
			WHERE id = p_anime_id;
		END;
		$$ LANGUAGE plpgsql`,

		`CREATE OR REPLACE FUNCTION sync_anime_tag_names()
		RETURNS TRIGGER AS $$
		BEGIN
			IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
				PERFORM refresh_anime_tag_names(NEW.anime_id);
				RETURN NEW;
			ELSIF TG_OP = 'DELETE' THEN
				PERFORM refresh_anime_tag_names(OLD.anime_id);
				RETURN OLD;
			END IF;
		END;
		$$ LANGUAGE plpgsql`,

		`CREATE OR REPLACE FUNCTION refresh_anime_studio_names(p_anime_id INTEGER)
		RETURNS VOID AS $$
		BEGIN
			UPDATE anime SET studio_names = (
				SELECT array_agg(s.name ORDER BY s.name)
				FROM anime_studio ast
				JOIN studio s ON ast.studio_id = s.id
				WHERE ast.anime_id = p_anime_id
			)
			WHERE id = p_anime_id;
		END;
		$$ LANGUAGE plpgsql`,

		`CREATE OR REPLACE FUNCTION sync_anime_studio_names()
		RETURNS TRIGGER AS $$
		BEGIN
			IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
				PERFORM refresh_anime_studio_names(NEW.anime_id);
				RETURN NEW;
			ELSIF TG_OP = 'DELETE' THEN
				PERFORM refresh_anime_studio_names(OLD.anime_id);
				RETURN OLD;
			END IF;
		END;
		$$ LANGUAGE plpgsql`,
	}
	for _, fn := range triggerFunctions {
		if _, err := h.pg.Exec(ctx, fn); err != nil {
			httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create function: %v", err))
			return
		}
	}
	results = append(results, "Created/updated sync functions")

	// 4. Create triggers.
	triggers := []struct{ name, table, function string }{
		{"trigger_sync_genre_names", "anime_genre", "sync_anime_genre_names"},
		{"trigger_sync_tag_names", "anime_tag", "sync_anime_tag_names"},
		{"trigger_sync_studio_names", "anime_studio", "sync_anime_studio_names"},
	}
	for _, trig := range triggers {
		h.pg.Exec(ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", trig.name, trig.table))
		_, err := h.pg.Exec(ctx, fmt.Sprintf(
			"CREATE TRIGGER %s AFTER INSERT OR UPDATE OR DELETE ON %s FOR EACH ROW EXECUTE FUNCTION %s()",
			trig.name, trig.table, trig.function))
		if err != nil {
			httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create trigger %s: %v", trig.name, err))
			return
		}
	}
	results = append(results, "Created sync triggers")

	// 5. Populate arrays from existing data.
	type populateResult struct {
		name  string
		count int64
	}
	populateQueries := []struct {
		label string
		sql   string
	}{
		{"genre_names", `UPDATE anime a SET genre_names = subq.names
			FROM (SELECT ag.anime_id, array_agg(g.name ORDER BY g.name) as names
				FROM anime_genre ag JOIN genre g ON ag.genre_id = g.id GROUP BY ag.anime_id) subq
			WHERE a.id = subq.anime_id`},
		{"tag_names", `UPDATE anime a SET tag_names = subq.names
			FROM (SELECT at.anime_id, array_agg(t.name ORDER BY t.name) as names
				FROM anime_tag at JOIN tag t ON at.tag_id = t.id GROUP BY at.anime_id) subq
			WHERE a.id = subq.anime_id`},
		{"studio_names", `UPDATE anime a SET studio_names = subq.names
			FROM (SELECT ast.anime_id, array_agg(s.name ORDER BY s.name) as names
				FROM anime_studio ast JOIN studio s ON ast.studio_id = s.id GROUP BY ast.anime_id) subq
			WHERE a.id = subq.anime_id`},
	}

	counts := map[string]int64{}
	for _, pq := range populateQueries {
		tag, err := h.pg.Exec(ctx, pq.sql)
		if err != nil {
			httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed to populate %s: %v", pq.label, err))
			return
		}
		counts[pq.label] = tag.RowsAffected()
		results = append(results, fmt.Sprintf("Populated %s for %d anime", pq.label, tag.RowsAffected()))
	}

	duration := time.Since(start)
	log.Printf("GIN Index Setup Complete! Duration: %v", duration)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "GIN indexes set up successfully",
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
		"results":  results,
		"counts": map[string]any{
			"genresPopulated":  counts["genre_names"],
			"tagsPopulated":    counts["tag_names"],
			"studiosPopulated": counts["studio_names"],
		},
	})
}

// CreateIndices creates Elasticsearch indices with autocomplete analyzers.
func (h *Handler) CreateIndices(w http.ResponseWriter, r *http.Request) {
	recreate := r.URL.Query().Get("recreate") == "true"

	indexSettings := map[string]any{
		"analysis": map[string]any{
			"analyzer": map[string]any{
				"autocomplete_analyzer": map[string]any{
					"type":      "custom",
					"tokenizer": "autocomplete_tokenizer",
					"filter":    []string{"lowercase", "asciifolding"},
				},
				"autocomplete_search_analyzer": map[string]any{
					"type":      "custom",
					"tokenizer": "standard",
					"filter":    []string{"lowercase", "asciifolding"},
				},
			},
			"tokenizer": map[string]any{
				"autocomplete_tokenizer": map[string]any{
					"type":        "edge_ngram",
					"min_gram":    2,
					"max_gram":    20,
					"token_chars": []string{"letter", "digit"},
				},
			},
		},
	}

	autocompleteText := func(hasStandard bool) map[string]any {
		fields := map[string]any{"keyword": map[string]any{"type": "keyword"}}
		if hasStandard {
			fields["standard"] = map[string]any{"type": "text"}
		}
		return map[string]any{
			"type":            "text",
			"analyzer":        "autocomplete_analyzer",
			"search_analyzer": "autocomplete_search_analyzer",
			"fields":          fields,
		}
	}

	indices := []struct {
		name    string
		mapping map[string]any
	}{
		{"anime", map[string]any{"properties": map[string]any{
			"id": map[string]any{"type": "keyword"},
			"title": autocompleteText(true),
			"title_english": autocompleteText(false),
			"title_romaji":  autocompleteText(false),
			"title_native": map[string]any{"type": "text", "fields": map[string]any{"keyword": map[string]any{"type": "keyword"}}},
			"cover_image":   map[string]any{"type": "keyword"},
			"format":        map[string]any{"type": "keyword"},
			"season_year":   map[string]any{"type": "integer"},
			"season":        map[string]any{"type": "keyword"},
			"average_score":  map[string]any{"type": "float"},
		}}},
		{"staff", map[string]any{"properties": map[string]any{
			"id":      map[string]any{"type": "keyword"},
			"name":    autocompleteText(true),
			"name_en": autocompleteText(false),
			"name_ja": map[string]any{"type": "text", "fields": map[string]any{"keyword": map[string]any{"type": "keyword"}}},
			"picture": map[string]any{"type": "keyword"},
		}}},
		{"studios", map[string]any{"properties": map[string]any{
			"studio_id": map[string]any{"type": "keyword"},
			"name":      autocompleteText(true),
		}}},
	}

	results := map[string]any{}

	for _, idx := range indices {
		result := map[string]any{"created": false, "existed": false, "error": nil}

		// Check if index exists.
		existsRes, err := h.es.Indices.Exists([]string{idx.name})
		if err != nil {
			result["error"] = err.Error()
			results[idx.name] = result
			continue
		}
		existsRes.Body.Close()

		if existsRes.StatusCode == 200 {
			result["existed"] = true
			if recreate {
				delRes, err := h.es.Indices.Delete([]string{idx.name})
				if err != nil {
					result["error"] = err.Error()
					results[idx.name] = result
					continue
				}
				delRes.Body.Close()
				log.Printf("Deleted existing index: %s", idx.name)
			} else {
				result["error"] = "Index already exists. Use ?recreate=true to recreate."
				results[idx.name] = result
				continue
			}
		}

		// Create index.
		body := map[string]any{
			"settings": indexSettings,
			"mappings": idx.mapping,
		}
		bodyBytes, _ := json.Marshal(body)
		createRes, err := h.es.Indices.Create(idx.name, h.es.Indices.Create.WithBody(strings.NewReader(string(bodyBytes))))
		if err != nil {
			result["error"] = err.Error()
			results[idx.name] = result
			continue
		}
		createRes.Body.Close()

		if createRes.IsError() {
			result["error"] = fmt.Sprintf("ES error: %s", createRes.String())
		} else {
			result["created"] = true
			log.Printf("Created index: %s", idx.name)
		}
		results[idx.name] = result
	}

	allCreated := true
	anyErrors := false
	for _, v := range results {
		r := v.(map[string]any)
		if !r["created"].(bool) {
			allCreated = false
		}
		if r["error"] != nil {
			anyErrors = true
		}
	}

	msg := "All indices created successfully"
	if !allCreated {
		if anyErrors {
			msg = "Some indices failed to create"
		} else {
			msg = "Some indices already exist"
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": allCreated && !anyErrors,
		"message": msg,
		"results": results,
	})
}

// ClearDatabases drops and recreates PostgreSQL schema, deletes ES indices, resets Neo4j.
func (h *Handler) ClearDatabases(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	log.Println("Starting database clear operation")

	results := map[string]any{}

	// 1. Clear PostgreSQL.
	pgErr := func() error {
		_, err := h.pg.Exec(ctx, "DROP SCHEMA public CASCADE")
		if err != nil {
			return fmt.Errorf("drop schema: %w", err)
		}
		_, err = h.pg.Exec(ctx, "CREATE SCHEMA public")
		if err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
		return nil
	}()
	if pgErr != nil {
		results["postgres"] = map[string]any{"success": false, "error": pgErr.Error()}
	} else {
		results["postgres"] = map[string]any{"success": true}
	}

	// 2. Clear Elasticsearch indices.
	esErr := func() error {
		for _, idx := range []string{"anime", "staff", "studios", "franchises"} {
			res, err := h.es.Indices.Delete([]string{idx})
			if err != nil {
				return err
			}
			res.Body.Close()
		}
		return nil
	}()
	if esErr != nil {
		results["elasticsearch"] = map[string]any{"success": false, "error": esErr.Error()}
	} else {
		results["elasticsearch"] = map[string]any{"success": true}
	}

	// 3. Clear Neo4j (if available).
	if h.neo4j != nil {
		neo4jErr := func() error {
			session := h.neo4j.NewSession(ctx, neoSessionConfig())
			defer session.Close(ctx)
			_, err := session.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
			return err
		}()
		if neo4jErr != nil {
			results["neo4j"] = map[string]any{"success": false, "error": neo4jErr.Error()}
		} else {
			results["neo4j"] = map[string]any{"success": true}
		}
	}

	duration := time.Since(start)
	log.Printf("Database clear complete in %v", duration)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "Databases cleared",
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
		"results":  results,
	})
}

// neoSessionConfig returns default Neo4j session config.
func neoSessionConfig() neo4j.SessionConfig {
	return neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite}
}

// neoInt safely converts a Neo4j value to int64.
func neoInt(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}

// neo4jSessionConfig returns default Neo4j session config for reads.
func neo4jReadSessionConfig() neo4j.SessionConfig {
	return neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead}
}

// readBodyJSON decodes JSON request body into dest. Returns false if decoding fails
// (error already written to response).
func readBodyJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		httputil.Error(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return false
	}
	return true
}

func pgExecCtx(ctx context.Context, h *Handler, sql string, args ...any) error {
	_, err := h.pg.Exec(ctx, sql, args...)
	return err
}
