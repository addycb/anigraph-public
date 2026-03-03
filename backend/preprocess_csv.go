package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Input file names (originals - never modified)
const (
	MEDIA_CSV     = "media_delta.csv"
	STAFF_CSV     = "staff_delta.csv"
	EDGES_CSV     = "media_staff_edges_delta.csv"
	RELATIONS_CSV = "media_relations_delta.csv"
)

// Output file names (processed versions for Neo4j to use)
const (
	// Processed versions of main files
	MEDIA_PROCESSED_CSV     = "media_delta_processed.csv"
	STAFF_PROCESSED_CSV     = "staff_delta_processed.csv"
	RELATIONS_PROCESSED_CSV = "media_relations_delta_processed.csv"

	// Extracted entities
	GENRES_CSV        = "genres_delta.csv"
	TAGS_CSV          = "tags_delta.csv"
	STUDIOS_CSV       = "studios_delta.csv"
	ANIME_GENRES_CSV  = "anime_genres_delta.csv"
	ANIME_TAGS_CSV    = "anime_tags_delta.csv"
	ANIME_STUDIOS_CSV = "anime_studios_delta.csv"

	// Pre-aggregated staff edges (roles combined per anime-staff pair)
	STAFF_EDGES_AGGREGATED_CSV = "staff_edges_aggregated_delta.csv"

	// Crossover anime (for franchise detection - anime with "Crossover" tag)
	ANIME_CROSSOVER_CSV = "anime_crossover_delta.csv"

	// Cumulative tag files for recommendations (split by type)
	ANIME_TAGS_FULL_CSV = "anime_tags_full.csv" // anime_id, tag_name, rank
	MANGA_TAGS_FULL_CSV = "manga_tags_full.csv" // manga_id, tag_name, rank
)

// Column indices for media CSV (based on header order in scrape_incremental.go)
const (
	COL_ID              = 0
	COL_TYPE            = 4 // "ANIME" or "MANGA"
	COL_START_YEAR      = 10
	COL_START_MONTH     = 11
	COL_START_DAY       = 12
	COL_END_YEAR        = 13
	COL_END_MONTH       = 14
	COL_END_DAY         = 15
	MEDIA_EXPECTED_COLS = 44
)

// Stats for reporting
type PreprocessStats struct {
	MediaTotal      int
	MediaDatesFixed int
	StaffTotal      int
	EdgesTotal      int
	RelationsTotal  int
	// Entity extraction stats
	GenresExtracted       int
	TagsExtracted         int
	StudiosExtracted      int
	AnimeGenresExtracted  int
	AnimeTagsExtracted    int
	AnimeStudiosExtracted int
	StaffEdgesAggregated  int
	CrossoverAnimeCount   int // Anime with "Crossover" tag
	// Cumulative tag file stats
	AnimeTagsFullTotal    int // Total entries in anime_tags_full.csv
	MangaTagsFullTotal    int // Total entries in manga_tags_full.csv
	AnimeTagsFullNewMedia int // New anime added to cumulative file
	MangaTagsFullNewMedia int // New manga added to cumulative file
}

// Tag structure for JSON parsing
type Tag struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Rank     int    `json:"rank"`
}

// TagEntry for cumulative file: anime_id -> tag_name -> rank
type TagEntry struct {
	TagName string
	Rank    int
}

// sanitizeField removes newlines and problematic quotes from CSV fields
// APOC's CSV loader doesn't handle multi-line quoted fields correctly
func sanitizeField(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\"", "'")
	return s
}

// sanitizeRow applies sanitizeField to all fields in a CSV row
func sanitizeRow(row []string) []string {
	result := make([]string, len(row))
	for i, field := range row {
		result[i] = sanitizeField(field)
	}
	return result
}

// Load existing cumulative tags file into a map: anime_id -> []TagEntry
func loadCumulativeTags(filePath string) (map[int][]TagEntry, error) {
	result := make(map[int][]TagEntry)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return result, nil // File doesn't exist yet, return empty map
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	header := true
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if header {
			header = false
			continue
		}

		if len(record) < 3 {
			continue
		}

		animeID, err := strconv.Atoi(record[0])
		if err != nil {
			continue
		}

		tagName := record[1]
		rank, err := strconv.Atoi(record[2])
		if err != nil {
			rank = 50
		}

		result[animeID] = append(result[animeID], TagEntry{TagName: tagName, Rank: rank})
	}

	return result, nil
}

