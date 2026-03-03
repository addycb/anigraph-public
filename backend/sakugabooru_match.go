package main

// sakugabooru_match.go
//
// Combined Sakugabooru tag download + anime/staff matching (single binary).
//
// 1. Downloads all tags from Sakugabooru API (or reads from cached JSON)
// 2. Reads anime CSV + staff CSV from input files
// 3. Matches copyright tags → anime rows, artist tags → staff rows
// 4. Writes output CSVs: anime_matches.csv, staff_matches.csv
//
// Usage:
//
//	./sakugabooru_match \
//	  -anime input_anime.csv \
//	  -staff input_staff.csv \
//	  -out ./output
//
// Input anime CSV columns: anilist_id, title_english, title_romaji
// Input staff CSV columns:  staff_id, name_en, name_ja
//
// Output anime_matches.csv: anilist_id, title_english, title_romaji, sakugabooru_tag, post_count, found, method
// Output staff_matches.csv: staff_id, name_en, sakugabooru_tag, post_count, tag_type, found, method

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	matchBase  = "https://www.sakugabooru.com"
	matchAgent = "AniGraph-SakugabooruMatch/1.0 (anigraph.xyz)"
)

// ---------------------------------------------------------------------------
// Tag structures
// ---------------------------------------------------------------------------

type tagEntry struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
	Type      int    `json:"type"`
	Ambiguous bool   `json:"ambiguous"`
}

var tagTypeNames = map[int]string{
	0: "general",
	1: "artist",
	3: "copyright",
	4: "terminology",
	5: "meta",
}

// ---------------------------------------------------------------------------
// Tag download
// ---------------------------------------------------------------------------

