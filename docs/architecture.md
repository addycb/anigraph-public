# Anigraph — Architecture

## System Context

```mermaid
C4Context
    title System Context — Anigraph

    Person(user, "User", "Browses anime recommendations")
    Person(admin, "Admin", "Manages data & scraping")

    System(app, "Anigraph", "Anime discovery platform")

    System_Ext(anilist, "AniList", "Anime metadata (GraphQL)")
    System_Ext(google, "Google OAuth", "Authentication")
    System_Ext(openai, "OpenAI", "Franchise naming")
    System_Ext(wikipedia, "Wikipedia / Wikidata", "Production info")
    System_Ext(sakugabooru, "Sakugabooru", "Animation clips")
    System_Ext(youtube, "YouTube", "Video metadata")

    Rel(user, app, "HTTPS")
    Rel(admin, app, "API key")
    Rel(app, anilist, "GraphQL")
    Rel(app, google, "OAuth 2.0")
    Rel(app, openai, "REST")
    Rel(app, wikipedia, "REST")
    Rel(app, sakugabooru, "REST")
    Rel(app, youtube, "REST")
```

## High-Level Architecture

```mermaid
flowchart TB
    subgraph Frontend["Frontend — Vue 3 SPA"]
        Pages["Pages (Vue Router)"]
        Components["Vuetify 3 + D3.js"]
        Composables["Composables"]
    end

    subgraph GoBackend["Backend — Go"]
        API["REST API (Chi router)"]
        AdminAPI["Admin API (pipelines)"]
        Auth["Auth (Google OAuth + anonymous UUID)"]
        GRPC["ConnectRPC Services"]
    end

    subgraph DataLayer["Data Layer"]
        PG["PostgreSQL 16"]
        Neo4j["Neo4j"]
        ES["Elasticsearch 8.11"]
    end

    subgraph ETL["ETL Pipeline (Go)"]
        Scraper["AniList Scraper"]
        Enrichment["Data Enrichment<br/>(Wikipedia, Wikidata,<br/>Sakugabooru, MAL)"]
        CSVExport["CSV Export"]
    end

    subgraph Infra["Infrastructure"]
        Nginx["Nginx + TLS (Certbot)"]
        Umami["Umami Analytics"]
    end

    Nginx -->|":443 → :8080"| GoBackend
    GoBackend -->|"serves SPA"| Frontend
    API -->|"pgx"| PG
    API -->|"Bolt"| Neo4j
    API -->|"REST"| ES
    AdminAPI -->|"bulk import"| PG
    AdminAPI -->|"indexing"| ES
    GRPC -->|"streaming RPCs"| ETL
    ETL -->|"CSV files"| AdminAPI
```

## Technology Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Frontend** | Vue 3, Vue Router, Vuetify 3 | SPA UI framework & component library |
| **Visualization** | D3.js | Force-directed graphs, timelines |
| **Backend** | Go, Chi router | REST API, SPA serving, admin pipelines |
| **RPC** | ConnectRPC (buf) | Streaming ETL orchestration (HTTP/1.1 + JSON compatible) |
| **Primary DB** | PostgreSQL 16 | Structured data, GIN indexes, JSONB cache, array columns |
| **Search** | Elasticsearch 8.11 | Full-text search with field boosting & fuzzy matching |
| **Graph DB** | Neo4j | Staff collaboration networks, graph traversal |
| **Auth** | Google OAuth 2.0 | User authentication (+ anonymous UUID fallback) |
| **AI** | OpenAI (GPT-4o-mini) | Franchise naming |
| **Reverse Proxy** | Nginx + Certbot | TLS termination, static assets |
| **Analytics** | Umami | Privacy-friendly analytics |
| **Containerization** | Docker Compose | Full-stack orchestration |

## ETL Pipeline

