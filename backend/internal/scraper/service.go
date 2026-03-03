package scraper

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	pb "anigraph/backend/gen/anigraph/v1"
)

// Service implements the ScraperService ConnectRPC handler.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) ScrapeIncremental(
	ctx context.Context,
	req *connect.Request[pb.ScrapeIncrementalRequest],
	stream *connect.ServerStream[pb.ScrapeIncrementalResponse],
) error {
	maxID := req.Msg.GetMaxId()
	startTime := time.Now()

	// Use a temp directory for output files.
	outputDir, err := os.MkdirTemp("", "scrape-*")
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("create temp dir: %w", err))
	}
	defer os.RemoveAll(outputDir)

	// Send initial progress.
	if err := stream.Send(&pb.ScrapeIncrementalResponse{
		Payload: &pb.ScrapeIncrementalResponse_Progress{
			Progress: &pb.ProgressUpdate{
				Phase:   "starting",
				Message: fmt.Sprintf("Starting incremental scrape from max_id=%d", maxID),
			},
		},
	}); err != nil {
		return err
	}

	// Build and run the scraper binary.
	binaryPath := findBinary("scrape_incremental")
	cmd := exec.CommandContext(ctx, binaryPath,
		fmt.Sprintf("--max-id=%d", maxID),
		fmt.Sprintf("--output-dir=%s", outputDir),
	)
	cmd.Env = os.Environ()

	// Capture stderr for progress.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("stderr pipe: %w", err))
	}

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start scraper: %w", err))
	}

	// Stream stderr lines as progress.
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if err := stream.Send(&pb.ScrapeIncrementalResponse{
			Payload: &pb.ScrapeIncrementalResponse_Progress{
				Progress: &pb.ProgressUpdate{
					Phase:       "scraping",
					Message:     line,
					ElapsedSecs: time.Since(startTime).Seconds(),
				},
			},
		}); err != nil {
			log.Printf("stream send error: %v", err)
		}
	}

	if err := cmd.Wait(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("scraper exited: %w", err))
	}

	// Read output CSV files and stream as batches.
	batch := &pb.ScrapeBatch{}

	// Read media delta.
	mediaCount, err := readMediaCSV(filepath.Join(outputDir, "media_delta.csv"), batch)
	if err != nil {
		log.Printf("warning: media CSV read error: %v", err)
	}

	// Read staff delta.
	staffCount, err := readStaffCSV(filepath.Join(outputDir, "staff_delta.csv"), batch)
	if err != nil {
		log.Printf("warning: staff CSV read error: %v", err)
	}

	// Read edges delta.
	edgeCount, err := readEdgesCSV(filepath.Join(outputDir, "media_staff_edges_delta.csv"), batch)
	if err != nil {
		log.Printf("warning: edges CSV read error: %v", err)
	}

	// Read relations delta.
	relCount, err := readRelationsCSV(filepath.Join(outputDir, "media_relations_delta.csv"), batch)
	if err != nil {
		log.Printf("warning: relations CSV read error: %v", err)
	}

	// Read changed staff IDs.
	changedStaffIDs, _ := readChangedStaffIDs(filepath.Join(outputDir, "changed_staff_ids.txt"))
	batch.ChangedStaffIds = changedStaffIDs

	// Send the batch.
	if err := stream.Send(&pb.ScrapeIncrementalResponse{
		Payload: &pb.ScrapeIncrementalResponse_Batch{Batch: batch},
	}); err != nil {
		return err
	}

	// Read failed pages.
	failedPages, _ := readFailedPages(filepath.Join(outputDir, "failed_pages.txt"))

	// Send completion.
	duration := time.Since(startTime)
	return stream.Send(&pb.ScrapeIncrementalResponse{
		Payload: &pb.ScrapeIncrementalResponse_Complete{
			Complete: &pb.ScrapeComplete{
				TotalMedia:        int32(mediaCount),
				TotalStaff:        int32(staffCount),
				TotalEdges:        int32(edgeCount),
				TotalRelations:    int32(relCount),
				ChangedStaffCount: int32(len(changedStaffIDs)),
				DurationSecs:      duration.Seconds(),
				FailedPages:       failedPages,
			},
		},
	})
}

