package main

// sakugabooru_lookup.go
//
// Sakugabooru artist tag → staff matching (offline, no API calls).
//
// Reads staff rows from stdin as CSV (exported from PostgreSQL) and matches
// each against artist tags from a pre-downloaded sakugabooru_tags.json.
//
// Strategies per artist tag, in priority order:
//
//  1. Exact match — tag name matches reversed(name_en) or forward(name_en)
//  2. Token match — all tokens in the tag name appear in a staff name
//
// Writes results CSV to stdout:
//   staff_id, name_en, sakugabooru_tag, post_count, tag_type, found, method
//
// Usage:
//
//	psql "$DATABASE_URL" -t -A -F',' \
//	  -c "SELECT staff_id, COALESCE(name_en,''), COALESCE(name_ja,'') FROM staff ORDER BY staff_id" \
//	  | ./sakugabooru_lookup -tags sakugabooru_tags.json > results.csv

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

type sakugaTag struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
	Type      int    `json:"type"`
	Ambiguous bool   `json:"ambiguous"`
}

type sakugaTagsFile struct {
	Artist []sakugaTag `json:"artist"`
}

// ---------------------------------------------------------------------------
// Name normalization helpers
// ---------------------------------------------------------------------------

// normalize converts a name string to Sakugabooru tag format:
// lowercase, spaces→underscores, keeping only [a-z0-9_-].
func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	return strings.Trim(result, "_")
}

// reversed returns the name tokens in reversed order joined by underscores.
// "Yutaka Nakamura" → "nakamura_yutaka"
func reversed(nameEn string) string {
	parts := strings.Fields(strings.TrimSpace(nameEn))
	if len(parts) < 2 {
		return normalize(nameEn)
	}
	rev := make([]string, len(parts))
	for i, p := range parts {
		rev[len(parts)-1-i] = p
	}
	return normalize(strings.Join(rev, " "))
}

// forward returns the name tokens in original order joined by underscores.
// "Yutaka Nakamura" → "yutaka_nakamura"
func forward(nameEn string) string {
	return normalize(nameEn)
}

// ---------------------------------------------------------------------------
// Data structures
// ---------------------------------------------------------------------------

type staffRow struct {
	staffID          string
	nameEn           string
	nameJa           string
	alternativeNames []string // pipe-delimited in CSV
}

type lookupResult struct {
	staffID   string
	nameEn    string
	sakugaTag string
	postCount int
	tagType   int
	found     bool
	method    string
}

