package enrichment

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"

	pb "anigraph/backend/gen/anigraph/v1"
)

// Service implements the EnrichmentService ConnectRPC handler.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

// EnrichWikidata resolves anime to Wikidata QIDs via stdin/stdout CSV.
func (s *Service) EnrichWikidata(
	ctx context.Context,
	stream *connect.BidiStream[pb.EnrichWikidataRequest, pb.EnrichWikidataResponse],
) error {
	startTime := time.Now()

	// Collect all input entries from client stream.
	var entries []*pb.AnimeWikidataInput
	for {
		msg, err := stream.Receive()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entries = append(entries, msg.GetEntries()...)
	}

	if len(entries) == 0 {
		return nil
	}

	// Run wikidata_qid binary with stdin CSV.
	binaryPath := findBinary("wikidata_qid")
	cmd := exec.CommandContext(ctx, binaryPath, "-sparql-workers=100")
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("stdin pipe: %w", err))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("stdout pipe: %w", err))
	}

	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start wikidata_qid: %w", err))
	}

	// Stream stderr as progress in background.
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			_ = stream.Send(&pb.EnrichWikidataResponse{
				Payload: &pb.EnrichWikidataResponse_Progress{
					Progress: &pb.ProgressUpdate{
						Phase:       "enriching",
						Message:     line,
						ElapsedSecs: time.Since(startTime).Seconds(),
					},
				},
			})
		}
	}()

	// Write CSV to stdin.
	go func() {
		for _, e := range entries {
			fmt.Fprintf(stdin, "%d,%d,%s,%s,%d,%s,%s\n",
				e.AnilistId, e.MalId,
				csvEscape(e.TitleEnglish), csvEscape(e.TitleRomaji),
				e.SeasonYear, e.Type, e.Format)
		}
		stdin.Close()
	}()

	// Read stdout CSV results.
	reader := csv.NewReader(stdout)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		_ = cmd.Wait()
		return connect.NewError(connect.CodeInternal, fmt.Errorf("read header: %w", err))
	}
	colIdx := makeColIndex(header)

	var batch []*pb.WikidataResult
	batchSize := 100

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		result := &pb.WikidataResult{
			AnilistId:            getInt32(record, colIdx, "anilist_id"),
			WikidataQid:          getStr(record, colIdx, "wikidata_qid"),
			Method:               getStr(record, colIdx, "method"),
			WikipediaEn:          getStr(record, colIdx, "wikipedia_en"),
			WikipediaJa:          getStr(record, colIdx, "wikipedia_ja"),
			LivechartId:          getStr(record, colIdx, "livechart_id"),
			NotifyId:             getStr(record, colIdx, "notify_id"),
			TvdbId:               getStr(record, colIdx, "tvdb_id"),
			TmdbMovieId:          getStr(record, colIdx, "tmdb_movie_id"),
			TmdbTvId:             getStr(record, colIdx, "tmdb_tv_id"),
			TvmazeId:             getStr(record, colIdx, "tvmaze_id"),
			MywaifulistId:        getStr(record, colIdx, "mywaifulist_id"),
			UnconsentingMediaId:  getStr(record, colIdx, "unconsenting_media_id"),
		}
		batch = append(batch, result)

		if len(batch) >= batchSize {
			if err := stream.Send(&pb.EnrichWikidataResponse{
				Payload: &pb.EnrichWikidataResponse_Results{
					Results: &pb.WikidataResultBatch{Entries: batch},
				},
			}); err != nil {
				log.Printf("stream send error: %v", err)
			}
			batch = nil
		}
	}

	// Send remaining batch.
	if len(batch) > 0 {
		_ = stream.Send(&pb.EnrichWikidataResponse{
			Payload: &pb.EnrichWikidataResponse_Results{
				Results: &pb.WikidataResultBatch{Entries: batch},
			},
		})
	}

	return cmd.Wait()
}

