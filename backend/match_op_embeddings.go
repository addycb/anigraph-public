package main

// Match AnimeThemes OP video embeddings to AniList IDs.
//
// Two-pass matching:
//   Pass 1: Fetch /anime endpoint → build normalized(slug) → anilist_id + synonym lookups
//   Pass 2: For unmatched titles only, fetch /animetheme endpoint → build
//           normalized(video_filename) → anime_slug → anilist_id
//
// Reads CSV from stdin (columns: title, title_op, embedding).
// Writes matched CSV to stdout: anilist_id, title_op, op_number, embedding
//
// Usage:
//
//	cd backend && go run match_op_embeddings.go < op_embeddings_raw.csv > matched_embeddings.csv

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	animeThemesBaseAPI = "https://api.animethemes.moe"
	animeThemesAgent   = "AniGraph-OPMatcher/1.0 (anigraph.xyz)"
	pageSize           = 100
)

// --- API response types ---

type paginatedLinks struct {
	Next string `json:"next"`
}

type animeResponse struct {
	Anime []animeRecord  `json:"anime"`
	Links paginatedLinks `json:"links"`
}

type animeRecord struct {
	Slug      string            `json:"slug"`
	Resources []resourceRecord  `json:"resources"`
	Synonyms  []synonymRecord   `json:"synonyms"`
}

type resourceRecord struct {
	Site       string `json:"site"`
	ExternalID int    `json:"external_id"`
}

type synonymRecord struct {
	Text string `json:"text"`
}

type animeThemeResponse struct {
	AnimeThemes []animeThemeRecord `json:"animethemes"`
	Links       paginatedLinks     `json:"links"`
}

type animeThemeRecord struct {
	Anime              animeThemeAnime           `json:"anime"`
	AnimeThemeEntries  []animeThemeEntryRecord   `json:"animethemeentries"`
}

type animeThemeAnime struct {
	Slug string `json:"slug"`
}

type animeThemeEntryRecord struct {
	Videos []videoRecord `json:"videos"`
}

type videoRecord struct {
	Filename string `json:"filename"`
}

// --- Helpers ---

var opNumberRe = regexp.MustCompile(`-(?:OP|ED)(\d+)$`)
var nonAlnumRe = regexp.MustCompile(`[^a-z0-9]`)

// normalize strips all non-alphanumeric chars and lowercases.
func normalize(s string) string {
	return nonAlnumRe.ReplaceAllString(strings.ToLower(s), "")
}

// extractVideoBase returns the anime name part of a video filename.
// e.g. "Mahoromatic-OP1" → "Mahoromatic", "CrushGear-OP1v2" → "CrushGear"
func extractVideoBase(filename string) string {
	// Find the last -OP or -ED marker
	for _, marker := range []string{"-OP", "-ED"} {
		if idx := strings.LastIndex(filename, marker); idx > 0 {
			return filename[:idx]
		}
	}
	return filename
}

func doRequestWithRetry(client *http.Client, url string) (*http.Response, error) {
	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			fmt.Fprintf(os.Stderr, "    Retry %d/%d in %s...\n", attempt+1, maxAttempts, backoff)
			time.Sleep(backoff)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", animeThemesAgent)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    Request error (attempt %d): %v\n", attempt+1, err)
			continue
		}

		if resp.StatusCode == 429 {
			wait := 60
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if n, err := strconv.Atoi(ra); err == nil {
					wait = n
				}
			}
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "    Rate limited (429), waiting %ds...\n", wait)
			time.Sleep(time.Duration(wait) * time.Second)
			continue
		}
		if resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "    HTTP %d (attempt %d), retrying...\n", resp.StatusCode, attempt+1)
			continue
		}
		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		return resp, nil
	}
	return nil, fmt.Errorf("failed after %d attempts", maxAttempts)
}

func truncateURL(url string) string {
	if len(url) > 100 {
		return url[:100] + "..."
	}
	return url
}

// --- Pass 1: Anime endpoint → slug/synonym → anilist_id ---

