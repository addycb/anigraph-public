package main

import (
	"bufio"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Helper to log memory usage
func logMemoryUsage(label string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("   [MEM %s] Alloc: %d MB, Sys: %d MB, NumGC: %d\n",
		label, m.Alloc/1024/1024, m.Sys/1024/1024, m.NumGC)
}

// =====================================================
// CONSTANTS
// =====================================================

const (
	TOP_N           = 200  // Number of recommendations per anime
	BINARY_VERSION  = 1    // Binary file format version
	MIN_SHARED_TAGS = 1    // Minimum shared tags to consider similarity
)

// Column indices for media CSV
const (
	MEDIA_COL_ID   = 0
	MEDIA_COL_TYPE = 4
	MEDIA_COL_TAGS = 33
)

// =====================================================
// DATA STRUCTURES
// =====================================================

// Tag represents a tag with its weight (rank)
type TagWeight struct {
	Name   string  `json:"name"`
	Rank   int     `json:"rank"`
	Weight float32 // Computed: rank / 100.0
}

// AnimeTagData holds an anime's tags with weights
type AnimeTagData struct {
	ID      int32
	Type    string // "ANIME" or "MANGA"
	Tags    map[string]float32 // tag_name -> weight
	TagSum  float32            // Sum of all weights (for weighted union)
}

// Recommendation represents a single recommendation
type Recommendation struct {
	TargetID   int32
	Similarity float32
}

// AnimeRecs holds all recommendations for one anime
type AnimeRecs struct {
	SourceID   int32
	Threshold  float32           // 200th similarity (for incremental updates)
	Recs       []Recommendation  // Sorted by similarity descending
}

// InvertedIndex maps tag -> list of (anime_id, weight)
type TagEntry struct {
	AnimeID int32
	Weight  float32
}

type InvertedIndex struct {
	Index map[string][]TagEntry
	mu    sync.RWMutex
}

// BinaryHeader for the recommendations file
type BinaryHeader struct {
	Version   uint32
	NumAnime  uint32
	Reserved1 uint32
	Reserved2 uint32
}

// ThresholdEntry for quick lookup
type ThresholdEntry struct {
	AnimeID   int32
	Threshold float32
	Offset    uint32 // Byte offset to recommendations in file
}

// =====================================================
// INVERTED INDEX OPERATIONS
// =====================================================

func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		Index: make(map[string][]TagEntry),
	}
}

func (idx *InvertedIndex) Add(tag string, animeID int32, weight float32) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.Index[tag] = append(idx.Index[tag], TagEntry{AnimeID: animeID, Weight: weight})
}

func (idx *InvertedIndex) Get(tag string) []TagEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.Index[tag]
}

// =====================================================
// WEIGHTED JACCARD SIMILARITY
// =====================================================

// Compute weighted Jaccard similarity between two anime
// weightedJaccard = sum(min(w1, w2)) / sum(max(w1, w2))
func computeWeightedJaccard(tags1, tags2 map[string]float32, sum1, sum2 float32) float32 {
	if len(tags1) == 0 || len(tags2) == 0 {
		return 0
	}

	var intersection float32 = 0
	var union float32 = 0

	// Process tags from anime1
	for tag, w1 := range tags1 {
		if w2, exists := tags2[tag]; exists {
			// Tag in both: intersection = min, union = max
			if w1 < w2 {
				intersection += w1
				union += w2
			} else {
				intersection += w2
				union += w1
			}
		} else {
			// Tag only in anime1
			union += w1
		}
	}

	// Process tags only in anime2
	for tag, w2 := range tags2 {
		if _, exists := tags1[tag]; !exists {
			union += w2
		}
	}

	if union == 0 {
		return 0
	}

	return intersection / union
}

// =====================================================
// FILE I/O
// =====================================================