// Write cumulative tags to file
func writeCumulativeTags(filePath string, tagsMap map[int][]TagEntry) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"anime_id", "tag_name", "rank"})

	// Sort anime IDs for consistent output
	var animeIDs []int
	for id := range tagsMap {
		animeIDs = append(animeIDs, id)
	}
	sort.Ints(animeIDs)

	totalTags := 0
	for _, animeID := range animeIDs {
		for _, tag := range tagsMap[animeID] {
			writer.Write([]string{
				strconv.Itoa(animeID),
				tag.TagName,
				strconv.Itoa(tag.Rank),
			})
			totalTags++
		}
	}

	writer.Flush()
	return writer.Error()
}

// Check if a year is a leap year
func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// Get max days in a month
func maxDaysInMonth(year, month int) int {
	switch month {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if isLeapYear(year) {
			return 29
		}
		return 28
	default:
		return 31
	}
}

// Clamp a day to valid range for the given year/month
func clampDay(year, month, day int) int {
	if day < 1 {
		return 1
	}
	maxDay := maxDaysInMonth(year, month)
	if day > maxDay {
		return maxDay
	}
	return day
}

// Clamp a month to valid range (1-12)
func clampMonth(month int) int {
	if month < 1 {
		return 1
	}
	if month > 12 {
		return 12
	}
	return month
}

