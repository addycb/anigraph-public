package main

// sakugabooru_anime.go
//
// Sakugabooru copyright tag → anime matching (offline, no API calls).
//
// Reads anime rows from stdin as CSV (exported from PostgreSQL) and matches
// each against copyright tags from a pre-downloaded sakugabooru_tags.json.
//
// Strategies per copyright tag, in priority order:
//
//  1. Exact match — normalized tag name matches title_romaji or title_english
//  2. Token match — all tokens in the tag name appear in an anime title
//
// Writes results CSV to stdout:
//   anilist_id, title_english, title_romaji, sakugabooru_tag, post_count, found, method
//
// Usage:
//
//	psql "$DATABASE_URL" -t -A -F',' \
//	  -c "SELECT anilist_id, COALESCE(title_english,''), COALESCE(title_romaji,'')
//	      FROM anime ORDER BY anilist_id" \
//	  | ./sakugabooru_anime -tags sakugabooru_tags.json > results.csv

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// ---------------------------------------------------------------------------
// Tag JSON structures
// ---------------------------------------------------------------------------

type tagEntry struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
	Type      int    `json:"type"`
	Ambiguous bool   `json:"ambiguous"`
}

type tagsFile struct {
	Copyright []tagEntry `json:"copyright"`
}

// ---------------------------------------------------------------------------
// Title normalization
// ---------------------------------------------------------------------------

// normalizeTitle converts a title to Sakugabooru tag format:
// lowercase, spaces→underscores, keeping only [a-z0-9_].
// Hyphens/colons/apostrophes are dropped (e.g. "Re:Zero" → "rezero").
func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Replace common punctuation-as-separator with space before stripping
	for _, sep := range []string{":", " - ", "–", "—"} {
		s = strings.ReplaceAll(s, sep, " ")
	}
	s = strings.ReplaceAll(s, " ", "_")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		}
	}
	// Collapse repeated underscores
	result := b.String()
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	return strings.Trim(result, "_")
}

// ---------------------------------------------------------------------------
// Data structures
// ---------------------------------------------------------------------------

type animeRow struct {
	anilistID    string
	titleEnglish string
	titleRomaji  string
	synonyms     []string // pipe-delimited in CSV
}

type animeResult struct {
	anilistID    string
	titleEnglish string
	titleRomaji  string
	sakugaTag    string
	postCount    int
	found        bool
	method       string
}