func downloadTags() ([]tagEntry, error) {
	url := matchBase + "/tag.json?limit=0"
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", matchAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var tags []tagEntry
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func loadTagsFromFile(path string) ([]tagEntry, []tagEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var categorized map[string][]tagEntry
	if err := json.NewDecoder(f).Decode(&categorized); err != nil {
		return nil, nil, err
	}
	return categorized["copyright"], categorized["artist"], nil
}

func categorizeTags(tags []tagEntry) (copyright, artist []tagEntry) {
	for _, t := range tags {
		switch t.Type {
		case 3:
			copyright = append(copyright, t)
		case 1:
			artist = append(artist, t)
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Title normalization (for anime matching)
// ---------------------------------------------------------------------------

func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
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
	result := b.String()
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	return strings.Trim(result, "_")
}

// ---------------------------------------------------------------------------
// Name normalization (for staff matching)
// ---------------------------------------------------------------------------

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

func forward(nameEn string) string {
	return normalize(nameEn)
}

// ---------------------------------------------------------------------------
// Data structures
// ---------------------------------------------------------------------------

type animeRow struct {
	anilistID    string
	titleEnglish string
	titleRomaji  string
	synonyms     []string
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

type staffRow struct {
	staffID          string
	nameEn           string
	nameJa           string
	alternativeNames []string
}

type staffResult struct {
	staffID   string
	nameEn    string
	sakugaTag string
	postCount int
	tagType   int
	found     bool
	method    string
}

// ---------------------------------------------------------------------------
// CSV loading
// ---------------------------------------------------------------------------

func loadAnimeCSV(filename string) ([]animeRow, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(bufio.NewReader(f))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var all []animeRow
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "anime CSV error: %v\n", err)
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
	return all, nil
}

func loadStaffCSV(filename string) ([]staffRow, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(bufio.NewReader(f))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	var all []staffRow
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "staff CSV error: %v\n", err)
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
	return all, nil
}

// ---------------------------------------------------------------------------
// Anime matching: copyright tags → anime rows
// ---------------------------------------------------------------------------

func matchAnime(copyrightTags []tagEntry, all []animeRow) (map[string]animeResult, int, []tagEntry) {
	// Track which tags got matched
	matchedTags := make(map[string]bool)
	// Build lookup indexes
	romajiIndex := make(map[string][]int)
	englishIndex := make(map[string][]int)
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

	results := make(map[string]animeResult, len(all))
	ambiguous := 0

	for _, tag := range copyrightTags {
		tagName := tag.Name

		// Exact match: romaji
		if indices, ok := romajiIndex[tagName]; ok {
			matchedTags[tagName] = true
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

		// Exact match: english
		if indices, ok := englishIndex[tagName]; ok {
			matchedTags[tagName] = true
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

		// Exact match: synonym
		if indices, ok := synonymIndex[tagName]; ok {
			matchedTags[tagName] = true
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

	// Collect unmatched copyright tags
	var unmatchedCopyright []tagEntry
	for _, tag := range copyrightTags {
		if !matchedTags[tag.Name] {
			unmatchedCopyright = append(unmatchedCopyright, tag)
		}
	}

	// Fill in unmatched anime
	for _, a := range all {
		if _, ok := results[a.anilistID]; !ok {
			method := "not_found"
			if a.titleRomaji == "" && a.titleEnglish == "" {
				method = "no_title"
			}
			results[a.anilistID] = animeResult{
				anilistID: a.anilistID, titleEnglish: a.titleEnglish, titleRomaji: a.titleRomaji,
				method: method,
			}
		}
	}

	return results, ambiguous, unmatchedCopyright
}

// ---------------------------------------------------------------------------
// Staff matching: artist tags → staff rows
// ---------------------------------------------------------------------------

func matchStaff(artistTags []tagEntry, all []staffRow) (map[string]staffResult, int, []tagEntry) {
	matchedTags := make(map[string]bool)

	// Build lookup indexes
	reversedIndex := make(map[string][]int)
	forwardIndex := make(map[string][]int)

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

	results := make(map[string]staffResult, len(all))
	ambiguous := 0

	for _, tag := range artistTags {
		tagName := tag.Name

		// Exact match: reversed
		if indices, ok := reversedIndex[tagName]; ok {
			matchedTags[tagName] = true
			for _, idx := range indices {
				s := all[idx]
				prev, exists := results[s.staffID]
				if !exists || !prev.found || tag.Count > prev.postCount {
					results[s.staffID] = staffResult{
						staffID: s.staffID, nameEn: s.nameEn,
						sakugaTag: tagName, postCount: tag.Count, tagType: tag.Type,
						found: true, method: "exact_reversed",
					}
				}
			}
			continue
		}

		// Exact match: forward
		if indices, ok := forwardIndex[tagName]; ok {
			matchedTags[tagName] = true
			for _, idx := range indices {
				s := all[idx]
				prev, exists := results[s.staffID]
				if !exists || !prev.found || tag.Count > prev.postCount {
					results[s.staffID] = staffResult{
						staffID: s.staffID, nameEn: s.nameEn,
						sakugaTag: tagName, postCount: tag.Count, tagType: tag.Type,
						found: true, method: "exact_forward",
					}
				}
			}
			continue
		}

	}

	// Collect unmatched artist tags
	var unmatchedArtist []tagEntry
	for _, tag := range artistTags {
		if !matchedTags[tag.Name] {
			unmatchedArtist = append(unmatchedArtist, tag)
		}
	}

	// Fill in unmatched staff
	for _, s := range all {
		if _, ok := results[s.staffID]; !ok {
			method := "not_found"
			if strings.TrimSpace(s.nameEn) == "" {
				method = "no_name"
			}
			results[s.staffID] = staffResult{
				staffID: s.staffID, nameEn: s.nameEn,
				method: method,
			}
		}
	}

	return results, ambiguous, unmatchedArtist
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	animeFile := flag.String("anime", "", "Input anime CSV (anilist_id, title_english, title_romaji)")
	staffFile := flag.String("staff", "", "Input staff CSV (staff_id, name_en, name_ja)")
	tagsCache := flag.String("tags", "", "Cached sakugabooru_tags.json (skip download if provided and exists)")
	outDir := flag.String("out", ".", "Output directory for match CSVs")
	flag.Parse()

	if *animeFile == "" && *staffFile == "" {
		fmt.Fprintln(os.Stderr, "usage: sakugabooru_match -anime <file> -staff <file> [-tags <cache>] [-out <dir>]")
		os.Exit(1)
	}

	// ---- Step 1: Get tags (download or load from cache) ----
	var copyrightTags, artistTags []tagEntry

	if *tagsCache != "" {
		if _, err := os.Stat(*tagsCache); err == nil {
			fmt.Fprintf(os.Stderr, "Loading cached tags from %s...\n", *tagsCache)
			var err2 error
			copyrightTags, artistTags, err2 = loadTagsFromFile(*tagsCache)
			if err2 != nil {
				fmt.Fprintf(os.Stderr, "Error loading cached tags: %v, will download fresh\n", err2)
				copyrightTags, artistTags = nil, nil
			}
		}
	}

	if copyrightTags == nil && artistTags == nil {
		fmt.Fprintln(os.Stderr, "Downloading all tags from Sakugabooru...")
		allTags, err := downloadTags()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error downloading tags: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Downloaded %d total tags\n", len(allTags))
		copyrightTags, artistTags = categorizeTags(allTags)

		// Save cache for future runs
		if *tagsCache != "" {
			saveTagsCache(*tagsCache, allTags)
		}
	}

	fmt.Fprintf(os.Stderr, "Tags: %d copyright, %d artist\n", len(copyrightTags), len(artistTags))

	if err := os.MkdirAll(*outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create output dir: %v\n", err)
		os.Exit(1)
	}

	// ---- Step 2: Match anime ----
	if *animeFile != "" {
		animeRows, err := loadAnimeCSV(*animeFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading anime CSV: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Loaded %d anime rows\n", len(animeRows))

		animeResults, animeAmbig, unmatchedCopyright := matchAnime(copyrightTags, animeRows)

		// Count stats
		var found, notFound, noTitle int
		for _, a := range animeRows {
			res := animeResults[a.anilistID]
			switch {
			case res.method == "no_title":
				noTitle++
			case !res.found:
				notFound++
			default:
				found++
			}
		}
		fmt.Fprintf(os.Stderr, "Anime: %d found (%d ambiguous), %d not found, %d no title, %d unmatched copyright tags\n",
			found, animeAmbig, notFound, noTitle, len(unmatchedCopyright))

		// Write anime_matches.csv
		outPath := filepath.Join(*outDir, "anime_matches.csv")
		f, err := os.Create(outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot create %s: %v\n", outPath, err)
			os.Exit(1)
		}
		w := csv.NewWriter(f)
		w.Write([]string{"anilist_id", "title_english", "title_romaji", "sakugabooru_tag", "post_count", "found", "method"})
		for _, a := range animeRows {
			res := animeResults[a.anilistID]
			foundStr := "0"
			if res.found {
				foundStr = "1"
			}
			w.Write([]string{
				a.anilistID, a.titleEnglish, a.titleRomaji,
				res.sakugaTag, fmt.Sprintf("%d", res.postCount),
				foundStr, res.method,
			})
		}
		w.Flush()
		f.Close()
		fmt.Fprintf(os.Stderr, "Wrote %s\n", outPath)

		// Write unmatched_copyright_tags.csv
		unmatchedPath := filepath.Join(*outDir, "unmatched_copyright_tags.csv")
		uf, err := os.Create(unmatchedPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot create %s: %v\n", unmatchedPath, err)
		} else {
			uw := csv.NewWriter(uf)
			uw.Write([]string{"tag_name", "post_count", "tag_id"})
			for _, t := range unmatchedCopyright {
				uw.Write([]string{t.Name, fmt.Sprintf("%d", t.Count), fmt.Sprintf("%d", t.ID)})
			}
			uw.Flush()
			uf.Close()
			fmt.Fprintf(os.Stderr, "Wrote %s (%d tags)\n", unmatchedPath, len(unmatchedCopyright))
		}
	}

	// ---- Step 3: Match staff ----
	if *staffFile != "" {
		staffRows, err := loadStaffCSV(*staffFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading staff CSV: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Loaded %d staff rows\n", len(staffRows))

		staffResults, staffAmbig, unmatchedArtist := matchStaff(artistTags, staffRows)

		// Count stats
		var found, notFound, noName int
		for _, s := range staffRows {
			res := staffResults[s.staffID]
			switch {
			case res.method == "no_name":
				noName++
			case !res.found:
				notFound++
			default:
				found++
			}
		}
		fmt.Fprintf(os.Stderr, "Staff: %d found (%d ambiguous), %d not found, %d no name, %d unmatched artist tags\n",
			found, staffAmbig, notFound, noName, len(unmatchedArtist))

		// Write staff_matches.csv
		outPath := filepath.Join(*outDir, "staff_matches.csv")
		f, err := os.Create(outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot create %s: %v\n", outPath, err)
			os.Exit(1)
		}
		w := csv.NewWriter(f)
		w.Write([]string{"staff_id", "name_en", "sakugabooru_tag", "post_count", "tag_type", "found", "method"})
		for _, s := range staffRows {
			res := staffResults[s.staffID]
			foundStr := "0"
			if res.found {
				foundStr = "1"
			}
			w.Write([]string{
				s.staffID, s.nameEn,
				res.sakugaTag, fmt.Sprintf("%d", res.postCount),
				fmt.Sprintf("%d", res.tagType),
				foundStr, res.method,
			})
		}
		w.Flush()
		f.Close()
		fmt.Fprintf(os.Stderr, "Wrote %s\n", outPath)

		// Write unmatched_artist_tags.csv
		unmatchedPath := filepath.Join(*outDir, "unmatched_artist_tags.csv")
		uf, err := os.Create(unmatchedPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot create %s: %v\n", unmatchedPath, err)
		} else {
			uw := csv.NewWriter(uf)
			uw.Write([]string{"tag_name", "post_count", "tag_id"})
			for _, t := range unmatchedArtist {
				uw.Write([]string{t.Name, fmt.Sprintf("%d", t.Count), fmt.Sprintf("%d", t.ID)})
			}
			uw.Flush()
			uf.Close()
			fmt.Fprintf(os.Stderr, "Wrote %s (%d tags)\n", unmatchedPath, len(unmatchedArtist))
		}
	}

	fmt.Fprintln(os.Stderr, "Done.")
}

func saveTagsCache(path string, allTags []tagEntry) {
	categorized := make(map[string][]tagEntry)
	for _, name := range tagTypeNames {
		categorized[name] = []tagEntry{}
	}
	for _, t := range allTags {
		name, ok := tagTypeNames[t.Type]
		if !ok {
			name = fmt.Sprintf("unknown_%d", t.Type)
			if categorized[name] == nil {
				categorized[name] = []tagEntry{}
			}
		}
		categorized[name] = append(categorized[name], t)
	}

	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not cache tags to %s: %v\n", path, err)
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(categorized); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error writing tags cache: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Cached tags to %s\n", path)
	}
}