// Parse an integer from string, return 0 if empty/invalid
func parseIntOrZero(s string) int {
	if s == "" {
		return 0
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// Fix date fields in a media row, returns true if any fixes were made
func fixMediaDates(row []string) bool {
	fixed := false

	// Fix start date
	startYear := parseIntOrZero(row[COL_START_YEAR])
	startMonth := parseIntOrZero(row[COL_START_MONTH])
	startDay := parseIntOrZero(row[COL_START_DAY])

	// Fix invalid start month (must be 1-12)
	if startMonth > 0 {
		clampedMonth := clampMonth(startMonth)
		if clampedMonth != startMonth {
			row[COL_START_MONTH] = strconv.Itoa(clampedMonth)
			startMonth = clampedMonth // Use clamped value for day validation
			fixed = true
		}
	}

	if startYear > 0 && startMonth > 0 && startDay > 0 {
		clampedDay := clampDay(startYear, startMonth, startDay)
		if clampedDay != startDay {
			row[COL_START_DAY] = strconv.Itoa(clampedDay)
			fixed = true
		}
	}

	// Fix end date
	endYear := parseIntOrZero(row[COL_END_YEAR])
	endMonth := parseIntOrZero(row[COL_END_MONTH])
	endDay := parseIntOrZero(row[COL_END_DAY])

	// Fix invalid end month (must be 1-12)
	if endMonth > 0 {
		clampedMonth := clampMonth(endMonth)
		if clampedMonth != endMonth {
			row[COL_END_MONTH] = strconv.Itoa(clampedMonth)
			endMonth = clampedMonth // Use clamped value for day validation
			fixed = true
		}
	}

	if endYear > 0 && endMonth > 0 && endDay > 0 {
		clampedDay := clampDay(endYear, endMonth, endDay)
		if clampedDay != endDay {
			row[COL_END_DAY] = strconv.Itoa(clampedDay)
			fixed = true
		}
	}

	return fixed
}

// Copy a file to a new destination
func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// Find column index by header name
func findColumnIndex(header []string, name string) int {
	for i, h := range header {
		if h == name {
			return i
		}
	}
	return -1
}

// Process media CSV: fix dates, extract entities, write to _processed file
// Original file is never modified
func processMediaCSV(dir string, stats *PreprocessStats) error {
	filePath := filepath.Join(dir, MEDIA_CSV)
	outputPath := filepath.Join(dir, MEDIA_PROCESSED_CSV)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("   Skipping %s (not found)\n", MEDIA_CSV)
		return nil
	}

	// Read all rows from original
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Variable field count
	reader.LazyQuotes = true    // Handle malformed quotes

	var header []string
	var rows [][]string

	// Entity maps for deduplication
	genreSet := make(map[string]bool)
	tagMap := make(map[string]string)    // name -> category
	studioMap := make(map[string]string) // studio_id -> name

	// Junction data
	var animeGenres [][]string  // anime_id, genre_name
	var animeTags [][]string    // anime_id, tag_name, rank
	var animeStudios [][]string // anime_id, studio_id, is_main

	// Crossover anime (have "Crossover" tag)
	var crossoverAnime []string // anime_id

	// Delta tags by type (for cumulative file updates)
	deltaAnimeTags := make(map[int][]TagEntry) // anime_id -> tags (ANIME type)
	deltaMangaTags := make(map[int][]TagEntry) // manga_id -> tags (MANGA type)

	// Column indices (will be set after reading header)
	var colGenres, colTags, colStudios int

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			file.Close()
			return err
		}

		if header == nil {
			header = record
			colGenres = findColumnIndex(header, "genres")
			colTags = findColumnIndex(header, "tags")
			colStudios = findColumnIndex(header, "studios")
			continue
		}

		animeID := record[COL_ID]
		animeIDInt, _ := strconv.Atoi(animeID)

		// Get media type (ANIME or MANGA)
		mediaType := ""
		if COL_TYPE < len(record) {
			mediaType = strings.TrimSpace(record[COL_TYPE])
		}

		// Ensure row has enough columns
		if len(record) >= MEDIA_EXPECTED_COLS {
			if fixMediaDates(record) {
				stats.MediaDatesFixed++
			}
		}

		// Track if this anime is a crossover (Crossover is a tag on AniList)
		isCrossover := false

		// Extract genres (pipe-delimited)
		if colGenres >= 0 && colGenres < len(record) && record[colGenres] != "" {
			genres := strings.Split(record[colGenres], "|")
			for _, g := range genres {
				g = strings.TrimSpace(g)
				if g == "" {
					continue
				}
				genreSet[g] = true
				animeGenres = append(animeGenres, []string{animeID, g})
			}
		}

		// Extract tags (JSON array)
		if colTags >= 0 && colTags < len(record) && record[colTags] != "" && record[colTags] != "[]" {
			var tags []Tag
			if err := json.Unmarshal([]byte(record[colTags]), &tags); err == nil {
				for _, t := range tags {
					if t.Name == "" {
						continue
					}
					tagMap[t.Name] = t.Category
					animeTags = append(animeTags, []string{animeID, t.Name, strconv.Itoa(t.Rank)})

					// Track for cumulative files by type
					if mediaType == "ANIME" {
						deltaAnimeTags[animeIDInt] = append(deltaAnimeTags[animeIDInt], TagEntry{TagName: t.Name, Rank: t.Rank})
					} else if mediaType == "MANGA" {
						deltaMangaTags[animeIDInt] = append(deltaMangaTags[animeIDInt], TagEntry{TagName: t.Name, Rank: t.Rank})
					}

					// Check if this is a crossover anime (Crossover is a tag, not a genre)
					if strings.EqualFold(t.Name, "Crossover") {
						isCrossover = true
					}
				}
			}
		}

		// Add to crossover list if detected
		if isCrossover {
			crossoverAnime = append(crossoverAnime, animeID)
		}

		// Extract studios (format: id:name:is_main|id:name:is_main)
		if colStudios >= 0 && colStudios < len(record) && record[colStudios] != "" {
			studios := strings.Split(record[colStudios], "|")
			for _, s := range studios {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				parts := strings.Split(s, ":")
				if len(parts) >= 3 {
					studioID := parts[0]
					studioName := parts[1]
					isMain := parts[2]
					studioMap[studioID] = studioName
					animeStudios = append(animeStudios, []string{animeID, studioID, isMain})
				}
			}
		}

		rows = append(rows, sanitizeRow(record))
		stats.MediaTotal++
	}
	file.Close()

	// Write processed media data to new file (original untouched)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	writer.Write(header)
	for _, row := range rows {
		writer.Write(row)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	fmt.Printf("   ✅ Wrote %s\n", MEDIA_PROCESSED_CSV)

	// Write extracted entity files
	fmt.Println("\n   Extracting entities...")

	// Genres
	if len(genreSet) > 0 {
		genresFile, err := os.Create(filepath.Join(dir, GENRES_CSV))
		if err != nil {
			return fmt.Errorf("failed to create genres file: %w", err)
		}
		gw := csv.NewWriter(genresFile)
		gw.Write([]string{"name"})
		for name := range genreSet {
			gw.Write([]string{name})
			stats.GenresExtracted++
		}
		gw.Flush()
		genresFile.Close()
		fmt.Printf("   ✅ Extracted %d unique genres\n", stats.GenresExtracted)
	}

	// Tags
	if len(tagMap) > 0 {
		tagsFile, err := os.Create(filepath.Join(dir, TAGS_CSV))
		if err != nil {
			return fmt.Errorf("failed to create tags file: %w", err)
		}
		tw := csv.NewWriter(tagsFile)
		tw.Write([]string{"name", "category"})
		for name, category := range tagMap {
			tw.Write([]string{name, category})
			stats.TagsExtracted++
		}
		tw.Flush()
		tagsFile.Close()
		fmt.Printf("   ✅ Extracted %d unique tags\n", stats.TagsExtracted)
	}

	// Studios
	if len(studioMap) > 0 {
		studiosFile, err := os.Create(filepath.Join(dir, STUDIOS_CSV))
		if err != nil {
			return fmt.Errorf("failed to create studios file: %w", err)
		}
		sw := csv.NewWriter(studiosFile)
		sw.Write([]string{"studio_id", "name"})
		for id, name := range studioMap {
			sw.Write([]string{id, name})
			stats.StudiosExtracted++
		}
		sw.Flush()
		studiosFile.Close()
		fmt.Printf("   ✅ Extracted %d unique studios\n", stats.StudiosExtracted)
	}

	// Anime-Genre junctions
	if len(animeGenres) > 0 {
		agFile, err := os.Create(filepath.Join(dir, ANIME_GENRES_CSV))
		if err != nil {
			return fmt.Errorf("failed to create anime_genres file: %w", err)
		}
		agw := csv.NewWriter(agFile)
		agw.Write([]string{"anime_id", "genre_name"})
		for _, row := range animeGenres {
			agw.Write(row)
			stats.AnimeGenresExtracted++
		}
		agw.Flush()
		agFile.Close()
		fmt.Printf("   ✅ Extracted %d anime-genre relationships\n", stats.AnimeGenresExtracted)
	}

	// Anime-Tag junctions
	if len(animeTags) > 0 {
		atFile, err := os.Create(filepath.Join(dir, ANIME_TAGS_CSV))
		if err != nil {
			return fmt.Errorf("failed to create anime_tags file: %w", err)
		}
		atw := csv.NewWriter(atFile)
		atw.Write([]string{"anime_id", "tag_name", "rank"})
		for _, row := range animeTags {
			atw.Write(row)
			stats.AnimeTagsExtracted++
		}
		atw.Flush()
		atFile.Close()
		fmt.Printf("   ✅ Extracted %d anime-tag relationships\n", stats.AnimeTagsExtracted)
	}

	// Anime-Studio junctions
	if len(animeStudios) > 0 {
		asFile, err := os.Create(filepath.Join(dir, ANIME_STUDIOS_CSV))
		if err != nil {
			return fmt.Errorf("failed to create anime_studios file: %w", err)
		}
		asw := csv.NewWriter(asFile)
		asw.Write([]string{"anime_id", "studio_id", "is_main"})
		for _, row := range animeStudios {
			asw.Write(row)
			stats.AnimeStudiosExtracted++
		}
		asw.Flush()
		asFile.Close()
		fmt.Printf("   ✅ Extracted %d anime-studio relationships\n", stats.AnimeStudiosExtracted)
	}

	// Crossover anime (for franchise detection)
	// Always write the file, even if empty, so Neo4j doesn't error
	{
		crossoverFile, err := os.Create(filepath.Join(dir, ANIME_CROSSOVER_CSV))
		if err != nil {
			return fmt.Errorf("failed to create anime_crossover file: %w", err)
		}
		cw := csv.NewWriter(crossoverFile)
		cw.Write([]string{"anime_id"})
		for _, animeID := range crossoverAnime {
			cw.Write([]string{animeID})
			stats.CrossoverAnimeCount++
		}
		cw.Flush()
		crossoverFile.Close()
		fmt.Printf("   ✅ Identified %d crossover anime\n", stats.CrossoverAnimeCount)
	}

	// Update cumulative tag files (source of truth for recommendations)
	fmt.Println("\n   Updating cumulative tag files...")

	// Load existing cumulative files
	animeTagsFullPath := filepath.Join(dir, ANIME_TAGS_FULL_CSV)
	mangaTagsFullPath := filepath.Join(dir, MANGA_TAGS_FULL_CSV)

	existingAnimeTags, err := loadCumulativeTags(animeTagsFullPath)
	if err != nil {
		return fmt.Errorf("failed to load existing anime tags: %w", err)
	}
	existingMangaTags, err := loadCumulativeTags(mangaTagsFullPath)
	if err != nil {
		return fmt.Errorf("failed to load existing manga tags: %w", err)
	}

	existingAnimeCount := len(existingAnimeTags)
	existingMangaCount := len(existingMangaTags)

	// Merge delta into cumulative (delta entries override existing)
	for animeID, tags := range deltaAnimeTags {
		if _, exists := existingAnimeTags[animeID]; !exists {
			stats.AnimeTagsFullNewMedia++
		}
		existingAnimeTags[animeID] = tags
	}
	for mangaID, tags := range deltaMangaTags {
		if _, exists := existingMangaTags[mangaID]; !exists {
			stats.MangaTagsFullNewMedia++
		}
		existingMangaTags[mangaID] = tags
	}

	// Write updated cumulative files
	if err := writeCumulativeTags(animeTagsFullPath, existingAnimeTags); err != nil {
		return fmt.Errorf("failed to write anime tags full: %w", err)
	}
	// Count total tags
	for _, tags := range existingAnimeTags {
		stats.AnimeTagsFullTotal += len(tags)
	}
	fmt.Printf("   ✅ %s: %d media (%d existing + %d new), %d total tags\n",
		ANIME_TAGS_FULL_CSV, len(existingAnimeTags), existingAnimeCount, stats.AnimeTagsFullNewMedia, stats.AnimeTagsFullTotal)

	if err := writeCumulativeTags(mangaTagsFullPath, existingMangaTags); err != nil {
		return fmt.Errorf("failed to write manga tags full: %w", err)
	}
	for _, tags := range existingMangaTags {
		stats.MangaTagsFullTotal += len(tags)
	}
	fmt.Printf("   ✅ %s: %d media (%d existing + %d new), %d total tags\n",
		MANGA_TAGS_FULL_CSV, len(existingMangaTags), existingMangaCount, stats.MangaTagsFullNewMedia, stats.MangaTagsFullTotal)

	return nil
}