// Load anime data from media CSV (for type info) and anime_tags CSV (for tags)
func loadAnimeData(mediaPath, tagsPath string) (map[int32]*AnimeTagData, error) {
	animeData := make(map[int32]*AnimeTagData)

	// First, load type info from media CSV
	fmt.Println("   Loading media types...")
	mediaFile, err := os.Open(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open media file: %w", err)
	}
	defer mediaFile.Close()

	reader := csv.NewReader(mediaFile)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header := true
	mediaCount := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip malformed rows
		}
		if header {
			header = false
			continue
		}

		if len(record) <= MEDIA_COL_TYPE {
			continue
		}

		id, err := strconv.Atoi(record[MEDIA_COL_ID])
		if err != nil {
			continue
		}

		mediaType := strings.TrimSpace(record[MEDIA_COL_TYPE])
		if mediaType != "ANIME" && mediaType != "MANGA" {
			continue
		}

		animeData[int32(id)] = &AnimeTagData{
			ID:   int32(id),
			Type: mediaType,
			Tags: make(map[string]float32),
		}
		mediaCount++
	}
	fmt.Printf("   Loaded %d media entries\n", mediaCount)

	// Now load tags from anime_tags CSV
	fmt.Println("   Loading tags...")
	tagsFile, err := os.Open(tagsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tags file: %w", err)
	}
	defer tagsFile.Close()

	tagsReader := csv.NewReader(tagsFile)
	tagsReader.FieldsPerRecord = -1

	header = true
	tagCount := 0
	for {
		record, err := tagsReader.Read()
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
			rank = 50 // Default weight
		}

		anime, exists := animeData[int32(animeID)]
		if !exists {
			continue
		}

		weight := float32(rank) / 100.0
		anime.Tags[tagName] = weight
		anime.TagSum += weight
		tagCount++
	}
	fmt.Printf("   Loaded %d tag associations\n", tagCount)

	return animeData, nil
}

// Load anime data directly from media CSV (extracts tags from JSON column)
func loadAnimeDataFromMedia(mediaPath string) (map[int32]*AnimeTagData, error) {
	animeData := make(map[int32]*AnimeTagData)

	fmt.Printf("   Loading media with embedded tags from %s...\n", filepath.Base(mediaPath))
	mediaFile, err := os.Open(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open media file: %w", err)
	}
	defer mediaFile.Close()

	reader := csv.NewReader(mediaFile)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header := true
	mediaCount := 0
	tagCount := 0

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

		if len(record) <= MEDIA_COL_TAGS {
			continue
		}

		id, err := strconv.Atoi(record[MEDIA_COL_ID])
		if err != nil {
			continue
		}

		mediaType := strings.TrimSpace(record[MEDIA_COL_TYPE])
		if mediaType != "ANIME" && mediaType != "MANGA" {
			continue
		}

		anime := &AnimeTagData{
			ID:   int32(id),
			Type: mediaType,
			Tags: make(map[string]float32),
		}

		// Parse tags JSON
		tagsJSON := record[MEDIA_COL_TAGS]
		if tagsJSON != "" && tagsJSON != "[]" {
			var tags []struct {
				Name string `json:"name"`
				Rank int    `json:"rank"`
			}
			if err := json.Unmarshal([]byte(tagsJSON), &tags); err == nil {
				for _, t := range tags {
					if t.Name != "" {
						weight := float32(t.Rank) / 100.0
						anime.Tags[t.Name] = weight
						anime.TagSum += weight
						tagCount++
					}
				}
			}
		}

		if len(anime.Tags) > 0 {
			animeData[int32(id)] = anime
			mediaCount++
		}
	}

	fmt.Printf("   Loaded %d media entries with %d tag associations\n", mediaCount, tagCount)
	return animeData, nil
}

// Load anime data from multiple media CSVs (full + delta), merging them
// Delta entries override full entries if they exist
func loadAnimeDataFromMultipleMedia(paths []string) (map[int32]*AnimeTagData, error) {
	animeData := make(map[int32]*AnimeTagData)

	for _, mediaPath := range paths {
		if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
			continue
		}

		partialData, err := loadAnimeDataFromMedia(mediaPath)
		if err != nil {
			return nil, err
		}

		// Merge into main map (later files override earlier ones)
		for id, anime := range partialData {
			animeData[id] = anime
		}
	}

	return animeData, nil
}

