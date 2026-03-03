package admin

import (
	"net/http"
	"net/http/httptest"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	anigraphv1connect "anigraph/backend/gen/anigraph/v1/anigraphv1connect"
	"anigraph/backend/internal/enrichment"
	"anigraph/backend/internal/preprocessor"
	"anigraph/backend/internal/recommendations"
	"anigraph/backend/internal/sakugabooru"
	"anigraph/backend/internal/scraper"
	studioSvc "anigraph/backend/internal/studio"
)

// Handler holds dependencies for all admin endpoints.
type Handler struct {
	pg    *pgxpool.Pool
	es    *elasticsearch.Client
	neo4j neo4j.DriverWithContext

	// Connect clients for in-process service calls via httptest server.
	scraper         anigraphv1connect.ScraperServiceClient
	enrichment      anigraphv1connect.EnrichmentServiceClient
	sakugabooru     anigraphv1connect.SakugabooruServiceClient
	studio          anigraphv1connect.StudioServiceClient
	preprocessor    anigraphv1connect.PreprocessorServiceClient
	recommendations anigraphv1connect.RecommendationServiceClient

	// In-process server kept alive for connect client calls.
	internalServer *httptest.Server
}

// NewHandler creates an admin Handler with all dependencies.
// It creates an in-process httptest server wrapping the ConnectRPC services,
// then creates connect clients pointing at it. This lets admin endpoints call
// service methods using the standard client-side streaming API without going
// through the main server's HTTP listener.
func NewHandler(
	pg *pgxpool.Pool,
	es *elasticsearch.Client,
	n4j neo4j.DriverWithContext,
	sc *scraper.Service,
	en *enrichment.Service,
	sak *sakugabooru.Service,
	stu *studioSvc.Service,
	pre *preprocessor.Service,
	rec *recommendations.Service,
) *Handler {
	mux := http.NewServeMux()

	path, handler := anigraphv1connect.NewScraperServiceHandler(sc)
	mux.Handle(path, handler)
	path, handler = anigraphv1connect.NewEnrichmentServiceHandler(en)
	mux.Handle(path, handler)
	path, handler = anigraphv1connect.NewSakugabooruServiceHandler(sak)
	mux.Handle(path, handler)
	path, handler = anigraphv1connect.NewStudioServiceHandler(stu)
	mux.Handle(path, handler)
	path, handler = anigraphv1connect.NewPreprocessorServiceHandler(pre)
	mux.Handle(path, handler)
	path, handler = anigraphv1connect.NewRecommendationServiceHandler(rec)
	mux.Handle(path, handler)

	server := httptest.NewUnstartedServer(mux)
	server.Start()
	httpClient := server.Client()

	return &Handler{
		pg:              pg,
		es:              es,
		neo4j:           n4j,
		scraper:         anigraphv1connect.NewScraperServiceClient(httpClient, server.URL),
		enrichment:      anigraphv1connect.NewEnrichmentServiceClient(httpClient, server.URL),
		sakugabooru:     anigraphv1connect.NewSakugabooruServiceClient(httpClient, server.URL),
		studio:          anigraphv1connect.NewStudioServiceClient(httpClient, server.URL),
		preprocessor:    anigraphv1connect.NewPreprocessorServiceClient(httpClient, server.URL),
		recommendations: anigraphv1connect.NewRecommendationServiceClient(httpClient, server.URL),
		internalServer:  server,
	}
}