// Process staff edges CSV: aggregate roles per (anime, staff) pair
func processStaffEdges(dir string, stats *PreprocessStats) error {
	filePath := filepath.Join(dir, EDGES_CSV)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("   Skipping %s (not found)\n", EDGES_CSV)
		return nil
	}

	// Read all rows
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	var header []string
	var colMediaId, colStaffId, colRole int

	// Map to aggregate roles: "mediaId:staffId" -> []roles
	roleMap := make(map[string][]string)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header == nil {
			header = record
			colMediaId = findColumnIndex(header, "mediaId")
			colStaffId = findColumnIndex(header, "staffId")
			colRole = findColumnIndex(header, "role")
			if colMediaId < 0 || colStaffId < 0 || colRole < 0 {
				return fmt.Errorf("missing required columns in staff edges CSV")
			}
			continue
		}

		mediaId := record[colMediaId]
		staffId := record[colStaffId]
		role := record[colRole]

		// Remove quotes from role names to avoid PostgreSQL array parsing issues
		role = strings.ReplaceAll(role, "\"", "")

		key := mediaId + ":" + staffId
		if role != "" {
			roleMap[key] = append(roleMap[key], role)
		} else if _, exists := roleMap[key]; !exists {
			// Ensure key exists even if no role
			roleMap[key] = []string{}
		}
	}

	// Write aggregated file
	outPath := filepath.Join(dir, STAFF_EDGES_AGGREGATED_CSV)
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create aggregated edges file: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	writer.Write([]string{"media_id", "staff_id", "roles"})

	for key, roles := range roleMap {
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			continue
		}
		// Join roles with pipe delimiter for Neo4j to split
		rolesStr := strings.Join(roles, "|")
		writer.Write([]string{parts[0], parts[1], rolesStr})
		stats.StaffEdgesAggregated++
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	fmt.Printf("   ✅ Aggregated %d unique staff-anime relationships\n", stats.StaffEdgesAggregated)
	return nil
}