// Load anime data from split cumulative tag files (anime_tags_full.csv and manga_tags_full.csv)
// Each file contains: anime_id, tag_name, rank
// The type (ANIME/MANGA) is determined by which file the data came from
func loadAnimeDataFromSplitFiles(animeTagsPath, mangaTagsPath string) (map[int32]*AnimeTagData, error) {
	animeData := make(map[int32]*AnimeTagData)

	// Load ANIME tags
	if _, err := os.Stat(animeTagsPath); err == nil {
		fmt.Printf("   Loading anime tags from %s...\n", filepath.Base(animeTagsPath))
		if err := loadTagsFromFile(animeTagsPath, "ANIME", animeData); err != nil {
			return nil, fmt.Errorf("failed to load anime tags: %w", err)
		}
	}

	// Load MANGA tags
	if _, err := os.Stat(mangaTagsPath); err == nil {
		fmt.Printf("   Loading manga tags from %s...\n", filepath.Base(mangaTagsPath))
		if err := loadTagsFromFile(mangaTagsPath, "MANGA", animeData); err != nil {
			return nil, fmt.Errorf("failed to load manga tags: %w", err)
		}
	}

	// Count stats
	animeCount := 0
	mangaCount := 0
	totalTags := 0
	for _, data := range animeData {
		if data.Type == "ANIME" {
			animeCount++
		} else {
			mangaCount++
		}
		totalTags += len(data.Tags)
	}
	fmt.Printf("   Loaded %d anime, %d manga (%d total tags)\n", animeCount, mangaCount, totalTags)

	return animeData, nil
}

// Helper to load tags from a cumulative file
func loadTagsFromFile(filePath, mediaType string, animeData map[int32]*AnimeTagData) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
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

		id, err := strconv.Atoi(record[0])
		if err != nil {
			continue
		}

		tagName := record[1]
		rank, err := strconv.Atoi(record[2])
		if err != nil {
			rank = 50
		}

		animeID := int32(id)
		anime, exists := animeData[animeID]
		if !exists {
			anime = &AnimeTagData{
				ID:   animeID,
				Type: mediaType,
				Tags: make(map[string]float32),
			}
			animeData[animeID] = anime
		}

		weight := float32(rank) / 100.0
		anime.Tags[tagName] = weight
		anime.TagSum += weight
	}

	return nil
}

// Get new anime IDs from delta file (for incremental mode)
func getNewAnimeIDsFromDelta(deltaPath string) (map[int32]bool, error) {
	newIDs := make(map[int32]bool)

	if _, err := os.Stat(deltaPath); os.IsNotExist(err) {
		return newIDs, nil
	}

	file, err := os.Open(deltaPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

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

		if len(record) < 1 {
			continue
		}

		id, err := strconv.Atoi(record[0])
		if err != nil {
			continue
		}

		newIDs[int32(id)] = true
	}

	return newIDs, nil
}

// Build inverted indices (one per type)
func buildInvertedIndices(animeData map[int32]*AnimeTagData) map[string]*InvertedIndex {
	indices := map[string]*InvertedIndex{
		"ANIME": NewInvertedIndex(),
		"MANGA": NewInvertedIndex(),
	}

	for _, anime := range animeData {
		idx := indices[anime.Type]
		if idx == nil {
			continue
		}
		for tag, weight := range anime.Tags {
			idx.Add(tag, anime.ID, weight)
		}
	}

	return indices
}

// =====================================================
// RECOMMENDATION COMPUTATION
// =====================================================

// Pool for reusing candidate maps to reduce allocations
var candidatePool = sync.Pool{
	New: func() interface{} {
		return make(map[int32]struct{}, 1000)
	},
}

// Pool for reusing similarity slices
var similarityPool = sync.Pool{
	New: func() interface{} {
		s := make([]Recommendation, 0, TOP_N*2)
		return &s
	},
}