// BackfillWikidataProps fetches extra properties for anime that already have QIDs.
func (s *Service) BackfillWikidataProps(
	ctx context.Context,
	stream *connect.BidiStream[pb.BackfillWikidataPropsRequest, pb.BackfillWikidataPropsResponse],
) error {
	startTime := time.Now()

	var entries []*pb.AnimeWikidataPropsInput
	for {
		msg, err := stream.Receive()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entries = append(entries, msg.GetEntries()...)
	}

	if len(entries) == 0 {
		return nil
	}

	binaryPath := findBinary("wikidata_backfill_props")
	cmd := exec.CommandContext(ctx, binaryPath, "-sparql-workers=100")
	cmd.Env = os.Environ()

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start: %w", err))
	}

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			_ = stream.Send(&pb.BackfillWikidataPropsResponse{
				Payload: &pb.BackfillWikidataPropsResponse_Progress{
					Progress: &pb.ProgressUpdate{
						Phase:       "backfilling",
						Message:     scanner.Text(),
						ElapsedSecs: time.Since(startTime).Seconds(),
					},
				},
			})
		}
	}()

	go func() {
		for _, e := range entries {
			fmt.Fprintf(stdin, "%d,%s\n", e.AnilistId, e.WikidataQid)
		}
		stdin.Close()
	}()

	reader := csv.NewReader(stdout)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		_ = cmd.Wait()
		return connect.NewError(connect.CodeInternal, fmt.Errorf("read header: %w", err))
	}
	colIdx := makeColIndex(header)

	var batch []*pb.WikidataResult
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		batch = append(batch, &pb.WikidataResult{
			AnilistId:            getInt32(record, colIdx, "anilist_id"),
			WikidataQid:          getStr(record, colIdx, "wikidata_qid"),
			WikipediaEn:          getStr(record, colIdx, "wikipedia_en"),
			WikipediaJa:          getStr(record, colIdx, "wikipedia_ja"),
			LivechartId:          getStr(record, colIdx, "livechart_id"),
			NotifyId:             getStr(record, colIdx, "notify_id"),
			TvdbId:               getStr(record, colIdx, "tvdb_id"),
			TmdbMovieId:          getStr(record, colIdx, "tmdb_movie_id"),
			TmdbTvId:             getStr(record, colIdx, "tmdb_tv_id"),
			TvmazeId:             getStr(record, colIdx, "tvmaze_id"),
			MywaifulistId:        getStr(record, colIdx, "mywaifulist_id"),
			UnconsentingMediaId:  getStr(record, colIdx, "unconsenting_media_id"),
		})

		if len(batch) >= 100 {
			_ = stream.Send(&pb.BackfillWikidataPropsResponse{
				Payload: &pb.BackfillWikidataPropsResponse_Results{
					Results: &pb.WikidataResultBatch{Entries: batch},
				},
			})
			batch = nil
		}
	}

	if len(batch) > 0 {
		_ = stream.Send(&pb.BackfillWikidataPropsResponse{
			Payload: &pb.BackfillWikidataPropsResponse_Results{
				Results: &pb.WikidataResultBatch{Entries: batch},
			},
		})
	}

	return cmd.Wait()
}

