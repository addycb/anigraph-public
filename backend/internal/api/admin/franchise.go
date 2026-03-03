package admin

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"connectrpc.com/connect"

	pb "anigraph/backend/gen/anigraph/v1"
	"anigraph/backend/internal/api/httputil"
	oaiClient "anigraph/backend/internal/openai"
)

// GenerateFranchises generates franchises from anime relations using Union-Find.
func (h *Handler) GenerateFranchises(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	skipAI := r.URL.Query().Get("skipAI") == "true"

	log.Println("Starting franchise generation")

	// Get all anime relations.
	rows, err := h.pg.Query(ctx, `
		SELECT source_anilist_id, target_anilist_id, relation_type
		FROM anime_relation
		ORDER BY source_anilist_id`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	type relation struct {
		sourceID, targetID int32
		relType            string
	}
	var relations []relation
	for rows.Next() {
		var r relation
		if err := rows.Scan(&r.sourceID, &r.targetID, &r.relType); err == nil {
			relations = append(relations, r)
		}
	}

	if len(relations) == 0 {
		httputil.JSON(w, http.StatusOK, map[string]any{"success": true, "message": "No relations found.", "franchises": 0})
		return
	}

	// Union-Find to compute connected components (exclude crossovers).
	uf := newUnionFind()
	for _, rel := range relations {
		if rel.relType == "CHARACTER" || rel.relType == "OTHER" {
			continue // Don't bridge franchises through crossovers.
		}
		uf.union(rel.sourceID, rel.targetID)
	}

	// Group anime by component.
	components := map[int32][]int32{}
	for id := range uf.parent {
		root := uf.find(id)
		components[root] = append(components[root], id)
	}

	// Filter to components with 2+ anime.
	var franchiseGroups [][]int32
	for _, ids := range components {
		if len(ids) >= 2 {
			franchiseGroups = append(franchiseGroups, ids)
		}
	}

	log.Printf("Found %d franchise groups from %d relations", len(franchiseGroups), len(relations))

	// Clear existing franchises.
	h.pg.Exec(ctx, "UPDATE anime SET franchise_id = NULL")
	h.pg.Exec(ctx, "DELETE FROM franchise")

	// Generate franchise names and insert.
	var openaiC *oaiClient.Client
	if !skipAI {
		openaiC = oaiClient.NewClient()
	}

	created := 0
	for _, group := range franchiseGroups {
		// Get titles for this group.
		titleRows, err := h.pg.Query(ctx, `
			SELECT anilist_id, COALESCE(title_english, ''), COALESCE(title_romaji, ''), COALESCE(title, '')
			FROM anime WHERE anilist_id = ANY($1)`, group)
		if err != nil {
			continue
		}

		var titles []string
		for titleRows.Next() {
			var id int32
			var eng, rom, title string
			if err := titleRows.Scan(&id, &eng, &rom, &title); err == nil {
				t := eng
				if t == "" {
					t = rom
				}
				if t == "" {
					t = title
				}
				if t != "" {
					titles = append(titles, t)
				}
			}
		}
		titleRows.Close()

		if len(titles) == 0 {
			continue
		}

		// Determine franchise name.
		name := determineFranchiseName(ctx, openaiC, titles)
		if name == "" {
			name = titles[0]
		}

		// Insert franchise.
		var franchiseID int32
		err = h.pg.QueryRow(ctx,
			"INSERT INTO franchise (title) VALUES ($1) RETURNING id", name).Scan(&franchiseID)
		if err != nil {
			log.Printf("[franchise] Insert failed for %q: %v", name, err)
			continue
		}

		// Update anime with franchise_id.
		_, err = h.pg.Exec(ctx,
			"UPDATE anime SET franchise_id = $1 WHERE anilist_id = ANY($2)",
			franchiseID, group)
		if err != nil {
			log.Printf("[franchise] Update failed: %v", err)
			continue
		}

		created++

		// Rate limit for OpenAI.
		if openaiC != nil {
			time.Sleep(oaiClient.RateLimitDelay)
		}
	}

	duration := time.Since(start)
	log.Printf("Franchise generation complete: %d created in %v", created, duration)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"message":    fmt.Sprintf("Generated %d franchises", created),
		"franchises": created,
		"groups":     len(franchiseGroups),
		"duration":   fmt.Sprintf("%dms", duration.Milliseconds()),
	})
}