// Compute top-N recommendations for a single anime
func computeRecsForAnime(
	anime *AnimeTagData,
	animeData map[int32]*AnimeTagData,
	index *InvertedIndex,
) *AnimeRecs {
	if anime == nil {
		fmt.Println("      ERROR: computeRecsForAnime called with nil anime")
		return &AnimeRecs{SourceID: 0, Recs: nil}
	}
	if len(anime.Tags) == 0 {
		return &AnimeRecs{SourceID: anime.ID, Recs: nil}
	}

	// Get candidate map from pool and clear it
	candidates := candidatePool.Get().(map[int32]struct{})
	for k := range candidates {
		delete(candidates, k)
	}

	// Find candidate anime (those sharing at least one tag)
	for tag := range anime.Tags {
		entries := index.Get(tag)
		for _, entry := range entries {
			if entry.AnimeID != anime.ID {
				candidates[entry.AnimeID] = struct{}{}
			}
		}
	}

	// Get similarity slice from pool and reset it
	similaritiesPtr := similarityPool.Get().(*[]Recommendation)
	similarities := (*similaritiesPtr)[:0]

	// Compute similarity for each candidate
	for candidateID := range candidates {
		candidate, exists := animeData[candidateID]
		if !exists || candidate == nil || candidate.Type != anime.Type {
			continue
		}

		sim := computeWeightedJaccard(anime.Tags, candidate.Tags, anime.TagSum, candidate.TagSum)
		if sim > 0 {
			similarities = append(similarities, Recommendation{
				TargetID:   candidateID,
				Similarity: sim,
			})
		}
	}

	// Return candidate map to pool
	candidatePool.Put(candidates)

	// Sort by similarity descending
	sort.Slice(similarities, func(i, j int) bool {
		return similarities[i].Similarity > similarities[j].Similarity
	})

	// Keep top N - make a copy for the result since we're returning the slice to pool
	resultLen := len(similarities)
	if resultLen > TOP_N {
		resultLen = TOP_N
	}
	resultRecs := make([]Recommendation, resultLen)
	copy(resultRecs, similarities[:resultLen])

	// Return similarity slice to pool
	*similaritiesPtr = similarities
	similarityPool.Put(similaritiesPtr)

	// Compute threshold (200th similarity or 0 if less than 200)
	var threshold float32 = 0
	if len(resultRecs) == TOP_N {
		threshold = resultRecs[TOP_N-1].Similarity
	}

	return &AnimeRecs{
		SourceID:  anime.ID,
		Threshold: threshold,
		Recs:      resultRecs,
	}
}

// Compute recommendations for all anime of a given type (parallelized)
// Processes in batches to control memory usage
func computeAllRecs(
	animeData map[int32]*AnimeTagData,
	index *InvertedIndex,
	mediaType string,
	numWorkers int,
) map[int32]*AnimeRecs {
	// Filter anime by type
	var animeList []*AnimeTagData
	for _, anime := range animeData {
		if anime.Type == mediaType && len(anime.Tags) > 0 {
			animeList = append(animeList, anime)
		}
	}

	fmt.Printf("   Computing recommendations for %d %s entries using %d workers...\n",
		len(animeList), mediaType, numWorkers)
	logMemoryUsage(fmt.Sprintf("%s start", mediaType))

	// Pre-allocate results map with expected capacity
	results := make(map[int32]*AnimeRecs, len(animeList))
	var mu sync.Mutex

	// Process in batches to control memory
	batchSize := 1000
	totalBatches := (len(animeList) + batchSize - 1) / batchSize
	startTime := time.Now()
	totalProcessed := 0

	// Create workers once, reuse across batches
	work := make(chan *AnimeTagData, 256)
	var wg sync.WaitGroup

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for anime := range work {
				recs := computeRecsForAnime(anime, animeData, index)
				mu.Lock()
				results[anime.ID] = recs
				mu.Unlock()
			}
		}()
	}

	for batchNum := 0; batchNum < totalBatches; batchNum++ {
		batchStart := batchNum * batchSize
		batchEnd := batchStart + batchSize
		if batchEnd > len(animeList) {
			batchEnd = len(animeList)
		}
		batch := animeList[batchStart:batchEnd]

		// Feed batch
		for _, anime := range batch {
			work <- anime
		}

		totalProcessed += len(batch)
		elapsed := time.Since(startTime)
		rate := float64(totalProcessed) / elapsed.Seconds()
		remaining := float64(len(animeList)-totalProcessed) / rate

		fmt.Printf("      Batch %d/%d: Processed %d/%d (%.1f/sec, ~%.0fs remaining)\n",
			batchNum+1, totalBatches, totalProcessed, len(animeList), rate, remaining)

		// Log memory and GC every 10 batches (10k items)
		if (batchNum+1)%10 == 0 {
			logMemoryUsage(fmt.Sprintf("%s @%d", mediaType, totalProcessed))
			runtime.GC()
		}
	}

	close(work)
	wg.Wait()

	fmt.Printf("   Completed %d %s in %v\n", len(results), mediaType, time.Since(startTime))
	logMemoryUsage(fmt.Sprintf("%s end", mediaType))

	return results
}