// Copy a CSV file to a processed version with sanitization (original untouched)
func copyToProcessed(dir, inputFilename, outputFilename string, countField *int) error {
	inputPath := filepath.Join(dir, inputFilename)
	outputPath := filepath.Join(dir, outputFilename)

	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Printf("   Skipping %s (not found)\n", inputFilename)
		return nil
	}

	// Read input file
	file, err := os.Open(inputPath)
	if err != nil {
		return err
	}

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		file.Close()
		return err
	}

	writer := csv.NewWriter(outFile)
	count := 0
	isHeader := true

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			file.Close()
			outFile.Close()
			return err
		}

		// Sanitize all fields (except header)
		if isHeader {
			writer.Write(record)
			isHeader = false
		} else {
			writer.Write(sanitizeRow(record))
			count++
		}
	}

	writer.Flush()
	file.Close()
	outFile.Close()

	if err := writer.Error(); err != nil {
		return err
	}

	*countField = count

	fmt.Printf("   ✅ Processed %s (%d rows)\n", outputFilename, count)
	return nil
}

// Main preprocessing function
func PreprocessCSVs(dir string) (*PreprocessStats, error) {
	stats := &PreprocessStats{}
	startTime := time.Now()

	fmt.Println("\n📦 CSV Preprocessing (with Entity Extraction)")
	fmt.Println("=" + string(make([]byte, 50)))
	fmt.Printf("   Directory: %s\n", dir)

	// Process media CSV (with date fixing and entity extraction)
	fmt.Println("\n   Processing media CSV...")
	if err := processMediaCSV(dir, stats); err != nil {
		return stats, fmt.Errorf("media CSV processing failed: %w", err)
	}
	fmt.Printf("   ✅ Media: %d rows, %d dates fixed\n", stats.MediaTotal, stats.MediaDatesFixed)

	// Copy and process other CSVs (originals never modified)
	fmt.Println("\n   Processing other CSVs...")

	if err := copyToProcessed(dir, STAFF_CSV, STAFF_PROCESSED_CSV, &stats.StaffTotal); err != nil {
		return stats, fmt.Errorf("staff CSV copy failed: %w", err)
	}
	fmt.Printf("      Staff: %d rows\n", stats.StaffTotal)

	// Count raw edges (we don't copy this one, just aggregate it)
	edgesPath := filepath.Join(dir, EDGES_CSV)
	if _, err := os.Stat(edgesPath); err == nil {
		file, _ := os.Open(edgesPath)
		reader := csv.NewReader(file)
		reader.FieldsPerRecord = -1
		count := 0
		for {
			_, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err == nil {
				count++
			}
		}
		file.Close()
		stats.EdgesTotal = count - 1
		fmt.Printf("      Edges (raw): %d rows\n", stats.EdgesTotal)
	}

	// Aggregate staff edges (combine roles per anime-staff pair)
	fmt.Println("\n   Aggregating staff edges...")
	if err := processStaffEdges(dir, stats); err != nil {
		return stats, fmt.Errorf("staff edges aggregation failed: %w", err)
	}

	if err := copyToProcessed(dir, RELATIONS_CSV, RELATIONS_PROCESSED_CSV, &stats.RelationsTotal); err != nil {
		return stats, fmt.Errorf("relations CSV copy failed: %w", err)
	}
	fmt.Printf("      Relations: %d rows\n", stats.RelationsTotal)

	duration := time.Since(startTime)
	fmt.Printf("\n   Preprocessing complete in %v\n", duration)
	fmt.Println("=" + string(make([]byte, 50)))

	return stats, nil
}