// BackfillMalIds maps AniList IDs to MAL IDs via public databases.
func (s *Service) BackfillMalIds(
	ctx context.Context,
	req *connect.Request[pb.BackfillMalIdsRequest],
) (*connect.Response[pb.BackfillMalIdsResponse], error) {
	ids := req.Msg.GetAnilistIds()
	if len(ids) == 0 {
		return connect.NewResponse(&pb.BackfillMalIdsResponse{}), nil
	}

	binaryPath := findBinary("backfill_mal_ids")
	cmd := exec.CommandContext(ctx, binaryPath)

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()

	if err := cmd.Start(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("start: %w", err))
	}

	go func() {
		for _, id := range ids {
			fmt.Fprintf(stdin, "%d\n", id)
		}
		stdin.Close()
	}()

	reader := csv.NewReader(stdout)
	reader.FieldsPerRecord = -1

	// Skip header.
	if _, err := reader.Read(); err != nil {
		_ = cmd.Wait()
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("read header: %w", err))
	}

	var mappings []*pb.MalIdMapping
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			continue
		}
		anilistID, _ := strconv.Atoi(record[0])
		malID, _ := strconv.Atoi(record[1])
		mappings = append(mappings, &pb.MalIdMapping{
			AnilistId: int32(anilistID),
			MalId:     int32(malID),
		})
	}

	if err := cmd.Wait(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("backfill_mal_ids exited: %w", err))
	}

	return connect.NewResponse(&pb.BackfillMalIdsResponse{
		Mappings: mappings,
		Found:    int32(len(mappings)),
		Missing:  int32(len(ids)) - int32(len(mappings)),
	}), nil
}

// ScrapeWikipediaProduction extracts "Production" sections from Wikipedia.
func (s *Service) ScrapeWikipediaProduction(
	ctx context.Context,
	stream *connect.BidiStream[pb.ScrapeWikipediaProductionRequest, pb.ScrapeWikipediaProductionResponse],
) error {
	startTime := time.Now()

	var entries []*pb.WikipediaInput
	for {
		msg, err := stream.Receive()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entries = append(entries, msg.GetEntries()...)
	}

	if len(entries) == 0 {
		return nil
	}

	binaryPath := findBinary("wikipedia_production")
	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Env = os.Environ()

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start: %w", err))
	}

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			_ = stream.Send(&pb.ScrapeWikipediaProductionResponse{
				Payload: &pb.ScrapeWikipediaProductionResponse_Progress{
					Progress: &pb.ProgressUpdate{
						Phase:       "scraping_wikipedia",
						Message:     scanner.Text(),
						ElapsedSecs: time.Since(startTime).Seconds(),
					},
				},
			})
		}
	}()

	go func() {
		fmt.Fprintln(stdin, "anilist_id,wikipedia_en")
		for _, e := range entries {
			fmt.Fprintf(stdin, "%d,%s\n", e.AnilistId, csvEscape(e.WikipediaEn))
		}
		stdin.Close()
	}()

	reader := csv.NewReader(stdout)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		_ = cmd.Wait()
		return connect.NewError(connect.CodeInternal, fmt.Errorf("read header: %w", err))
	}
	colIdx := makeColIndex(header)

	var batch []*pb.WikipediaProductionResult
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		batch = append(batch, &pb.WikipediaProductionResult{
			AnilistId:      getInt32(record, colIdx, "anilist_id"),
			WikipediaUrl:   getStr(record, colIdx, "wikipedia_url"),
			ProductionHtml: getStr(record, colIdx, "production_html"),
		})

		if len(batch) >= 50 {
			_ = stream.Send(&pb.ScrapeWikipediaProductionResponse{
				Payload: &pb.ScrapeWikipediaProductionResponse_Results{
					Results: &pb.WikipediaProductionBatch{Entries: batch},
				},
			})
			batch = nil
		}
	}

	if len(batch) > 0 {
		_ = stream.Send(&pb.ScrapeWikipediaProductionResponse{
			Payload: &pb.ScrapeWikipediaProductionResponse_Results{
				Results: &pb.WikipediaProductionBatch{Entries: batch},
			},
		})
	}

	return cmd.Wait()
}