// =====================================================
// BINARY FILE FORMAT
// =====================================================

// Write recommendations to binary file
func writeBinaryFile(path string, allRecs map[int32]*AnimeRecs) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	// Write header
	header := BinaryHeader{
		Version:  BINARY_VERSION,
		NumAnime: uint32(len(allRecs)),
	}
	if err := binary.Write(writer, binary.LittleEndian, header); err != nil {
		return err
	}

	// Sort anime IDs for consistent ordering
	var animeIDs []int32
	for id := range allRecs {
		animeIDs = append(animeIDs, id)
	}
	sort.Slice(animeIDs, func(i, j int) bool { return animeIDs[i] < animeIDs[j] })

	// Calculate offsets for threshold index
	headerSize := uint32(16) // 4 uint32s
	thresholdEntrySize := uint32(12) // int32 + float32 + uint32
	thresholdTableSize := thresholdEntrySize * uint32(len(animeIDs))
	recsStart := headerSize + thresholdTableSize

	// Write threshold index
	currentOffset := recsStart
	for _, id := range animeIDs {
		recs := allRecs[id]
		entry := ThresholdEntry{
			AnimeID:   id,
			Threshold: recs.Threshold,
			Offset:    currentOffset,
		}
		if err := binary.Write(writer, binary.LittleEndian, entry); err != nil {
			return err
		}
		// Each rec is 8 bytes (int32 + float32), plus 4 bytes for count
		currentOffset += 4 + uint32(len(recs.Recs))*8
	}

	// Write recommendations
	for _, id := range animeIDs {
		recs := allRecs[id]
		// Write count
		if err := binary.Write(writer, binary.LittleEndian, uint32(len(recs.Recs))); err != nil {
			return err
		}
		// Write each recommendation
		for _, rec := range recs.Recs {
			if err := binary.Write(writer, binary.LittleEndian, rec.TargetID); err != nil {
				return err
			}
			if err := binary.Write(writer, binary.LittleEndian, rec.Similarity); err != nil {
				return err
			}
		}
	}

	return writer.Flush()
}

// Read thresholds from binary file (for incremental updates)
func readThresholds(path string) (map[int32]float32, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	// Read header
	var header BinaryHeader
	if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Version != BINARY_VERSION {
		return nil, fmt.Errorf("unsupported binary version: %d", header.Version)
	}

	// Read threshold entries
	thresholds := make(map[int32]float32, header.NumAnime)
	for i := uint32(0); i < header.NumAnime; i++ {
		var entry ThresholdEntry
		if err := binary.Read(reader, binary.LittleEndian, &entry); err != nil {
			return nil, err
		}
		thresholds[entry.AnimeID] = entry.Threshold
	}

	return thresholds, nil
}

