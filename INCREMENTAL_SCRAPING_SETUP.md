# Incremental Scraping Setup Guide

This guide explains how to set up and use the automated incremental scraping system for AniGraph.

## Overview

The incremental scraping system consists of:

1. **Go Scraper** (`go scraper/scrape.go`) - Fetches data from AniList API
2. **Delta CSV Ingestion** (`nuxt-app/server/api/admin/load-csv-delta.post.ts`) - Loads delta CSVs into Neo4j
3. **Workflow Trigger** (`nuxt-app/server/api/admin/trigger-incremental-scrape.post.ts`) - Orchestrates the full process
4. **Scheduler** (`nuxt-app/server/plugins/scheduler.ts`) - Runs incremental scrape every 24 hours

## How It Works

### Full Scrape (Initial Setup)
```
AniList API → scraper → anime.csv, staff.csv, edges.csv
             ↓
           Neo4j (MERGE) → Elasticsearch
             ↓
       scraper_state.json (saved)
```

### Incremental Scrape (Daily Updates)
```
Load scraper_state.json
             ↓
AniList API (newest 1000 anime, ID_DESC) → scraper
             ↓
anime_delta.csv, staff_delta.csv, edges_delta.csv
             ↓
           Neo4j (MERGE updates existing, creates new)
             ↓
      Elasticsearch (sync updated records)
             ↓
  scraper_state.json (updated)
```

## Initial Setup

### 1. Build the Go Scraper

```bash
cd "go scraper"

# For current platform
./build.sh

# For Linux server (if building on different OS)
./build.sh linux

# This creates: scraper (or scraper_linux_amd64)
```

### 2. Configure Proxy Settings

Edit `go scraper/scrape.go`:

```go
const (
    PROXY_PORT_START    = 10001
    PROXY_PORT_END      = 10100
    MIN_SECONDS_PER_PROXY = 2.0  // 30 req/min

    EXAMPLE_PROXY = "your-proxy-host:port:username:password"
)
```

### 3. Run Initial Full Scrape

```bash
cd "go scraper"
./scraper
```

Wait for completion (~10-15 minutes with 100 proxies). This creates:
- `anime.csv` (all anime)
- `staff.csv` (all staff)
- `anime_staff_edges.csv` (relationships)
- `scraper_state.json` (checkpoint)

### 4. Ingest Initial Data into Neo4j

```bash
# Move CSVs to Neo4j import directory
# Then call the ingestion endpoint
curl -X POST http://localhost:3000/api/admin/load-csv-data
```

### 5. Remove ADR Roles (Optional)

```bash
curl -X POST http://localhost:3000/api/admin/remove-adr-roles
```

### 6. Sync to Elasticsearch

```bash
curl -X POST http://localhost:3000/api/admin/sync-es-from-neo4j?recreate=true
```

## Configure Automatic Scheduling

### Environment Variables

Add to your `.env` file:

```bash
# Scheduler Configuration
ENABLE_SCHEDULER=true              # Enable automatic scraping
SCRAPE_INTERVAL_HOURS=24           # Run every 24 hours
RUN_SCRAPE_ON_STARTUP=false        # Don't run on server start (optional)
```

### Restart Nuxt Server

```bash
cd nuxt-app
npm run dev  # or npm run build && npm start for production
```

The scheduler will:
- Start automatically when the server starts
- Run incremental scrape every 24 hours
- Log next scheduled run time
- Run the full workflow automatically

## Manual Incremental Scrape

You can manually trigger an incremental scrape anytime:

```bash
# Option 1: Run scraper directly (just data fetch)
cd "go scraper"
./scraper -incremental

# Option 2: Full workflow (scraper + Neo4j + Elasticsearch)
curl -X POST http://localhost:3000/api/admin/trigger-incremental-scrape
```

## Workflow Steps

### Automated Workflow (via API)

When you call `/api/admin/trigger-incremental-scrape`:

1. **Run Scraper**: Executes `./scraper -incremental`
   - Loads `scraper_state.json`
   - Fetches newest ~1000 anime (20 pages, sorted by ID DESC)
   - Generates delta CSVs
   - Updates state file

2. **Ingest to Neo4j**: Calls `/api/admin/load-csv-delta`
   - Loads `anime_delta.csv`, `staff_delta.csv`, `edges_delta.csv`
   - Uses MERGE to update/create records
   - Updates staff counts for affected anime

3. **Sync to Elasticsearch**: Calls `/api/admin/sync-es-from-neo4j?recreate=false`
   - Syncs updated records to Elasticsearch
   - Does NOT recreate indices (incremental update)

## Monitoring

### Check Scheduler Status

Look for logs when server starts:
```
📅 Scheduler enabled - will run incremental scrape every 24 hours
📅 Next scheduled scrape: 2025-12-22T12:00:00.000Z
```

### Check Scraper Logs

```bash
cd "go scraper"
tail -f failed_pages.txt  # See any failures
```

### Check State File

```bash
cd "go scraper"
cat scraper_state.json
```