```mermaid
flowchart LR
    subgraph Sources["Data Sources"]
        AL["AniList API"]
        WK["Wikipedia / Wikidata"]
        SK["Sakugabooru"]
        MAL["MyAnimeList"]
        YT["YouTube API"]
    end

    subgraph Pipeline["Go ETL Pipeline"]
        Full["Full Scrape<br/>~500 pages / ~21k anime"]
        Incr["Incremental<br/>newest 1000"]
        Enrich["Enrichment<br/>(MAL IDs, Wikipedia,<br/>Wikidata, Sakugabooru)"]
    end

    subgraph Output["CSV Output"]
        AnimeCSV["anime.csv / anime_delta.csv"]
        StaffCSV["staff.csv / staff_delta.csv"]
        EdgesCSV["media_staff_edges.csv<br/>media_relations.csv"]
    end

    subgraph Import["Bulk Import"]
        PGImport["PostgreSQL (COPY)"]
        Neo4jImport["Neo4j (APOC)"]
        ESIndex["Elasticsearch indexing"]
    end

    subgraph Precompute["Precomputation"]
        Recs["Recommendations<br/>(tag cosine similarity)"]
        OPSim["OP Similarity<br/>(video embedding cosine sim)"]
        Graphs["Graph Cache<br/>(D3 nodes + links)"]
        Rankings["Global Rankings"]
        Filters["Filter Counts"]
    end

    AL --> Full & Incr
    WK & SK & MAL & YT --> Enrich
    Full & Incr & Enrich --> Output
    Output --> PGImport & Neo4jImport
    PGImport --> ESIndex
    PGImport --> Precompute

```

## ConnectRPC Services

6 services defined in `proto/anigraph/v1/`:

| Service | RPCs | Streaming |
|---------|------|-----------|
| **ScraperService** | ScrapeIncremental | Server-streaming |
| **PreprocessorService** | PreprocessData | Server-streaming |
| **RecommendationService** | ComputeRecommendations | Server-streaming |
| **EnrichmentService** | 7 RPCs (BackfillMalIds, etc.) | Bidi-stream + unary |
| **SakugabooruService** | MatchTags, FetchPosts | Server-streaming |
| **StudioService** | FetchStudioImages | Server-streaming |

## Polyglot Persistence

```mermaid
flowchart LR
    subgraph Queries["Query Types"]
        CRUD["Structured Lookups<br/>Anime, Staff, Studio,<br/>User data"]
        Search["Full-Text Search<br/>Fuzzy matching,<br/>field boosting"]
        Graph["Graph Traversal<br/>Staff collaborations,<br/>multi-hop relations"]
    end

    subgraph Stores["Data Stores"]
        PG["PostgreSQL<br/>───────────<br/>GIN indexes<br/>Trigram search<br/>JSONB cache<br/>Array columns"]
        ES["Elasticsearch<br/>───────────<br/>Inverted index<br/>Field boosting<br/>Highlighting"]
        Neo["Neo4j<br/>───────────<br/>Cypher queries<br/>APOC procedures<br/>Pattern matching"]
    end

    CRUD --> PG
    Search --> ES
    Graph --> Neo

```

## Database Schema (Core Entities)

```mermaid
erDiagram
    Anime ||--o{ Anime : "relation (sequel, prequel, etc.)"
    Anime ||--o{ Anime : "recommendation (tag similarity)"
    Anime ||--o{ Anime : "OP similarity (video embeddings)"
    Anime }o--o{ Genre : "anime_genre"
    Anime }o--o{ Tag : "anime_tag (rank)"
    Anime }o--o{ Studio : "anime_studio (is_main)"
    Anime }o--o{ Staff : "anime_staff (role)"
    Anime }o--|| Franchise : "belongs to"
    User ||--o{ UserList : "owns"
    UserList ||--o{ Anime : "list items"
    User ||--o{ Anime : "favorites"
    User ||--o| UserTasteProfile : "has"
    User ||--o{ Anime : "predictions"
```

## Deployment

```mermaid
flowchart TB
    subgraph Host["Docker Compose"]
        subgraph Proxy["Reverse Proxy"]
            Nginx["Nginx Alpine<br/>:80 / :443"]
        end

        subgraph App["Application"]
            Go["Go Backend<br/>:8080<br/>(REST + ConnectRPC + SPA)"]
        end

        subgraph Data["Data Stores"]
            PG["PostgreSQL 16<br/>:5432"]
            Neo4j["Neo4j<br/>:7687 / :7474"]
            ES["Elasticsearch 8.11<br/>:9200"]
        end

        subgraph Analytics["Analytics"]
            Umami["Umami<br/>:3001"]
        end
    end

    Internet["Internet"] -->|":443"| Nginx
    Nginx -->|":8080"| Go
    Go -->|":5432"| PG
    Go -->|":7687"| Neo4j
    Go -->|":9200"| ES

```