// Read full recommendations from binary file
func readBinaryFile(path string) (map[int32]*AnimeRecs, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)

	// Read header
	var header BinaryHeader
	if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
		return nil, err
	}

	if header.Version != BINARY_VERSION {
		return nil, fmt.Errorf("unsupported binary version: %d", header.Version)
	}

	// Read threshold entries (to get offsets)
	entries := make([]ThresholdEntry, header.NumAnime)
	for i := uint32(0); i < header.NumAnime; i++ {
		if err := binary.Read(reader, binary.LittleEndian, &entries[i]); err != nil {
			return nil, err
		}
	}

	// Read recommendations
	allRecs := make(map[int32]*AnimeRecs, header.NumAnime)
	for _, entry := range entries {
		var count uint32
		if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
			return nil, err
		}

		recs := make([]Recommendation, count)
		for j := uint32(0); j < count; j++ {
			if err := binary.Read(reader, binary.LittleEndian, &recs[j].TargetID); err != nil {
				return nil, err
			}
			if err := binary.Read(reader, binary.LittleEndian, &recs[j].Similarity); err != nil {
				return nil, err
			}
		}

		allRecs[entry.AnimeID] = &AnimeRecs{
			SourceID:  entry.AnimeID,
			Threshold: entry.Threshold,
			Recs:      recs,
		}
	}

	return allRecs, nil
}

// =====================================================
// CSV OUTPUT (for PostgreSQL import)
// =====================================================

func writeCSV(path string, allRecs map[int32]*AnimeRecs) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"source_anilist_id", "target_anilist_id", "similarity", "rank"})

	// Sort for consistent output
	var animeIDs []int32
	for id := range allRecs {
		animeIDs = append(animeIDs, id)
	}
	sort.Slice(animeIDs, func(i, j int) bool { return animeIDs[i] < animeIDs[j] })

	totalRecs := 0
	for _, id := range animeIDs {
		recs := allRecs[id]
		for rank, rec := range recs.Recs {
			writer.Write([]string{
				strconv.Itoa(int(id)),
				strconv.Itoa(int(rec.TargetID)),
				fmt.Sprintf("%.6f", rec.Similarity),
				strconv.Itoa(rank + 1),
			})
			totalRecs++
		}
	}

	writer.Flush()
	fmt.Printf("   Wrote %d recommendations to CSV\n", totalRecs)
	return writer.Error()
}

// =====================================================
// INCREMENTAL UPDATE
// =====================================================

// Update existing recommendations with new anime
func incrementalUpdate(
	existingRecs map[int32]*AnimeRecs,
	newAnimeData map[int32]*AnimeTagData,
	allAnimeData map[int32]*AnimeTagData,
	indices map[string]*InvertedIndex,
) (updatedRecs map[int32]*AnimeRecs, newRecs map[int32]*AnimeRecs) {
	updatedRecs = make(map[int32]*AnimeRecs)
	newRecs = make(map[int32]*AnimeRecs)

	// Part 1: Compute recommendations for new anime
	fmt.Println("   Part 1: Computing recommendations for new anime...")
	for _, anime := range newAnimeData {
		index := indices[anime.Type]
		if index == nil {
			continue
		}
		recs := computeRecsForAnime(anime, allAnimeData, index)
		newRecs[anime.ID] = recs
	}
	fmt.Printf("   Generated recommendations for %d new entries\n", len(newRecs))

	// Part 2: Check if new anime should enter existing anime's top 200
	fmt.Println("   Part 2: Updating existing anime with new recommendations...")
	updatedCount := 0

	for existingID, existingAnimeRecs := range existingRecs {
		existingAnime, exists := allAnimeData[existingID]
		if !exists {
			continue
		}

		// Check each new anime of the same type
		updated := false
		for _, newAnime := range newAnimeData {
			if newAnime.Type != existingAnime.Type {
				continue
			}

			sim := computeWeightedJaccard(existingAnime.Tags, newAnime.Tags,
				existingAnime.TagSum, newAnime.TagSum)

			// Check if this new anime should enter the top 200
			if sim > existingAnimeRecs.Threshold || len(existingAnimeRecs.Recs) < TOP_N {
				// Add new recommendation
				existingAnimeRecs.Recs = append(existingAnimeRecs.Recs, Recommendation{
					TargetID:   newAnime.ID,
					Similarity: sim,
				})
				updated = true
			}
		}

		if updated {
			// Re-sort and trim to top 200
			sort.Slice(existingAnimeRecs.Recs, func(i, j int) bool {
				return existingAnimeRecs.Recs[i].Similarity > existingAnimeRecs.Recs[j].Similarity
			})
			if len(existingAnimeRecs.Recs) > TOP_N {
				existingAnimeRecs.Recs = existingAnimeRecs.Recs[:TOP_N]
			}
			// Update threshold
			if len(existingAnimeRecs.Recs) == TOP_N {
				existingAnimeRecs.Threshold = existingAnimeRecs.Recs[TOP_N-1].Similarity
			}
			updatedRecs[existingID] = existingAnimeRecs
			updatedCount++
		}
	}

	fmt.Printf("   Updated %d existing anime with new recommendations\n", updatedCount)
	return updatedRecs, newRecs
}