// EnrichStaffAlternativeNames fetches staff alternative names from AniList.
func (s *Service) EnrichStaffAlternativeNames(
	ctx context.Context,
	stream *connect.BidiStream[pb.EnrichStaffAlternativeNamesRequest, pb.EnrichStaffAlternativeNamesResponse],
) error {
	startTime := time.Now()

	var staffIDs []int32
	for {
		msg, err := stream.Receive()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		staffIDs = append(staffIDs, msg.GetStaffIds()...)
	}

	if len(staffIDs) == 0 {
		return nil
	}

	binaryPath := findBinary("staff_alternative_names")
	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Env = os.Environ()

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start: %w", err))
	}

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			_ = stream.Send(&pb.EnrichStaffAlternativeNamesResponse{
				Payload: &pb.EnrichStaffAlternativeNamesResponse_Progress{
					Progress: &pb.ProgressUpdate{
						Phase:       "enriching_staff",
						Message:     scanner.Text(),
						ElapsedSecs: time.Since(startTime).Seconds(),
					},
				},
			})
		}
	}()

	go func() {
		for _, id := range staffIDs {
			fmt.Fprintf(stdin, "%d\n", id)
		}
		stdin.Close()
	}()

	reader := csv.NewReader(stdout)
	reader.FieldsPerRecord = -1

	// Skip header.
	if _, err := reader.Read(); err != nil {
		_ = cmd.Wait()
		return connect.NewError(connect.CodeInternal, fmt.Errorf("read header: %w", err))
	}

	var batch []*pb.StaffAlternativeNamesResult
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			continue
		}

		staffID, _ := strconv.Atoi(record[0])
		var altNames []string
		if record[1] != "" {
			altNames = strings.Split(record[1], "|")
		}

		batch = append(batch, &pb.StaffAlternativeNamesResult{
			StaffId:          int32(staffID),
			AlternativeNames: altNames,
		})

		if len(batch) >= 100 {
			_ = stream.Send(&pb.EnrichStaffAlternativeNamesResponse{
				Payload: &pb.EnrichStaffAlternativeNamesResponse_Results{
					Results: &pb.StaffAlternativeNamesBatch{Entries: batch},
				},
			})
			batch = nil
		}
	}

	if len(batch) > 0 {
		_ = stream.Send(&pb.EnrichStaffAlternativeNamesResponse{
			Payload: &pb.EnrichStaffAlternativeNamesResponse_Results{
				Results: &pb.StaffAlternativeNamesBatch{Entries: batch},
			},
		})
	}

	return cmd.Wait()
}

// EnrichWikidataStudios resolves studios to Wikidata entities.
func (s *Service) EnrichWikidataStudios(
	ctx context.Context,
	stream *connect.BidiStream[pb.EnrichWikidataStudiosRequest, pb.EnrichWikidataStudiosResponse],
) error {
	startTime := time.Now()

	var entries []*pb.StudioWikidataInput
	for {
		msg, err := stream.Receive()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entries = append(entries, msg.GetEntries()...)
	}

	if len(entries) == 0 {
		return nil
	}

	binaryPath := findBinary("wikidata_studios")
	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Env = os.Environ()

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start: %w", err))
	}

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			_ = stream.Send(&pb.EnrichWikidataStudiosResponse{
				Payload: &pb.EnrichWikidataStudiosResponse_Progress{
					Progress: &pb.ProgressUpdate{
						Phase:       "enriching_studios",
						Message:     scanner.Text(),
						ElapsedSecs: time.Since(startTime).Seconds(),
					},
				},
			})
		}
	}()

	go func() {
		for _, e := range entries {
			fmt.Fprintf(stdin, "%d,%s\n", e.StudioId, csvEscape(e.Name))
		}
		stdin.Close()
	}()

	reader := csv.NewReader(stdout)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		_ = cmd.Wait()
		return connect.NewError(connect.CodeInternal, fmt.Errorf("read header: %w", err))
	}
	colIdx := makeColIndex(header)

	var batch []*pb.WikidataStudioResult
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		batch = append(batch, &pb.WikidataStudioResult{
			StudioId:         getInt32(record, colIdx, "studio_id"),
			WikidataQid:      getStr(record, colIdx, "wikidata_qid"),
			Method:           getStr(record, colIdx, "method"),
			WikipediaEn:      getStr(record, colIdx, "wikipedia_en"),
			WikipediaJa:      getStr(record, colIdx, "wikipedia_ja"),
			WebsiteUrl:       getStr(record, colIdx, "website_url"),
			TwitterHandle:    getStr(record, colIdx, "twitter_handle"),
			YoutubeChannelId: getStr(record, colIdx, "youtube_channel_id"),
		})

		if len(batch) >= 50 {
			_ = stream.Send(&pb.EnrichWikidataStudiosResponse{
				Payload: &pb.EnrichWikidataStudiosResponse_Results{
					Results: &pb.WikidataStudioResultBatch{Entries: batch},
				},
			})
			batch = nil
		}
	}

	if len(batch) > 0 {
		_ = stream.Send(&pb.EnrichWikidataStudiosResponse{
			Payload: &pb.EnrichWikidataStudiosResponse_Results{
				Results: &pb.WikidataStudioResultBatch{Entries: batch},
			},
		})
	}

	return cmd.Wait()
}

