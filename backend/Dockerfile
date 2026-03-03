# Multi-stage build for the Go scraper gRPC server + Vue SPA.

# Stage 1: Build Vue SPA
FROM node:20-alpine AS vue-builder
WORKDIR /app
COPY vue-app/package*.json ./
RUN npm install
COPY vue-app/ .
RUN npm run build

# Stage 2: Build all Go binaries
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build

# Copy go.mod and go.sum first for layer caching.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy all source files.
COPY backend/ .

# Build the gRPC server.
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/scraper-server ./cmd/server/

# Build standalone binaries (used by gRPC service implementations).
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/scrape_incremental scrape_incremental.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/preprocess_csv preprocess_csv.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/compute_recommendations compute_recommendations.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/wikidata_qid wikidata_qid.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/wikidata_backfill_props wikidata_backfill_props.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/wikidata_studios wikidata_studios.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/wikipedia_production wikipedia_production.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/wikipedia_studio_content wikipedia_studio_content.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/staff_alternative_names staff_alternative_names.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/backfill_mal_ids backfill_mal_ids.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/sakugabooru_match sakugabooru_match.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/sakugabooru_posts sakugabooru_posts.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/fetch_studio_images fetch_studio_images.go

# Stage 3: Minimal runtime image
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app/backend

# Copy all binaries from builder.
COPY --from=builder /bin/scraper-server /app/backend/scraper-server
COPY --from=builder /bin/scrape_incremental /app/backend/scrape_incremental
COPY --from=builder /bin/preprocess_csv /app/backend/preprocess_csv
COPY --from=builder /bin/compute_recommendations /app/backend/compute_recommendations
COPY --from=builder /bin/wikidata_qid /app/backend/wikidata_qid
COPY --from=builder /bin/wikidata_backfill_props /app/backend/wikidata_backfill_props
COPY --from=builder /bin/wikidata_studios /app/backend/wikidata_studios
COPY --from=builder /bin/wikipedia_production /app/backend/wikipedia_production
COPY --from=builder /bin/wikipedia_studio_content /app/backend/wikipedia_studio_content
COPY --from=builder /bin/staff_alternative_names /app/backend/staff_alternative_names
COPY --from=builder /bin/backfill_mal_ids /app/backend/backfill_mal_ids
COPY --from=builder /bin/sakugabooru_match /app/backend/sakugabooru_match
COPY --from=builder /bin/sakugabooru_posts /app/backend/sakugabooru_posts
COPY --from=builder /bin/fetch_studio_images /app/backend/fetch_studio_images

# Copy Vue SPA dist.
COPY --from=vue-builder /app/dist /app/vue-dist

EXPOSE 50051

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s \
  CMD wget -qO- http://localhost:50051/healthz || exit 1

ENTRYPOINT ["/app/backend/scraper-server"]