func fetchAnimeSlugLookup(client *http.Client) (map[string]int, error) {
	lookup := make(map[string]int)
	url := fmt.Sprintf("%s/anime?page[size]=%d&include=resources,synonyms&fields[resource]=link,site,external_id&filter[has]=resources", animeThemesBaseAPI, pageSize)
	page := 0

	for url != "" {
		page++
		fmt.Fprintf(os.Stderr, "  [anime] page %d\n", page)

		resp, err := doRequestWithRetry(client, url)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}

		var data animeResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("page %d decode: %w", page, err)
		}
		resp.Body.Close()

		for _, anime := range data.Anime {
			var anilistID int
			for _, res := range anime.Resources {
				if res.Site == "AniList" && res.ExternalID > 0 {
					anilistID = res.ExternalID
					break
				}
			}
			if anilistID == 0 {
				continue
			}
			// Primary: slug without underscores
			key := strings.ToLower(strings.ReplaceAll(anime.Slug, "_", ""))
			lookup[key] = anilistID
			// Synonyms
			for _, syn := range anime.Synonyms {
				synKey := normalize(syn.Text)
				if synKey != "" && synKey != key {
					if _, exists := lookup[synKey]; !exists {
						lookup[synKey] = anilistID
					}
				}
			}
		}

		url = data.Links.Next
		if url != "" {
			time.Sleep(1 * time.Second)
		}
	}

	fmt.Fprintf(os.Stderr, "  [anime] %d pages, %d lookup entries\n", page, len(lookup))
	return lookup, nil
}

// --- Pass 2: AnimeTheme endpoint → video filename → slug → anilist_id ---

// fetchVideoFilenameLookup paginates the /animetheme endpoint to build
// normalized(video_base) → anime_slug mapping. Only processes until all
// needed titles are resolved or pages are exhausted.
func fetchVideoFilenameLookup(client *http.Client, needed map[string]bool, slugToAnilist map[string]int) (map[string]int, error) {
	lookup := make(map[string]int)
	remaining := len(needed)
	url := fmt.Sprintf("%s/animetheme?page[size]=%d&include=anime,animethemeentries.videos", animeThemesBaseAPI, pageSize)
	page := 0

	for url != "" && remaining > 0 {
		page++
		if page%10 == 0 {
			fmt.Fprintf(os.Stderr, "  [animetheme] page %d (remaining: %d)\n", page, remaining)
		}

		resp, err := doRequestWithRetry(client, url)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}

		var data animeThemeResponse
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("page %d decode: %w", page, err)
		}
		resp.Body.Close()

		for _, theme := range data.AnimeThemes {
			animeSlug := theme.Anime.Slug
			slugKey := strings.ToLower(strings.ReplaceAll(animeSlug, "_", ""))
			anilistID, hasAnilist := slugToAnilist[slugKey]
			if !hasAnilist {
				continue
			}

			for _, entry := range theme.AnimeThemeEntries {
				for _, video := range entry.Videos {
					base := extractVideoBase(video.Filename)
					normBase := normalize(base)
					if needed[normBase] {
						lookup[normBase] = anilistID
						delete(needed, normBase)
						remaining--
					}
				}
			}
		}

		url = data.Links.Next
		if url != "" {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Fprintf(os.Stderr, "  [animetheme] %d pages, %d filename matches found, %d still unmatched\n", page, len(lookup), remaining)
	return lookup, nil
}

// --- Main ---