type ambiguousMatch struct {
	anilistID    string
	titleEnglish string
	titleRomaji  string
	candidates   []tagEntry
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	tagsPath := flag.String("tags", "sakugabooru_tags.json", "Path to sakugabooru_tags.json")
	ambigFile := flag.String("ambiguous", "sakugabooru_anime_ambiguous.csv", "File for ambiguous token-match candidates")
	flag.Parse()

	// ---- Read stdin CSV: anilist_id, title_english, title_romaji ----
	r := csv.NewReader(bufio.NewReader(os.Stdin))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var all []animeRow
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "CSV error: %v\n", err)
			continue
		}
		if len(row) < 1 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		a := animeRow{anilistID: strings.TrimSpace(row[0])}
		if len(row) > 1 {
			a.titleEnglish = strings.TrimSpace(row[1])
		}
		if len(row) > 2 {
			a.titleRomaji = strings.TrimSpace(row[2])
		}
		if len(row) > 3 && strings.TrimSpace(row[3]) != "" {
			for _, syn := range strings.Split(row[3], "|") {
				syn = strings.TrimSpace(syn)
				if syn != "" {
					a.synonyms = append(a.synonyms, syn)
				}
			}
		}
		all = append(all, a)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d anime from stdin\n", len(all))

	// ---- Build lookup indexes from anime rows ----
	// Map normalized title → list of anime rows (multiple anime could normalize the same)
	romajiIndex := make(map[string][]int)  // normalized romaji → indices into all
	englishIndex := make(map[string][]int)  // normalized english → indices into all

	synonymIndex := make(map[string][]int)

	for i, a := range all {
		if a.titleRomaji != "" {
			key := normalizeTitle(a.titleRomaji)
			if key != "" {
				romajiIndex[key] = append(romajiIndex[key], i)
			}
		}
		if a.titleEnglish != "" {
			key := normalizeTitle(a.titleEnglish)
			if key != "" {
				englishIndex[key] = append(englishIndex[key], i)
			}
		}
		for _, syn := range a.synonyms {
			key := normalizeTitle(syn)
			if key != "" {
				synonymIndex[key] = append(synonymIndex[key], i)
			}
		}
	}

	// ---- Load copyright tags from JSON ----
	tf, err := os.Open(*tagsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", *tagsPath, err)
		os.Exit(1)
	}
	var tags tagsFile
	if err := json.NewDecoder(tf).Decode(&tags); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding %s: %v\n", *tagsPath, err)
		os.Exit(1)
	}
	tf.Close()
	fmt.Fprintf(os.Stderr, "Loaded %d copyright tags\n", len(tags.Copyright))

	// ---- Match copyright tags → anime ----
	// Track results per anime (best match wins: exact > token)
	results := make(map[string]animeResult, len(all))
	var ambiguousMatches []ambiguousMatch

	for _, tag := range tags.Copyright {
		tagName := tag.Name

		// Strategy 1: Exact match against romaji index
		if indices, ok := romajiIndex[tagName]; ok {
			for _, idx := range indices {
				a := all[idx]
				prev, exists := results[a.anilistID]
				if !exists || !prev.found || tag.Count > prev.postCount {
					results[a.anilistID] = animeResult{
						anilistID: a.anilistID, titleEnglish: a.titleEnglish, titleRomaji: a.titleRomaji,
						sakugaTag: tagName, postCount: tag.Count,
						found: true, method: "exact_romaji",
					}
				}
			}
			continue
		}

		// Strategy 1b: Exact match against english index
		if indices, ok := englishIndex[tagName]; ok {
			for _, idx := range indices {
				a := all[idx]
				prev, exists := results[a.anilistID]
				if !exists || !prev.found || tag.Count > prev.postCount {
					results[a.anilistID] = animeResult{
						anilistID: a.anilistID, titleEnglish: a.titleEnglish, titleRomaji: a.titleRomaji,
						sakugaTag: tagName, postCount: tag.Count,
						found: true, method: "exact_english",
					}
				}
			}
			continue
		}

		// Strategy 1c: Exact match against synonym index
		if indices, ok := synonymIndex[tagName]; ok {
			for _, idx := range indices {
				a := all[idx]
				prev, exists := results[a.anilistID]
				if !exists || !prev.found || tag.Count > prev.postCount {
					results[a.anilistID] = animeResult{
						anilistID: a.anilistID, titleEnglish: a.titleEnglish, titleRomaji: a.titleRomaji,
						sakugaTag: tagName, postCount: tag.Count,
						found: true, method: "exact_synonym",
					}
				}
			}
			continue
		}

	}

	// ---- Summary ----
	var found, notFound, noTitle, ambiguous int
	for _, a := range all {
		res, ok := results[a.anilistID]
		switch {
		case a.titleRomaji == "" && a.titleEnglish == "":
			noTitle++
			if !ok {
				results[a.anilistID] = animeResult{
					anilistID: a.anilistID, titleEnglish: a.titleEnglish, titleRomaji: a.titleRomaji,
					method: "no_title",
				}
			}
		case !ok || !res.found:
			notFound++
			if !ok {
				results[a.anilistID] = animeResult{
					anilistID: a.anilistID, titleEnglish: a.titleEnglish, titleRomaji: a.titleRomaji,
					method: "not_found",
				}
			}
		case strings.HasPrefix(res.method, "token_ambiguous"):
			ambiguous++
			found++
		default:
			found++
		}
	}
	fmt.Fprintf(os.Stderr, "\nDone: %d found (%d ambiguous), %d not found, %d no title (total %d)\n",
		found, ambiguous, notFound, noTitle, len(all))

	// ---- Write ambiguous file ----
	if len(ambiguousMatches) > 0 {
		af, err := os.Create(*ambigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create %s: %v\n", *ambigFile, err)
		} else {
			aw := csv.NewWriter(af)
			aw.Write([]string{"anilist_id", "title_english", "title_romaji", "candidate_tag", "post_count"})
			for _, am := range ambiguousMatches {
				for _, c := range am.candidates {
					aw.Write([]string{
						am.anilistID, am.titleEnglish, am.titleRomaji,
						c.Name, fmt.Sprintf("%d", c.Count),
					})
				}
			}
			aw.Flush()
			af.Close()
			fmt.Fprintf(os.Stderr, "Wrote ambiguous candidates to %s\n", *ambigFile)
		}
	}

	// ---- Write main results CSV to stdout ----
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"anilist_id", "title_english", "title_romaji", "sakugabooru_tag", "post_count", "found", "method"})
	for _, a := range all {
		res := results[a.anilistID]
		foundStr := "0"
		if res.found {
			foundStr = "1"
		}
		w.Write([]string{
			a.anilistID,
			a.titleEnglish,
			a.titleRomaji,
			res.sakugaTag,
			fmt.Sprintf("%d", res.postCount),
			foundStr,
			res.method,
		})
	}
	w.Flush()
}

