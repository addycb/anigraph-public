package main

// Precompute top-48 similar OPs by cosine similarity.
//
// Loads all embeddings from anime_op_embedding into memory,
// computes pairwise cosine similarity per OP, keeps top 48 per source OP,
// and writes results into anime_similar_op.
//
// Usage:
//
//	cd backend && DATABASE_URL=postgres://anigraph:pass@localhost:4832/anigraph go run compute_similar_ops.go

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

type opEmbedding struct {
	animeID   int
	opNumber  int
	embedding []float32
	norm      float64
}

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

	// Load all embeddings
	rows, err := pool.Query(ctx, `SELECT anime_id, op_number, embedding FROM anime_op_embedding`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var items []opEmbedding
	for rows.Next() {
		var animeID, opNumber int
		var emb []float32
		if err := rows.Scan(&animeID, &opNumber, &emb); err != nil {
			fmt.Fprintf(os.Stderr, "scan: %v\n", err)
			continue
		}
		norm := vecNorm(emb)
		items = append(items, opEmbedding{animeID: animeID, opNumber: opNumber, embedding: emb, norm: norm})
	}
	rows.Close()

	fmt.Fprintf(os.Stderr, "Loaded %d OP embeddings\n", len(items))
	if len(items) < 2 {
		fmt.Fprintln(os.Stderr, "Not enough embeddings to compute similarities")
		os.Exit(0)
	}

	// Clear existing data
	_, err = pool.Exec(ctx, `TRUNCATE anime_similar_op`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "truncate: %v\n", err)
		os.Exit(1)
	}

	// Compute pairwise similarities and insert top 48
	type simPair struct {
		animeID  int
		opNumber int
		sim      float64
	}

	total := 0
	for i, a := range items {
		if (i+1)%500 == 0 {
			fmt.Fprintf(os.Stderr, "Processing %d/%d...\n", i+1, len(items))
		}

		sims := make([]simPair, 0, len(items)-1)
		for j, b := range items {
			if i == j || b.animeID == a.animeID {
				continue
			}
			sim := cosineSim(a.embedding, b.embedding, a.norm, b.norm)
			sims = append(sims, simPair{animeID: b.animeID, opNumber: b.opNumber, sim: sim})
		}

		sort.Slice(sims, func(x, y int) bool {
			return sims[x].sim > sims[y].sim
		})

		// Deduplicate: keep only the best-matching OP per anime
		seen := make(map[int]bool)
		var deduped []simPair
		for _, s := range sims {
			if seen[s.animeID] {
				continue
			}
			seen[s.animeID] = true
			deduped = append(deduped, s)
			if len(deduped) == 48 {
				break
			}
		}

		for rank, s := range deduped {
			_, err := pool.Exec(ctx, `
				INSERT INTO anime_similar_op (anime_id, op_number, similar_anime_id, similar_op_number, similarity, rank)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				a.animeID, a.opNumber, s.animeID, s.opNumber, s.sim, rank+1)
			if err != nil {
				fmt.Fprintf(os.Stderr, "insert: %v\n", err)
			}
			total++
		}
	}

	fmt.Fprintf(os.Stderr, "Done: inserted %d similarity pairs for %d OPs\n", total, len(items))
}

func vecNorm(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}

func cosineSim(a, b []float32, normA, normB float64) float64 {
	if normA == 0 || normB == 0 {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot / (normA * normB)
}