type ambiguousMatch struct {
	staffID    string
	nameEn     string
	candidates []sakugaTag
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	tagsPath := flag.String("tags", "sakugabooru_tags.json", "Path to sakugabooru_tags.json")
	ambigFile := flag.String("ambiguous", "sakugabooru_ambiguous.csv", "File for ambiguous token-match candidates")
	flag.Parse()

	// ---- Read stdin CSV: staff_id, name_en, name_ja ----
	r := csv.NewReader(bufio.NewReader(os.Stdin))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var all []staffRow
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
		s := staffRow{staffID: strings.TrimSpace(row[0])}
		if len(row) > 1 {
			s.nameEn = strings.TrimSpace(row[1])
		}
		if len(row) > 2 {
			s.nameJa = strings.TrimSpace(row[2])
		}
		if len(row) > 3 && strings.TrimSpace(row[3]) != "" {
			for _, alt := range strings.Split(row[3], "|") {
				alt = strings.TrimSpace(alt)
				if alt != "" {
					s.alternativeNames = append(s.alternativeNames, alt)
				}
			}
		}
		all = append(all, s)
	}
	fmt.Fprintf(os.Stderr, "Loaded %d staff from stdin\n", len(all))

	// ---- Build lookup indexes from staff rows ----
	// Map normalized name → list of indices into all
	reversedIndex := make(map[string][]int) // reversed(name_en) → indices
	forwardIndex := make(map[string][]int)  // forward(name_en) → indices

	for i, s := range all {
		if strings.TrimSpace(s.nameEn) == "" {
			continue
		}
		// Index primary name
		rev := reversed(s.nameEn)
		if rev != "" {
			reversedIndex[rev] = append(reversedIndex[rev], i)
		}
		fwd := forward(s.nameEn)
		if fwd != "" && fwd != rev {
			forwardIndex[fwd] = append(forwardIndex[fwd], i)
		}
		// Index alternative names
		for _, alt := range s.alternativeNames {
			altRev := reversed(alt)
			if altRev != "" {
				reversedIndex[altRev] = append(reversedIndex[altRev], i)
			}
			altFwd := forward(alt)
			if altFwd != "" && altFwd != altRev {
				forwardIndex[altFwd] = append(forwardIndex[altFwd], i)
			}
		}
	}

	// ---- Load artist tags from JSON ----
	tf, err := os.Open(*tagsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", *tagsPath, err)
		os.Exit(1)
	}
	var tags sakugaTagsFile
	if err := json.NewDecoder(tf).Decode(&tags); err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding %s: %v\n", *tagsPath, err)
		os.Exit(1)
	}
	tf.Close()
	fmt.Fprintf(os.Stderr, "Loaded %d artist tags\n", len(tags.Artist))

	// ---- Match artist tags → staff ----
	results := make(map[string]lookupResult, len(all))
	var ambiguousMatches []ambiguousMatch

	for _, tag := range tags.Artist {
		tagName := tag.Name

		// Strategy 1: Exact match against reversed index
		if indices, ok := reversedIndex[tagName]; ok {
			for _, idx := range indices {
				s := all[idx]
				prev, exists := results[s.staffID]
				if !exists || !prev.found || tag.Count > prev.postCount {
					results[s.staffID] = lookupResult{
						staffID: s.staffID, nameEn: s.nameEn,
						sakugaTag: tagName, postCount: tag.Count, tagType: tag.Type,
						found: true, method: "exact_reversed",
					}
				}
			}
			continue
		}

		// Strategy 1b: Exact match against forward index
		if indices, ok := forwardIndex[tagName]; ok {
			for _, idx := range indices {
				s := all[idx]
				prev, exists := results[s.staffID]
				if !exists || !prev.found || tag.Count > prev.postCount {
					results[s.staffID] = lookupResult{
						staffID: s.staffID, nameEn: s.nameEn,
						sakugaTag: tagName, postCount: tag.Count, tagType: tag.Type,
						found: true, method: "exact_forward",
					}
				}
			}
			continue
		}

	}

	// ---- Summary stats ----
	var found, notFound, noName, ambiguous int
	for _, s := range all {
		res, ok := results[s.staffID]
		switch {
		case strings.TrimSpace(s.nameEn) == "":
			noName++
			if !ok {
				results[s.staffID] = lookupResult{
					staffID: s.staffID, nameEn: s.nameEn,
					method: "no_name",
				}
			}
		case !ok || !res.found:
			notFound++
			if !ok {
				results[s.staffID] = lookupResult{
					staffID: s.staffID, nameEn: s.nameEn,
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
	fmt.Fprintf(os.Stderr, "\nDone: %d found (%d ambiguous), %d not found, %d had no name (total %d)\n",
		found, ambiguous, notFound, noName, len(all))

	// ---- Write ambiguous candidates file ----
	if len(ambiguousMatches) > 0 {
		af, err := os.Create(*ambigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create %s: %v\n", *ambigFile, err)
		} else {
			aw := csv.NewWriter(af)
			aw.Write([]string{"staff_id", "name_en", "candidate_tag", "post_count", "tag_type"})
			for _, am := range ambiguousMatches {
				for _, c := range am.candidates {
					aw.Write([]string{
						am.staffID, am.nameEn,
						c.Name, fmt.Sprintf("%d", c.Count), fmt.Sprintf("%d", c.Type),
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
	w.Write([]string{"staff_id", "name_en", "sakugabooru_tag", "post_count", "tag_type", "found", "method"})
	for _, s := range all {
		res := results[s.staffID]
		foundStr := "0"
		if res.found {
			foundStr = "1"
		}
		w.Write([]string{
			s.staffID,
			s.nameEn,
			res.sakugaTag,
			fmt.Sprintf("%d", res.postCount),
			fmt.Sprintf("%d", res.tagType),
			foundStr,
			res.method,
		})
	}
	w.Flush()
}