// ScrapeWikipediaStudioContent extracts Wikipedia content for studios.
func (s *Service) ScrapeWikipediaStudioContent(
	ctx context.Context,
	stream *connect.BidiStream[pb.ScrapeWikipediaStudioContentRequest, pb.ScrapeWikipediaStudioContentResponse],
) error {
	startTime := time.Now()

	var entries []*pb.WikipediaStudioInput
	for {
		msg, err := stream.Receive()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entries = append(entries, msg.GetEntries()...)
	}

	if len(entries) == 0 {
		return nil
	}

	binaryPath := findBinary("wikipedia_studio_content")
	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Env = os.Environ()

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start: %w", err))
	}

	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			_ = stream.Send(&pb.ScrapeWikipediaStudioContentResponse{
				Payload: &pb.ScrapeWikipediaStudioContentResponse_Progress{
					Progress: &pb.ProgressUpdate{
						Phase:       "scraping_studio_wikipedia",
						Message:     scanner.Text(),
						ElapsedSecs: time.Since(startTime).Seconds(),
					},
				},
			})
		}
	}()

	go func() {
		fmt.Fprintln(stdin, "studio_id,wikipedia_en")
		for _, e := range entries {
			fmt.Fprintf(stdin, "%d,%s\n", e.StudioId, csvEscape(e.WikipediaEn))
		}
		stdin.Close()
	}()

	reader := csv.NewReader(stdout)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		_ = cmd.Wait()
		return connect.NewError(connect.CodeInternal, fmt.Errorf("read header: %w", err))
	}
	colIdx := makeColIndex(header)

	var batch []*pb.WikipediaStudioContentResult
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		batch = append(batch, &pb.WikipediaStudioContentResult{
			StudioId:    getInt32(record, colIdx, "studio_id"),
			WikipediaUrl: getStr(record, colIdx, "wikipedia_url"),
			ContentHtml: getStr(record, colIdx, "content_html"),
		})

		if len(batch) >= 50 {
			_ = stream.Send(&pb.ScrapeWikipediaStudioContentResponse{
				Payload: &pb.ScrapeWikipediaStudioContentResponse_Results{
					Results: &pb.WikipediaStudioContentBatch{Entries: batch},
				},
			})
			batch = nil
		}
	}

	if len(batch) > 0 {
		_ = stream.Send(&pb.ScrapeWikipediaStudioContentResponse{
			Payload: &pb.ScrapeWikipediaStudioContentResponse_Results{
				Results: &pb.WikipediaStudioContentBatch{Entries: batch},
			},
		})
	}

	return cmd.Wait()
}

// --- Helpers ---

func findBinary(name string) string {
	candidates := []string{
		"/app/backend/" + name,
		"./" + name,
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return name
}

func makeColIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[h] = i
	}
	return m
}

func getStr(record []string, colIdx map[string]int, col string) string {
	if idx, ok := colIdx[col]; ok && idx < len(record) {
		return record[idx]
	}
	return ""
}

func getInt32(record []string, colIdx map[string]int, col string) int32 {
	s := getStr(record, colIdx, col)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return int32(v)
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