// RenameFranchises renames existing franchises using AI.
func (h *Handler) RenameFranchises(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()

	log.Println("Starting franchise renaming")

	openaiC := oaiClient.NewClient()
	if openaiC == nil {
		httputil.Error(w, http.StatusServiceUnavailable, "OpenAI not configured")
		return
	}

	rows, err := h.pg.Query(ctx, `
		SELECT f.id, f.title, array_agg(COALESCE(a.title_english, a.title_romaji, a.title))
		FROM franchise f
		JOIN anime a ON a.franchise_id = f.id
		GROUP BY f.id, f.title
		ORDER BY f.id`)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Query failed: %v", err))
		return
	}
	defer rows.Close()

	renamed := 0
	for rows.Next() {
		var id int32
		var currentTitle string
		var titles []string
		if err := rows.Scan(&id, &currentTitle, &titles); err != nil {
			continue
		}

		if len(titles) < 2 {
			continue
		}

		name := determineFranchiseName(ctx, openaiC, titles)
		if name != "" && name != currentTitle {
			_, err := h.pg.Exec(ctx, "UPDATE franchise SET title = $1 WHERE id = $2", name, id)
			if err == nil {
				renamed++
				log.Printf("[rename] %q → %q", currentTitle, name)
			}
		}

		time.Sleep(oaiClient.RateLimitDelay)
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  fmt.Sprintf("Renamed %d franchises", renamed),
		"renamed":  renamed,
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
	})
}

// GenerateRecommendations runs the Go recommendations binary.
func (h *Handler) GenerateRecommendations(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Println("Starting recommendation generation")

	dataDir := "/app/data/neo4j_import"
	incremental := r.URL.Query().Get("incremental") == "true"

	req := connect.NewRequest(&pb.ComputeRecommendationsRequest{
		DataDir:     dataDir,
		Incremental: incremental,
	})

	stream, err := h.recommendations.ComputeRecommendations(r.Context(), req)
	if err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed: %v", err))
		return
	}

	var lastMsg string
	for stream.Receive() {
		msg := stream.Msg()
		if p := msg.GetProgress(); p != nil {
			lastMsg = p.Message
			log.Printf("[recommendations] %s", p.Message)
		}
	}
	if err := stream.Err(); err != nil {
		httputil.Error(w, http.StatusInternalServerError, fmt.Sprintf("Failed: %v", err))
		return
	}

	duration := time.Since(start)
	httputil.JSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"message":  "Recommendations generated",
		"lastMsg":  lastMsg,
		"duration": fmt.Sprintf("%dms", duration.Milliseconds()),
	})
}

// Union-Find data structure for franchise grouping.
type unionFind struct {
	parent map[int32]int32
	rank   map[int32]int32
}

func newUnionFind() *unionFind {
	return &unionFind{
		parent: make(map[int32]int32),
		rank:   make(map[int32]int32),
	}
}

func (uf *unionFind) find(x int32) int32 {
	if _, ok := uf.parent[x]; !ok {
		uf.parent[x] = x
		uf.rank[x] = 0
	}
	if uf.parent[x] != x {
		uf.parent[x] = uf.find(uf.parent[x])
	}
	return uf.parent[x]
}

func (uf *unionFind) union(x, y int32) {
	rx, ry := uf.find(x), uf.find(y)
	if rx == ry {
		return
	}
	if uf.rank[rx] < uf.rank[ry] {
		uf.parent[rx] = ry
	} else if uf.rank[rx] > uf.rank[ry] {
		uf.parent[ry] = rx
	} else {
		uf.parent[ry] = rx
		uf.rank[rx]++
	}
}

// determineFranchiseName uses AI-first naming with fallback to phrase extraction.
func determineFranchiseName(ctx context.Context, openaiC *oaiClient.Client, titles []string) string {
	// Try AI first.
	if openaiC != nil {
		result, err := openaiC.GenerateFranchiseName(ctx, titles)
		if err == nil && result != nil && result.Name != "" && result.Confidence > 0.5 {
			return result.Name
		}
		if err != nil {
			log.Printf("[franchise] AI naming failed: %v", err)
		}
	}

	// Fallback to phrase extraction.
	return findMostCommonPhrase(titles)
}

// findMostCommonPhrase extracts the most common word-boundary-aligned phrase across titles.
func findMostCommonPhrase(titles []string) string {
	if len(titles) == 0 {
		return ""
	}
	if len(titles) == 1 {
		return titles[0]
	}

	// Normalize titles.
	normalized := make([]string, len(titles))
	for i, t := range titles {
		normalized[i] = strings.ToLower(strings.TrimSpace(t))
	}

	// Extract all word-level subphrases.
	type phraseScore struct {
		phrase   string
		coverage float64
		score    float64
	}
	var candidates []phraseScore

	for _, t := range normalized {
		words := strings.Fields(t)
		for start := 0; start < len(words); start++ {
			for end := start + 1; end <= len(words) && end-start <= 8; end++ {
				phrase := strings.Join(words[start:end], " ")
				if len(phrase) < 3 {
					continue
				}

				// Count how many titles contain this phrase.
				matching := 0
				for _, other := range normalized {
					if strings.Contains(other, phrase) {
						matching++
					}
				}

				coverage := float64(matching) / float64(len(titles))
				if coverage < 0.25 {
					continue
				}

				wordCount := float64(end - start)
				score := coverage * math.Sqrt(wordCount)
				candidates = append(candidates, phraseScore{phrase, coverage, score})
			}
		}
	}

	if len(candidates) == 0 {
		return titles[0]
	}

	// Sort by score descending.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Title-case the result.
	return titleCase(candidates[0].phrase)
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			runes := []rune(w)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}