func main() {
	dir := flag.String("dir", ".", "Directory containing CSV files")
	flag.Parse()

	stats, err := PreprocessCSVs(*dir)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Media rows: %d (dates fixed: %d)\n", stats.MediaTotal, stats.MediaDatesFixed)
	fmt.Printf("  Staff rows: %d\n", stats.StaffTotal)
	fmt.Printf("  Edge rows: %d (raw), %d (aggregated)\n", stats.EdgesTotal, stats.StaffEdgesAggregated)
	fmt.Printf("  Relation rows: %d\n", stats.RelationsTotal)
	fmt.Printf("  Entities extracted:\n")
	fmt.Printf("    Genres: %d unique, %d relationships\n", stats.GenresExtracted, stats.AnimeGenresExtracted)
	fmt.Printf("    Tags: %d unique, %d relationships\n", stats.TagsExtracted, stats.AnimeTagsExtracted)
	fmt.Printf("    Studios: %d unique, %d relationships\n", stats.StudiosExtracted, stats.AnimeStudiosExtracted)
	fmt.Printf("    Crossover anime: %d\n", stats.CrossoverAnimeCount)
	fmt.Printf("  Cumulative tag files:\n")
	fmt.Printf("    Anime: %d tags (%d new media added)\n", stats.AnimeTagsFullTotal, stats.AnimeTagsFullNewMedia)
	fmt.Printf("    Manga: %d tags (%d new media added)\n", stats.MangaTagsFullTotal, stats.MangaTagsFullNewMedia)
}