Example output:
```json
{
  "last_scrape_timestamp": "2025-12-21T18:30:00Z",
  "max_anime_id": 205000,
  "max_staff_id": 383500,
  "total_anime_scraped": 22100,
  "total_staff_scraped": 89000,
  "scrape_mode": "incremental",
  "csv_files": {
    "anime": "anime_delta.csv",
    "staff": "staff_delta.csv",
    "edges": "anime_staff_edges_delta.csv"
  }
}
```

## API Endpoints

### Trigger Incremental Scrape
```bash
POST /api/admin/trigger-incremental-scrape
```

Response:
```json
{
  "success": true,
  "message": "Incremental scrape workflow completed",
  "duration": "45000ms",
  "steps": {
    "scraper": { "success": true, "duration": 35000 },
    "neo4j": { "success": true, "duration": 8000 },
    "elasticsearch": { "success": true, "duration": 2000 }
  }
}
```

### Load Delta CSVs
```bash
POST /api/admin/load-csv-delta
```

### Full Scrape Workflow (Initial)
```bash
POST /api/admin/load-csv-data         # Full CSV load
POST /api/admin/remove-adr-roles      # Clean ADR roles
POST /api/admin/sync-es-from-neo4j?recreate=true  # Create ES indices
```

## File Locations

```
anigraph/
├── go scraper/
│   ├── scrape.go                    # Scraper source
│   ├── scraper                      # Compiled binary
│   ├── build.sh                     # Build script
│   ├── scraper_state.json          # State checkpoint
│   ├── anime.csv                    # Full scrape output
│   ├── staff.csv
│   ├── anime_staff_edges.csv
│   ├── anime_delta.csv             # Incremental output
│   ├── staff_delta.csv
│   ├── anime_staff_edges_delta.csv
│   └── failed_pages.txt            # Error log
│
└── nuxt-app/
    └── server/
        ├── api/admin/
        │   ├── load-csv-data.post.ts           # Full CSV load
        │   ├── load-csv-delta.post.ts          # Delta CSV load
        │   ├── trigger-incremental-scrape.post.ts  # Workflow
        │   ├── remove-adr-roles.post.ts
        │   └── sync-es-from-neo4j.post.ts
        └── plugins/
            └── scheduler.ts                     # 24h scheduler
```

## Troubleshooting

### Scraper can't find state file
- Normal on first run - will fall back to full scrape
- Ensure `scraper_state.json` exists in `go scraper/` directory

### Delta CSV files not found
- Expected if no new data in that category
- Delta ingestion gracefully skips missing files

### Scheduler not running
- Check `ENABLE_SCHEDULER=true` in `.env`
- Check server logs for scheduler initialization
- Restart Nuxt server

### Rate limit errors
- Adjust `MIN_SECONDS_PER_PROXY` in `scrape.go`
- Reduce `PROXY_PORT_END` to use fewer proxies
- Verify proxy credentials

### Scraper binary permission denied
```bash
chmod +x "go scraper/scraper"
```

### CSV import directory issues
- Neo4j must have access to CSV files
- Check Neo4j import directory setting
- Move/copy CSVs to Neo4j import directory if needed

## Production Deployment

### Build for Server
```bash
cd "go scraper"
./build.sh linux
# Upload scraper_linux_amd64 to server
```

### Server Setup
1. Copy binary to server: `scp scraper_linux_amd64 user@server:/path/to/go\ scraper/scraper`
2. Copy state file: `scp scraper_state.json user@server:/path/to/go\ scraper/`
3. Set up `.env` with `ENABLE_SCHEDULER=true`
4. Start Nuxt app: `npm start`

### Monitoring in Production
- Set up log aggregation (e.g., PM2 logs)
- Monitor `failed_pages.txt` for errors
- Set up alerts for workflow failures
- Check disk space for CSV files

## Performance Tuning

### Scraper
- **More proxies** = faster scraping (up to 100 proxies = ~50 req/s)
- **Fewer pages** in incremental mode = faster updates (default: 20 pages = 1000 anime)
- **Retry attempts** = reliability vs speed (default: 4 attempts)

### Database
- **MERGE** operations are efficient for incremental updates
- **Indices** on `anilistId` and `staff_id` speed up lookups
- **Staff count updates** only run for affected anime

### Elasticsearch
- **Incremental sync** (`recreate=false`) is faster than full reindex
- **Batch size** controls memory vs speed (default: 1000)

## Best Practices

1. **Always run full scrape first** before enabling incremental mode
2. **Keep scraper_state.json** - it's critical for incremental scraping
3. **Monitor failed_pages.txt** for persistent errors
4. **Test workflow manually** before enabling scheduler
5. **Back up state file** before major changes
6. **Use incremental mode** for daily updates, not full scrape
7. **Schedule during low-traffic hours** if possible

## Future Improvements

- [ ] Add webhook notifications for scrape completion/failure
- [ ] Add dashboard for monitoring scrape status
- [ ] Implement differential sync for Elasticsearch (only updated records)
- [ ] Add scraper health check endpoint
- [ ] Implement automatic retry for failed scrapes
- [ ] Add metrics/analytics for scrape performance