func main() {
	unmatchedFile := flag.String("unmatched", "unmatched.csv", "File for unmatched titles")
	skipPass2 := flag.Bool("skip-pass2", false, "Skip pass 2 (video filename matching)")
	flag.Parse()

	client := &http.Client{Timeout: 30 * time.Second}

	// ---- Read stdin CSV ----
	fmt.Fprintln(os.Stderr, "Reading CSV from stdin...")
	csvR := csv.NewReader(bufio.NewReader(os.Stdin))
	csvR.FieldsPerRecord = -1
	csvR.LazyQuotes = true

	type entry struct {
		title     string
		titleOP   string
		embedding string
	}
	var entries []entry
	headerSkipped := false

	for {
		row, err := csvR.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "CSV error: %v\n", err)
			continue
		}
		if len(row) < 3 {
			continue
		}
		if !headerSkipped {
			lower0 := strings.ToLower(strings.TrimSpace(row[0]))
			if lower0 == "title" || lower0 == "a.title_romaji" {
				headerSkipped = true
				continue
			}
			headerSkipped = true
		}
		// The embedding is a bracketed array like [0.01, -0.02, ...] whose
	// internal commas cause the CSV reader to split it across row[2:].
	// Rejoin all fields from index 2 onward to reconstruct it.
	embedding := strings.TrimSpace(strings.Join(row[2:], ","))
	entries = append(entries, entry{
			title:     strings.TrimSpace(row[0]),
			titleOP:   strings.TrimSpace(row[1]),
			embedding: embedding,
		})
	}
	fmt.Fprintf(os.Stderr, "Loaded %d embeddings from stdin\n\n", len(entries))

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "Nothing to do.")
		os.Exit(0)
	}

	// ---- Pass 1: slug + synonym lookup ----
	fmt.Fprintln(os.Stderr, "Pass 1: Fetching anime slugs + synonyms...")
	slugLookup, err := fetchAnimeSlugLookup(client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Pass 1 failed: %v\n", err)
		os.Exit(1)
	}

	// First matching pass
	type unmatchedEntry struct {
		title   string
		titleOP string
	}
	w := csv.NewWriter(os.Stdout)
	w.Write([]string{"anilist_id", "title_op", "op_number", "embedding"})

	var matched, unmatched int
	unmatchedTitles := make(map[string]bool)
	var unmatchedEntries []unmatchedEntry
	var unmatchedForPass2 []int // indices into entries

	for i, e := range entries {
		key := normalize(e.title)
		anilistID, ok := slugLookup[key]
		if ok {
			opNumber := ""
			if m := opNumberRe.FindStringSubmatch(e.titleOP); m != nil {
				opNumber = m[1]
			}
			w.Write([]string{strconv.Itoa(anilistID), e.titleOP, opNumber, e.embedding})
			matched++
		} else {
			unmatchedForPass2 = append(unmatchedForPass2, i)
			if !unmatchedTitles[e.title] {
				unmatchedTitles[e.title] = true
			}
		}
	}

	fmt.Fprintf(os.Stderr, "\nPass 1 result: matched=%d unmatched=%d (unique: %d)\n\n", matched, len(unmatchedForPass2), len(unmatchedTitles))

	// ---- Pass 2: video filename lookup (only for unmatched) ----
	if len(unmatchedForPass2) > 0 && !*skipPass2 {
		fmt.Fprintln(os.Stderr, "Pass 2: Fetching video filenames for unmatched titles...")
		neededSet := make(map[string]bool, len(unmatchedTitles))
		for title := range unmatchedTitles {
			neededSet[normalize(title)] = true
		}

		filenameLookup, err := fetchVideoFilenameLookup(client, neededSet, slugLookup)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Pass 2 failed: %v\n", err)
		} else {
			pass2Matched := 0
			for _, idx := range unmatchedForPass2 {
				e := entries[idx]
				key := normalize(e.title)
				anilistID, ok := filenameLookup[key]
				if ok {
					opNumber := ""
					if m := opNumberRe.FindStringSubmatch(e.titleOP); m != nil {
						opNumber = m[1]
					}
					w.Write([]string{strconv.Itoa(anilistID), e.titleOP, opNumber, e.embedding})
					matched++
					pass2Matched++
					delete(unmatchedTitles, e.title)
				} else {
					unmatched++
					if !unmatchedTitles[e.title] {
						// Already removed by a previous match
					}
				}
			}
			fmt.Fprintf(os.Stderr, "Pass 2 result: +%d matched\n", pass2Matched)
		}
	} else if *skipPass2 {
		unmatched = len(unmatchedForPass2)
		fmt.Fprintln(os.Stderr, "Pass 2 skipped (--skip-pass2)")
	}

	w.Flush()

	// Rebuild unmatched entries list from remaining unmatchedTitles
	for _, idx := range unmatchedForPass2 {
		e := entries[idx]
		if unmatchedTitles[e.title] {
			unmatchedEntries = append(unmatchedEntries, unmatchedEntry{e.title, e.titleOP})
		}
	}
	// Deduplicate
	seen := make(map[string]bool)
	var dedupedUnmatched []unmatchedEntry
	for _, e := range unmatchedEntries {
		if !seen[e.title] {
			seen[e.title] = true
			dedupedUnmatched = append(dedupedUnmatched, e)
		}
	}

	fmt.Fprintf(os.Stderr, "\nFinal: matched=%d unmatched_unique_titles=%d total_entries=%d\n",
		matched, len(dedupedUnmatched), len(entries))

	// ---- Write unmatched CSV ----
	if len(dedupedUnmatched) > 0 {
		uf, err := os.Create(*unmatchedFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not create %s: %v\n", *unmatchedFile, err)
		} else {
			uw := csv.NewWriter(uf)
			uw.Write([]string{"title", "title_op"})
			for _, e := range dedupedUnmatched {
				uw.Write([]string{e.title, e.titleOP})
			}
			uw.Flush()
			uf.Close()
			fmt.Fprintf(os.Stderr, "Wrote %d unmatched titles to %s\n", len(dedupedUnmatched), *unmatchedFile)
		}
	}
}
