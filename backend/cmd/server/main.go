package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"anigraph/backend/gen/anigraph/v1/anigraphv1connect"
	"anigraph/backend/internal/api"
	"anigraph/backend/internal/db"
	"anigraph/backend/internal/enrichment"
	"anigraph/backend/internal/preprocessor"
	"anigraph/backend/internal/recommendations"
	"anigraph/backend/internal/sakugabooru"
	"anigraph/backend/internal/scraper"
	studioSvc "anigraph/backend/internal/studio"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Initialize database clients.
	pg, err := db.NewPostgresPool(ctx)
	if err != nil {
		log.Printf("WARNING: postgres unavailable: %v", err)
	} else {
		defer pg.Close()
	}

	es, err := db.NewElasticsearchClient()
	if err != nil {
		log.Printf("WARNING: elasticsearch unavailable: %v", err)
	}

	n4j, err := db.NewNeo4jDriver(ctx)
	if err != nil {
		log.Printf("WARNING: neo4j unavailable: %v", err)
	} else if n4j != nil {
		defer n4j.Close(context.Background())
	}

	// Create shared service instances (used by both ConnectRPC and admin REST).
	scraperSvc := scraper.NewService()
	preprocessorSvc := preprocessor.NewService()
	recsSvc := recommendations.NewService()
	enrichmentSvc := enrichment.NewService()
	sakugabooruSvc := sakugabooru.NewService()
	studioService := studioSvc.NewService()

	// Build the REST API router (chi), including admin endpoints.
	apiRouter, adminHandler := api.NewRouter(pg, es, n4j, scraperSvc, enrichmentSvc, sakugabooruSvc, studioService, preprocessorSvc, recsSvc)

	// Restore any scheduled tasks from before restart.
	if adminHandler != nil {
		adminHandler.RestoreSchedules()
	}

	// Build the ConnectRPC mux.
	connectMux := http.NewServeMux()
	interceptors := connect.WithInterceptors(loggingInterceptor())

	scraperPath, scraperHandler := anigraphv1connect.NewScraperServiceHandler(scraperSvc, interceptors)
	connectMux.Handle(scraperPath, scraperHandler)

	preprocessorPath, preprocessorHandler := anigraphv1connect.NewPreprocessorServiceHandler(preprocessorSvc, interceptors)
	connectMux.Handle(preprocessorPath, preprocessorHandler)

	recsPath, recsHandler := anigraphv1connect.NewRecommendationServiceHandler(recsSvc, interceptors)
	connectMux.Handle(recsPath, recsHandler)

	enrichmentPath, enrichmentHandler := anigraphv1connect.NewEnrichmentServiceHandler(enrichmentSvc, interceptors)
	connectMux.Handle(enrichmentPath, enrichmentHandler)

	sakugabooruPath, sakugabooruHandler := anigraphv1connect.NewSakugabooruServiceHandler(sakugabooruSvc, interceptors)
	connectMux.Handle(sakugabooruPath, sakugabooruHandler)

	studioPath, studioHandler := anigraphv1connect.NewStudioServiceHandler(studioService, interceptors)
	connectMux.Handle(studioPath, studioHandler)

	connectMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// SPA file server (serves Vue app static files with index.html fallback).
	spaDir := os.Getenv("SPA_DIR")
	if spaDir == "" {
		spaDir = "/app/vue-dist"
	}
	var spaFS http.Handler
	if _, err := os.Stat(spaDir); err == nil {
		spaFS = spaHandler(http.Dir(spaDir))
		log.Printf("serving SPA from %s", spaDir)
	}

	// Combined handler: /api/* → chi, /anigraph.v1.* + /healthz → ConnectRPC, /* → SPA.
	combined := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiRouter.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/anigraph.v1.") || r.URL.Path == "/healthz" {
			connectMux.ServeHTTP(w, r)
			return
		}
		if spaFS != nil {
			spaFS.ServeHTTP(w, r)
			return
		}
		connectMux.ServeHTTP(w, r)
	})

	addr := ":" + port
	server := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(combined, &http2.Server{}),
	}

	go func() {
		log.Printf("server listening on %s (ConnectRPC + REST API)", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}

// spaHandler serves static files and falls back to index.html for client-side routing.
func spaHandler(fs http.FileSystem) http.Handler {
	fileServer := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path

		// Cache headers for Vite hashed assets.
		if strings.HasPrefix(p, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		// Try to serve the file directly.
		f, err := fs.Open(p)
		if err != nil {
			// File doesn't exist → serve index.html (SPA fallback).
			w.Header().Set("Cache-Control", "no-cache")
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		// Don't serve directory listings — fall back to index.html.
		stat, _ := fs.Open(p)
		if stat != nil {
			info, _ := stat.Stat()
			stat.Close()
			if info != nil && info.IsDir() {
				// Check for index.html in the directory.
				idx, err := fs.Open(path.Join(p, "index.html"))
				if err != nil {
					w.Header().Set("Cache-Control", "no-cache")
					r.URL.Path = "/"
					fileServer.ServeHTTP(w, r)
					return
				}
				idx.Close()
			}
		}

		// index.html should never be cached.
		if p == "/" || p == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache")
		}

		fileServer.ServeHTTP(w, r)
	})
}

// loggingInterceptor logs RPC calls.
func loggingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			log.Printf("%s %s %v", req.Spec().Procedure, time.Since(start), err)
			return resp, err
		}
	}
}
