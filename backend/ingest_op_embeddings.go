package main

// Ingest matched OP video embeddings into Postgres.
//
// Reads matched_embeddings.csv (columns: anilist_id, title_op, op_number, embedding).
// Inserts all OPs per anime (not just the first).
// Upserts into anime_op_embedding table.
//
// Usage:
//
//	cd backend && DATABASE_URL=postgres://anigraph:pass@localhost:5432/anigraph go run ingest_op_embeddings.go < matched_embeddings.csv

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var opNumRe = regexp.MustCompile(`-OP(\d+)`)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Read CSV from stdin
	reader := csv.NewReader(os.Stdin)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	// Skip header
	if _, err := reader.Read(); err != nil {
		fmt.Fprintf(os.Stderr, "read header: %v\n", err)
		os.Exit(1)
	}

	inserted, skipped := 0, 0
	rowCount := 0
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "csv error: %v\n", err)
			continue
		}
		if len(row) < 4 {
			continue
		}
		rowCount++

		anilistID, err := strconv.Atoi(strings.TrimSpace(row[0]))
		if err != nil {
			skipped++
			continue
		}
		titleOP := strings.Trim(strings.TrimSpace(row[1]), "\"")
		opNum := 1
		if n, err := strconv.Atoi(strings.TrimSpace(row[2])); err == nil && n > 0 {
			opNum = n
		} else {
			// Parse from title_op, e.g. "InuYasha-OP3" → 3
			if m := opNumRe.FindStringSubmatch(titleOP); m != nil {
				if n, err := strconv.Atoi(m[1]); err == nil {
					opNum = n
				}
			}
		}
		embedding := strings.TrimSpace(row[3])

		// Parse embedding string "[0.01, -0.02, ...]" into Postgres float4[] literal
		embStr := strings.Trim(embedding, "[]")
		floats := strings.Split(embStr, ",")
		pgArray := "{" + strings.Join(trimAll(floats), ",") + "}"

		var animeDBID int
		err = pool.QueryRow(ctx, `SELECT id FROM anime WHERE anilist_id = $1`, anilistID).Scan(&animeDBID)
		if err != nil {
			skipped++
			continue
		}

		tag, err := pool.Exec(ctx, `
			INSERT INTO anime_op_embedding (anime_id, op_number, title_op, embedding)
			VALUES ($1, $2, $3, $4::float4[])
			ON CONFLICT (anime_id, op_number) DO UPDATE SET
				title_op = EXCLUDED.title_op,
				embedding = EXCLUDED.embedding`,
			animeDBID, opNum, titleOP, pgArray)
		if err != nil {
			fmt.Fprintf(os.Stderr, "insert anilist_id=%d op=%d: %v\n", anilistID, opNum, err)
			skipped++
			continue
		}
		if tag.RowsAffected() > 0 {
			inserted++
		} else {
			skipped++
		}
	}

	fmt.Fprintf(os.Stderr, "Read %d rows. Inserted/updated=%d skipped=%d\n", rowCount, inserted, skipped)
}

func trimAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = strings.TrimSpace(s)
	}
	return out
}
