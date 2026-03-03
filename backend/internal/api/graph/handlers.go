package graph

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"anigraph/backend/internal/api/httputil"
)

type Handler struct {
	pg *pgxpool.Pool
}

func NewHandler(pg *pgxpool.Pool) *Handler {
	return &Handler{pg: pg}
}

// GetByAnimeID handles GET /api/graph/{animeId} — returns precomputed graph data.
func (h *Handler) GetByAnimeID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "animeId")
	anilistID, err := strconv.Atoi(idStr)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "Invalid anime ID")
		return
	}

	var graphData any
	err = h.pg.QueryRow(r.Context(),
		"SELECT graph_data FROM anime_graph_cache WHERE anilist_id = $1", anilistID).Scan(&graphData)
	if err == pgx.ErrNoRows {
		httputil.Error(w, http.StatusNotFound, "Graph not found for this anime. It may not have been computed yet.")
		return
	}
	if err != nil {
		log.Printf("graph cache error: %v", err)
		httputil.Error(w, http.StatusInternalServerError, "Failed to fetch graph data")
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    graphData,
		"cached":  true,
	})
}
