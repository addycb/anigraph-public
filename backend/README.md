# AniList Scraper

Go-based scraper for fetching anime and staff data from the AniList GraphQL API.

## Features

- **Full Scrape Mode**: Fetches all anime, staff, and relationships from scratch
- **Incremental Mode**: Fetches only new/updated anime since last scrape (sorted by ID DESC)
- **Proxy Support**: Uses rotating proxies with automatic rate limiting
- **State Tracking**: Saves scraper state for incremental updates
- **CSV Output**: Generates CSV files for Neo4j ingestion
- **Retry Logic**: Automatic retry with exponential backoff
- **Stats Tracking**: Real-time request statistics

## Building the Scraper

### Build for your current platform:
```bash
cd "go scraper"
go build -o scraper scrape.go
```

### Cross-compile for Linux server (from any OS):
```bash
# For Linux x64
GOOS=linux GOARCH=amd64 go build -o scraper scrape.go

# For Linux ARM64 (e.g., some cloud instances)
GOOS=linux GOARCH=arm64 go build -o scraper scrape.go
```

## Usage

### Full Scrape (first time):
```bash
./scraper
```

This will:
- Fetch ~500 pages of anime (all anime in AniList)
- Generate `anime.csv`, `staff.csv`, `anime_staff_edges.csv`
- Save state to `scraper_state.json`

### Incremental Scrape (updates only):
```bash
./scraper -incremental
```

This will:
- Load previous state from `scraper_state.json`
- Fetch only ~20 pages (newest 1000 anime, sorted by ID DESC)
- Generate `anime_delta.csv`, `staff_delta.csv`, `anime_staff_edges_delta.csv`
- Database MERGE handles deduplication

### Flags

- `-incremental`: Enable incremental mode (default: false)

## Configuration

Edit constants in `scrape.go`:

```go
const (
    APIURL              = "https://graphql.anilist.co"
    PER_PAGE            = 50
    STAFF_BATCH_SIZE    = 20
    PROXY_PORT_START    = 10001
    PROXY_PORT_END      = 10100
    MIN_SECONDS_PER_PROXY = 2.0  // Rate limit: 30 req/min per proxy

    EXAMPLE_PROXY = "host:port:username:password"
)
```

## Output Files

### Full Scrape:
- `anime.csv` - All anime data
- `staff.csv` - All staff data
- `anime_staff_edges.csv` - Anime-staff relationships
- `scraper_state.json` - State for incremental scraping
- `failed_pages.txt` - Log of failed requests

### Incremental Scrape:
- `anime_delta.csv` - New/updated anime only
- `staff_delta.csv` - New/updated staff only
- `anime_staff_edges_delta.csv` - New relationships only
- `scraper_state.json` - Updated state

## State File Format

```json
{
  "last_scrape_timestamp": "2025-12-16T17:41:20Z",
  "max_anime_id": 204221,
  "max_staff_id": 382800,
  "total_anime_scraped": 21764,
  "total_staff_scraped": 88197,
  "scrape_mode": "incremental",
  "csv_files": {
    "anime": "anime_delta.csv",
    "staff": "staff_delta.csv",
    "edges": "anime_staff_edges_delta.csv"
  }
}
```

## Performance

With 100 proxies:
- **Theoretical max**: 50 req/s (30 req/min per proxy)
- **Typical full scrape**: ~565 seconds (~9.5 minutes)
- **Typical incremental**: ~30-60 seconds

## Integration with Nuxt App

1. **Manual trigger**:
   ```bash
   curl -X POST http://localhost:3000/api/admin/trigger-incremental-scrape
   ```

2. **Automatic (24-hour schedule)**:
   - Enabled via `ENABLE_SCHEDULER=true` in `.env`
   - Configured in `nuxt-app/server/plugins/scheduler.ts`

## Troubleshooting

### "Could not load state file"
- Normal on first run - will fall back to full scrape
- Ensure `scraper_state.json` exists for incremental mode

### Rate Limit Errors
- Adjust `MIN_SECONDS_PER_PROXY` constant
- Reduce number of proxies
- Check proxy credentials

### Missing Delta Files
- Expected if no new data in that category
- Delta ingestion endpoint gracefully handles missing files

## Dependencies

- Go 1.16+
- Valid proxy credentials
- AniList GraphQL API access
