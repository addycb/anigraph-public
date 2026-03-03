package health

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type Handler struct {
	pg    *pgxpool.Pool
	es    *elasticsearch.Client
	neo4j neo4j.DriverWithContext
}

func NewHandler(pg *pgxpool.Pool, es *elasticsearch.Client, n4j neo4j.DriverWithContext) *Handler {
	return &Handler{pg: pg, es: es, neo4j: n4j}
}

// Health returns a simple ok status.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Status checks all database connections and returns their health.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	result := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"server":    "go",
		"goVersion": runtime.Version(),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Postgres
	pgStatus := map[string]any{"connected": false}
	if err := h.pg.Ping(ctx); err != nil {
		pgStatus["error"] = err.Error()
	} else {
		pgStatus["connected"] = true
		stats := h.pg.Stat()
		pgStatus["total_conns"] = stats.TotalConns()
		pgStatus["idle_conns"] = stats.IdleConns()
	}
	result["postgres"] = pgStatus

	// Elasticsearch
	esStatus := map[string]any{"connected": false}
	res, err := h.es.Cluster.Health()
	if err != nil {
		esStatus["error"] = err.Error()
	} else {
		defer res.Body.Close()
		if res.IsError() {
			esStatus["error"] = res.Status()
		} else {
			var body map[string]any
			json.NewDecoder(res.Body).Decode(&body)
			esStatus["connected"] = true
			esStatus["cluster_status"] = body["status"]
		}
	}
	result["elasticsearch"] = esStatus

	// Neo4j
	neo4jStatus := map[string]any{"connected": false}
	if h.neo4j != nil {
		if err := h.neo4j.VerifyConnectivity(ctx); err != nil {
			neo4jStatus["error"] = err.Error()
		} else {
			neo4jStatus["connected"] = true
		}
	} else {
		neo4jStatus["error"] = "not configured"
	}
	result["neo4j"] = neo4jStatus

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
