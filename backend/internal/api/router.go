package api

import (
	"net/http"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"anigraph/backend/internal/api/admin"
	"anigraph/backend/internal/api/anime"
	"anigraph/backend/internal/api/auth"
	"anigraph/backend/internal/api/franchise"
	"anigraph/backend/internal/api/graph"
	"anigraph/backend/internal/api/health"
	"anigraph/backend/internal/api/lists"
	"anigraph/backend/internal/api/middleware"
	"anigraph/backend/internal/api/search"
	"anigraph/backend/internal/api/staff"
	"anigraph/backend/internal/api/studio"
	"anigraph/backend/internal/api/user"
	"anigraph/backend/internal/enrichment"
	"anigraph/backend/internal/preprocessor"
	"anigraph/backend/internal/recommendations"
	"anigraph/backend/internal/sakugabooru"
	"anigraph/backend/internal/scraper"
	studioSvc "anigraph/backend/internal/studio"
)

// NewRouter creates the chi router with all API routes mounted.
func NewRouter(
	pg *pgxpool.Pool,
	es *elasticsearch.Client,
	n4j neo4j.DriverWithContext,
	sc *scraper.Service,
	en *enrichment.Service,
	sak *sakugabooru.Service,
	stu *studioSvc.Service,
	pre *preprocessor.Service,
	rec *recommendations.Service,
) (http.Handler, *admin.Handler) {
	r := chi.NewRouter()

	// Global middleware.
	r.Use(middleware.Security)
	r.Use(middleware.Timing)

	// Health endpoints.
	h := health.NewHandler(pg, es, n4j)
	r.Get("/api/health", h.Health)
	r.Get("/api/status", h.Status)

	// Search endpoints (Elasticsearch).
	s := search.NewHandler(es)
	r.Get("/api/search", s.Search)
	r.Get("/api/search/unified", s.Unified)

	// Anime endpoints (PostgreSQL + Elasticsearch).
	a := anime.NewHandler(pg)
	r.Get("/api/anime/search", s.AnimeSearch)
	r.Get("/api/anime/popular", a.Popular)
	r.Get("/api/anime/advanced-search", a.AdvancedSearch)
	r.Get("/api/anime/bulk", a.Bulk)
	r.Get("/api/anime/filter-counts", a.FilterCounts)
	r.Get("/api/anime/filter-metadata", a.FilterMetadata)
	r.Get("/api/anime/genres-tags", a.GenresTags)
	r.Get("/api/anime/{id}", a.GetByID)
	r.Get("/api/anime/{id}/recommendations", a.Recommendations)
	r.Get("/api/anime/{id}/relations", a.Relations)

	// Staff endpoints.
	st := staff.NewHandler(pg, es)
	r.Get("/api/staff/search", st.Search)
	r.Get("/api/staff/{id}", st.GetByID)

	// Studio endpoints.
	studioH := studio.NewHandler(pg, es)
	r.Get("/api/studio/search", studioH.Search)
	r.Get("/api/studio/{name}", studioH.GetByName)

	// Franchise endpoints.
	f := franchise.NewHandler(pg, es)
	r.Get("/api/franchise/search", f.Search)
	r.Get("/api/franchise/{id}", f.GetByID)

	// Graph endpoint.
	g := graph.NewHandler(pg)
	r.Get("/api/graph/{animeId}", g.GetByAnimeID)

	// Auth endpoints (Google OAuth, session, CSRF).
	au := auth.NewHandler(pg)
	r.Get("/api/auth/google/login", au.GoogleLogin)
	r.Get("/api/auth/google/callback", au.GoogleCallback)
	r.Get("/api/auth/me", au.Me)
	r.Get("/api/auth/csrf-token", au.CSRFToken)
	// Logout requires CSRF validation (state-changing).
	r.With(middleware.RequireCSRF).Post("/api/auth/logout", au.Logout)

	// User endpoints (auth required).
	u := user.NewHandler(pg)
	authRequired := middleware.Auth(pg, true)
	authOptional := middleware.Auth(pg, false)
	csrfAuth := func(next http.Handler) http.Handler {
		return authRequired(middleware.RequireCSRF(next))
	}

	// Preferences.
	r.With(authRequired).Get("/api/user/preferences", u.GetPreferences)
	r.With(csrfAuth).Patch("/api/user/preferences", u.PatchPreferences)

	// Favorites.
	r.With(authRequired).Get("/api/user/favorites", u.GetFavorites)
	r.With(csrfAuth).Post("/api/user/favorite-anime", u.FavoriteAnime)

	// Taste profile.
	r.With(authRequired).Get("/api/user/taste-profile", u.GetTasteProfile)
	r.With(csrfAuth).Post("/api/user/compute-taste-profile", u.ComputeTasteProfile)

	// Recommendations.
	r.With(authRequired).Get("/api/user/recommendations", u.GetRecommendations)
	r.With(csrfAuth).Post("/api/user/compute-recommendations", u.ComputeRecommendations)

	// Lists (CRUD).
	l := lists.NewHandler(pg)
	r.With(authRequired).Get("/api/user/lists", l.GetLists)
	r.With(csrfAuth).Post("/api/user/lists", l.CreateList)
	r.With(csrfAuth).Patch("/api/user/lists/{id}", l.UpdateList)
	r.With(csrfAuth).Delete("/api/user/lists/{id}", l.DeleteList)

	// List items.
	r.With(authOptional).Get("/api/user/lists/{id}/items", l.GetListItems)
	r.With(csrfAuth).Post("/api/user/lists/{id}/items", l.AddListItem)
	r.With(csrfAuth).Delete("/api/user/lists/{id}/items", l.RemoveListItem)

	// Public lists (no auth required).
	r.Get("/api/lists/public", l.PublicLists)
	r.Get("/api/lists/share/{token}", l.ShareToken)

	// Admin endpoints (API key auth).
	adm := admin.NewHandler(pg, es, n4j, sc, en, sak, stu, pre, rec)
	r.Route("/api/admin", func(r chi.Router) {
		r.Use(admin.RequireAdmin)

		// Database stats & management.
		r.Get("/db-stats", adm.DBStats)
		r.Post("/setup-gin-indexes", adm.SetupGINIndexes)
		r.Post("/create-indices", adm.CreateIndices)
		r.Post("/clear-databases", adm.ClearDatabases)

		// Precomputation.
		r.Post("/precompute-bitmaps", adm.PrecomputeBitmaps)
		r.Post("/precompute-counts", adm.PrecomputeCounts)
		r.Post("/precompute-global-rankings", adm.PrecomputeGlobalRankings)
		r.Post("/populate-mal-ids", adm.PopulateMalIDs)

		// Enrichment (delegates to ConnectRPC services).
		r.Post("/enrich-wikidata", adm.EnrichWikidata)
		r.Post("/backfill-wikidata-props", adm.BackfillWikidataProps)
		r.Post("/wikipedia-production", adm.WikipediaProduction)
		r.Post("/wikipedia-studio-content", adm.WikipediaStudioContent)
		r.Post("/enrich-staff-alternative-names", adm.EnrichStaffAlternativeNames)
		r.Post("/sakugabooru-enrich", adm.SakugabooruEnrich)
		r.Post("/fetch-studio-images", adm.FetchStudioImages)
		r.Post("/enrich-wikidata-studios", adm.EnrichWikidataStudios)
		r.Post("/backfill-mal-ids", adm.BackfillMalIDs)
		r.Post("/backfill-wikipedia", adm.BackfillWikipedia)
		r.Get("/test-scraper-path", adm.TestScraperPath)

		// Elasticsearch sync.
		r.Post("/sync-elasticsearch", adm.SyncElasticsearch)

		// CSV operations.
		r.Post("/import-csv-to-postgres", adm.ImportCSVToPostgres)
		r.Post("/ingest-es", adm.IngestES)
		r.Post("/preprocess-csv", adm.PreprocessCSV)
		r.Post("/export-neo4j-to-csv", adm.ExportNeo4jToCSV)
		r.Post("/export-backup", adm.ExportBackup)

		// Neo4j + graph operations.
		r.Post("/recompute-graphs", adm.RecomputeGraphs)
		r.Post("/compute-missing-graphs", adm.ComputeMissingGraphs)
		r.Post("/remove-adr-roles", adm.RemoveADRRoles)
		r.Post("/update-dates", adm.UpdateDates)
		r.Post("/update-season-years", adm.UpdateSeasonYears)
		r.Post("/load-csv-data", adm.LoadCSVData)
		r.Post("/load-csv-delta", adm.LoadCSVDelta)
		r.Post("/migrate-to-postgres", adm.MigrateToPostgres)
		r.Post("/extract-colors", adm.ExtractColors)

		// Franchise operations.
		r.Post("/generate-franchises", adm.GenerateFranchises)
		r.Post("/rename-franchises", adm.RenameFranchises)
		r.Post("/generate-recommendations", adm.GenerateRecommendations)

		// Pipeline & scheduler.
		r.Post("/incremental-update", adm.IncrementalUpdate)
		r.Post("/schedule-incremental-update", adm.ScheduleIncrementalUpdate)
		r.Get("/incremental-update-logs", adm.IncrementalUpdateLogs)
	})

	return r, adm
}
