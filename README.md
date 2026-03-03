# Anigraph

**Live at [anigraph.xyz](https://anigraph.xyz)**

An anime discovery and recommendation platform with graph-based relationship exploration. Browse ~150,000 anime, explore staff collaboration networks through interactive force-directed graphs, get personalized recommendations, and build shareable watchlists.

## Architecture Overview

> View this on GitHub for rendered Mermaid diagrams.

See [docs/architecture.md](docs/architecture.md) for full architecture diagrams including system context, data flow, ETL pipeline, database schema, and deployment topology.

### Key Design Decisions

- **Three databases, each for its strength** — PostgreSQL for structured data and JSONB caching, Elasticsearch for full-text search with fuzzy matching and field boosting, Neo4j for graph traversal powering the staff collaboration visualizations
- **ConnectRPC streaming for ETL** — Long-running pipeline steps (scraping, enrichment, recommendation computation) stream real-time progress back to the admin UI over HTTP/1.1, no WebSockets needed
- **Pre-computed graph cache** — D3 graph layouts are expensive, so node+link JSON is computed during ETL and served as static JSONB from PostgreSQL
- **Denormalized arrays** — `genre_names`, `tag_names`, `studio_names` on the anime table eliminate JOINs on the hot read path; kept in sync by the ETL pipeline
- **Anonymous UUID fallback** — Users who skip Google OAuth still get a session, so favorites and lists work without sign-in

## Setup

### Prerequisites

- Docker and Docker Compose v2+

### Environment Variables

Create a `.env` file:

```env
# Database
POSTGRES_PASSWORD=<your-password>
POSTGRES_USER=anigraph
POSTGRES_DB=anigraph

# Auth
GOOGLE_CLIENT_ID=<your-google-client-id>
GOOGLE_CLIENT_SECRET=<your-google-client-secret>
SESSION_SECRET=<random-string>

# Graph DB
NEO4J_URI=bolt://neo4j:7687
NEO4J_USER=neo4j
NEO4J_PASSWORD=<your-password>

# Search
ELASTICSEARCH_NODE=http://elasticsearch:9200

# Optional
OPENAI_API_KEY=<your-key>       # Franchise name generation
ADMIN_API_KEY=<your-key>        # Admin pipeline endpoints
```

### Development

```bash
docker compose -f docker-compose-dev.yml up
```

| Service | Port |
|---------|------|
| Nginx | 80 |
| Backend (Go) | 50051 |
| Vue App (Vite HMR) | 5173 |
| PostgreSQL | 5432 |
| Neo4j | 7474 / 7687 |
| Elasticsearch | 9200 |
| Umami Analytics | 3001 |

### Production

```bash
docker compose -f docker-compose-prod.yml up -d
```

Production uses Nginx TLS termination via Certbot, builds the Vue SPA into the Go Docker image (multi-stage), enforces memory limits, and only starts Neo4j during pipeline runs.

### Restoring a Database Backup

Start PostgreSQL first, then run the restore:

```bash
docker compose -f docker-compose-dev.yml up -d postgres
./restore-db.sh                # Restores backup.dump
./restore-db.sh other.dump     # Restores a different file
```

## License

MIT