// =====================================================
// MAIN
// =====================================================

func main() {
	// Command line flags
	dataDir := flag.String("dir", ".", "Directory containing CSV files")
	outputDir := flag.String("output", "", "Output directory (defaults to data dir)")
	incremental := flag.Bool("incremental", false, "Incremental mode (requires existing binary file)")
	workers := flag.Int("workers", 0, "Number of worker goroutines (default: num CPUs)")
	flag.Parse()

	if *outputDir == "" {
		*outputDir = *dataDir
	}

	if *workers == 0 {
		*workers = runtime.NumCPU()
	}

	startTime := time.Now()
	fmt.Println("\n🔄 Recommendation Computation")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("   Data directory: %s\n", *dataDir)
	fmt.Printf("   Output directory: %s\n", *outputDir)
	fmt.Printf("   Mode: %s\n", map[bool]string{true: "incremental", false: "full"}[*incremental])
	fmt.Printf("   Workers: %d\n", *workers)

	// File paths
	animeTagsFullPath := filepath.Join(*dataDir, "anime_tags_full.csv")
	mangaTagsFullPath := filepath.Join(*dataDir, "manga_tags_full.csv")
	mediaDeltaPath := filepath.Join(*dataDir, "media_delta.csv")
	binaryPath := filepath.Join(*outputDir, "recommendations.bin")
	csvPath := filepath.Join(*outputDir, "recommendations.csv")

	// Load anime data from cumulative split files
	fmt.Println("\n📂 Loading data from cumulative tag files...")
	var animeData map[int32]*AnimeTagData
	var err error

	// Check if cumulative files exist
	animeTagsExists := false
	mangaTagsExists := false
	if _, err := os.Stat(animeTagsFullPath); err == nil {
		animeTagsExists = true
	}
	if _, err := os.Stat(mangaTagsFullPath); err == nil {
		mangaTagsExists = true
	}

	if animeTagsExists || mangaTagsExists {
		// Load from cumulative split files (preferred)
		animeData, err = loadAnimeDataFromSplitFiles(animeTagsFullPath, mangaTagsFullPath)
	} else {
		// Fallback: try legacy loading methods
		fmt.Println("   Cumulative files not found, trying legacy methods...")
		legacyTagsPath := filepath.Join(*dataDir, "anime_tags_delta.csv")
		legacyMediaPath := filepath.Join(*dataDir, "media_delta.csv")

		// Check for full media.csv
		fullMediaPath := filepath.Join(*dataDir, "media.csv")
		if _, err := os.Stat(fullMediaPath); err == nil {
			legacyMediaPath = fullMediaPath
		}

		if _, err := os.Stat(legacyTagsPath); err == nil {
			animeData, err = loadAnimeData(legacyMediaPath, legacyTagsPath)
		} else {
			animeData, err = loadAnimeDataFromMedia(legacyMediaPath)
		}
	}
	if err != nil {
		fmt.Printf("ERROR: Failed to load data: %v\n", err)
		os.Exit(1)
	}

	// Build inverted indices
	fmt.Println("\n📊 Building inverted indices...")
	indices := buildInvertedIndices(animeData)
	for mediaType, idx := range indices {
		fmt.Printf("   %s: %d unique tags\n", mediaType, len(idx.Index))
	}

	var allRecs map[int32]*AnimeRecs
	var changedRecs map[int32]*AnimeRecs

	if *incremental {
		// Incremental mode
		fmt.Println("\n🔄 Incremental update mode...")

		// Load existing recommendations
		existingRecs, err := readBinaryFile(binaryPath)
		if err != nil {
			fmt.Printf("ERROR: Failed to read existing binary file: %v\n", err)
			fmt.Println("   Falling back to full computation...")
			*incremental = false
		} else {
			fmt.Printf("   Loaded %d existing recommendation sets\n", len(existingRecs))

			// Get new anime IDs from delta file
			newAnimeIDs, err := getNewAnimeIDsFromDelta(mediaDeltaPath)
			if err != nil {
				fmt.Printf("WARNING: Failed to read delta file: %v\n", err)
				newAnimeIDs = make(map[int32]bool)
			}

			// Build new anime data map (only anime from delta that we have tag data for)
			newAnimeData := make(map[int32]*AnimeTagData)
			for id := range newAnimeIDs {
				if anime, exists := animeData[id]; exists {
					newAnimeData[id] = anime
				}
			}
			fmt.Printf("   Found %d new anime from delta to process\n", len(newAnimeData))

			if len(newAnimeData) > 0 {
				// Run incremental update with FULL anime data for similarity computation
				updatedRecs, newRecs := incrementalUpdate(existingRecs, newAnimeData, animeData, indices)

				// Track changed recs for CSV output
				changedRecs = make(map[int32]*AnimeRecs)
				for id, recs := range updatedRecs {
					changedRecs[id] = recs
				}
				for id, recs := range newRecs {
					changedRecs[id] = recs
				}

				// Merge all recommendations
				allRecs = existingRecs
				for id, recs := range updatedRecs {
					allRecs[id] = recs
				}
				for id, recs := range newRecs {
					allRecs[id] = recs
				}
			} else {
				allRecs = existingRecs
				changedRecs = make(map[int32]*AnimeRecs)
				fmt.Println("   No new anime to process")
			}
		}
	}

	if !*incremental || allRecs == nil {
		// Full computation mode
		fmt.Println("\n🔄 Full computation mode...")
		allRecs = make(map[int32]*AnimeRecs)

		for _, mediaType := range []string{"ANIME", "MANGA"} {
			idx := indices[mediaType]
			if len(idx.Index) == 0 {
				continue
			}

			typeRecs := computeAllRecs(animeData, idx, mediaType, *workers)
			for id, recs := range typeRecs {
				allRecs[id] = recs
			}
		}
		changedRecs = nil // Full mode: write all recs to CSV
	}

	// Write outputs
	fmt.Println("\n💾 Writing output files...")

	if err := writeBinaryFile(binaryPath, allRecs); err != nil {
		fmt.Printf("ERROR: Failed to write binary file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Wrote binary file: %s\n", binaryPath)

	// In incremental mode, write only changed recs to CSV; in full mode, write all
	recsToWrite := allRecs
	if changedRecs != nil {
		recsToWrite = changedRecs
		fmt.Printf("   Incremental mode: Writing only %d changed anime to CSV\n", len(changedRecs))
	}

	if err := writeCSV(csvPath, recsToWrite); err != nil {
		fmt.Printf("ERROR: Failed to write CSV file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("   Wrote CSV file: %s\n", csvPath)

	// Summary
	totalRecs := 0
	for _, recs := range allRecs {
		totalRecs += len(recs.Recs)
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("✅ Completed in %v\n", time.Since(startTime))
	fmt.Printf("   Total anime processed: %d\n", len(allRecs))
	fmt.Printf("   Total recommendations: %d\n", totalRecs)
	fmt.Printf("   Binary file: %s\n", binaryPath)
	fmt.Printf("   CSV file: %s\n", csvPath)
}