// findBinary locates a Go binary in common locations.
func findBinary(name string) string {
	// Check for pre-built binary in the scraper directory.
	candidates := []string{
		filepath.Join("/app/backend", name),
		filepath.Join(".", name),
		name,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return name // fallback to PATH
}

// readMediaCSV reads media_delta.csv and populates batch.Media.
func readMediaCSV(path string, batch *pb.ScrapeBatch) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return 0, err
	}

	colIdx := makeColIndex(header)
	count := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		entry := &pb.MediaEntry{
			AnilistId:        getInt32(record, colIdx, "id"),
			Type:             getStr(record, colIdx, "type"),
			Format:           getStr(record, colIdx, "format"),
			Status:           getStr(record, colIdx, "status"),
			TitleRomaji:      getStr(record, colIdx, "title_romaji"),
			TitleEnglish:     getStr(record, colIdx, "title_english"),
			TitleNative:      getStr(record, colIdx, "title_native"),
			Description:      getStr(record, colIdx, "description"),
			Season:           getStr(record, colIdx, "season"),
			SeasonYear:       getInt32(record, colIdx, "seasonYear"),
			Episodes:         getInt32(record, colIdx, "episodes"),
			Duration:         getInt32(record, colIdx, "duration"),
			Chapters:         getInt32(record, colIdx, "chapters"),
			Volumes:          getInt32(record, colIdx, "volumes"),
			CountryOfOrigin:  getStr(record, colIdx, "countryOfOrigin"),
			Source:           getStr(record, colIdx, "source"),
			CoverImageLarge:  getStr(record, colIdx, "coverImage_large"),
			CoverImageMedium: getStr(record, colIdx, "coverImage_medium"),
			CoverImageColor:  getStr(record, colIdx, "coverImage_color"),
			BannerImage:      getStr(record, colIdx, "bannerImage"),
			AverageScore:     getInt32(record, colIdx, "averageScore"),
			MeanScore:        getInt32(record, colIdx, "meanScore"),
			Popularity:       getInt32(record, colIdx, "popularity"),
			Favourites:       getInt32(record, colIdx, "favourites"),
			IsAdult:          getStr(record, colIdx, "isAdult") == "true",
			SiteUrl:          getStr(record, colIdx, "siteUrl"),
			MalId:            getInt32(record, colIdx, "idMal"),
			UpdatedAt:        getInt32(record, colIdx, "updatedAt"),
		}

		// Parse start date.
		if y := getInt32(record, colIdx, "startDate_year"); y > 0 {
			entry.StartDate = &pb.FuzzyDate{
				Year:  y,
				Month: getInt32(record, colIdx, "startDate_month"),
				Day:   getInt32(record, colIdx, "startDate_day"),
			}
		}

		// Parse end date.
		if y := getInt32(record, colIdx, "endDate_year"); y > 0 {
			entry.EndDate = &pb.FuzzyDate{
				Year:  y,
				Month: getInt32(record, colIdx, "endDate_month"),
				Day:   getInt32(record, colIdx, "endDate_day"),
			}
		}

		// Parse genres (pipe-delimited).
		if g := getStr(record, colIdx, "genres"); g != "" {
			entry.Genres = strings.Split(g, "|")
		}

		// Parse synonyms (pipe-delimited).
		if syn := getStr(record, colIdx, "synonyms"); syn != "" {
			entry.Synonyms = strings.Split(syn, "|")
		}

		batch.Media = append(batch.Media, entry)
		count++
	}

	return count, nil
}

// readStaffCSV reads staff_delta.csv and populates batch.Staff.
func readStaffCSV(path string, batch *pb.ScrapeBatch) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return 0, err
	}

	colIdx := makeColIndex(header)
	count := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		entry := &pb.StaffEntry{
			StaffId:    getInt32(record, colIdx, "id"),
			NameFull:   getStr(record, colIdx, "name_full"),
			NameNative: getStr(record, colIdx, "name_native"),
			NameFirst:  getStr(record, colIdx, "name_first"),
			NameMiddle: getStr(record, colIdx, "name_middle"),
			NameLast:   getStr(record, colIdx, "name_last"),
			ImageLarge: getStr(record, colIdx, "image_large"),
			Description: getStr(record, colIdx, "description"),
			Gender:     getStr(record, colIdx, "gender"),
			HomeTown:   getStr(record, colIdx, "homeTown"),
			SiteUrl:    getStr(record, colIdx, "siteUrl"),
			Favourites: getInt32(record, colIdx, "favourites"),
		}

		batch.Staff = append(batch.Staff, entry)
		count++
	}

	return count, nil
}

// readEdgesCSV reads media_staff_edges_delta.csv and populates batch.Edges.
func readEdgesCSV(path string, batch *pb.ScrapeBatch) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return 0, err
	}

	colIdx := makeColIndex(header)
	count := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		batch.Edges = append(batch.Edges, &pb.MediaStaffEdge{
			MediaId: getInt32(record, colIdx, "mediaId"),
			StaffId: getInt32(record, colIdx, "staffId"),
			Role:    getStr(record, colIdx, "role"),
		})
		count++
	}

	return count, nil
}

// readRelationsCSV reads media_relations_delta.csv and populates batch.Relations.
func readRelationsCSV(path string, batch *pb.ScrapeBatch) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return 0, err
	}

	colIdx := makeColIndex(header)
	count := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		batch.Relations = append(batch.Relations, &pb.MediaRelation{
			SourceId:     getInt32(record, colIdx, "sourceId"),
			TargetId:     getInt32(record, colIdx, "targetId"),
			RelationType: getStr(record, colIdx, "relationType"),
		})
		count++
	}

	return count, nil
}

// readChangedStaffIDs reads changed_staff_ids.txt (one ID per line).
func readChangedStaffIDs(path string) ([]int32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var ids []int32
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if id, err := strconv.Atoi(line); err == nil {
			ids = append(ids, int32(id))
		}
	}
	return ids, nil
}

// readFailedPages reads failed_pages.txt.
func readFailedPages(path string) ([]int32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pages []int32
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if p, err := strconv.Atoi(line); err == nil {
			pages = append(pages, int32(p))
		}
	}
	return pages, nil
}

// Helper: build column name -> index map.
func makeColIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[h] = i
	}
	return m
}

// Helper: get string from record by column name.
func getStr(record []string, colIdx map[string]int, col string) string {
	if idx, ok := colIdx[col]; ok && idx < len(record) {
		return record[idx]
	}
	return ""
}

// Helper: get int32 from record by column name.
func getInt32(record []string, colIdx map[string]int, col string) int32 {
	s := getStr(record, colIdx, col)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return int32(v)
}
