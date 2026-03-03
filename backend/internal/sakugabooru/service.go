package sakugabooru

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

// Service implements the SakugabooruService ConnectRPC handler.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

// MatchTags downloads Sakugabooru tags and matches them to anime/staff.
func (s *Service) MatchTags(
	ctx context.Context,
	req *connect.Request[pb.MatchTagsRequest],
	stream *connect.ServerStream[pb.MatchTagsResponse],
) error {
	startTime := time.Now()

	// Create temp dir for I/O files.
	tmpDir, err := os.MkdirTemp("", "sakuga-match-*")
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("temp dir: %w", err))
	}
	defer os.RemoveAll(tmpDir)

	// Write anime input CSV.
	animeFile := filepath.Join(tmpDir, "anime_input.csv")
	if err := writeAnimeInputCSV(animeFile, req.Msg.GetAnime()); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("write anime CSV: %w", err))
	}

	// Write staff input CSV.
	staffFile := filepath.Join(tmpDir, "staff_input.csv")
	if err := writeStaffInputCSV(staffFile, req.Msg.GetStaff()); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("write staff CSV: %w", err))
	}

	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0755)

	// Run sakugabooru_match binary.
	binaryPath := findBinary("sakugabooru_match")
	cmd := exec.CommandContext(ctx, binaryPath,
		fmt.Sprintf("-anime=%s", animeFile),
		fmt.Sprintf("-staff=%s", staffFile),
		fmt.Sprintf("-out=%s", outDir),
	)

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start: %w", err))
	}

	// Stream stderr as progress.
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			_ = stream.Send(&pb.MatchTagsResponse{
				Payload: &pb.MatchTagsResponse_Progress{
					Progress: &pb.ProgressUpdate{
						Phase:       "matching",
						Message:     scanner.Text(),
						ElapsedSecs: time.Since(startTime).Seconds(),
					},
				},
			})
		}
	}()

	if err := cmd.Wait(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("sakugabooru_match exited: %w", err))
	}

	// Read anime matches.
	animeMatches, err := readAnimeMatches(filepath.Join(outDir, "anime_matches.csv"))
	if err != nil {
		log.Printf("warning: anime matches read error: %v", err)
	}

	// Read staff matches.
	staffMatches, err := readStaffMatches(filepath.Join(outDir, "staff_matches.csv"))
	if err != nil {
		log.Printf("warning: staff matches read error: %v", err)
	}

	// Send results batch.
	if err := stream.Send(&pb.MatchTagsResponse{
		Payload: &pb.MatchTagsResponse_Results{
			Results: &pb.MatchTagsBatch{
				AnimeMatches: animeMatches,
				StaffMatches: staffMatches,
			},
		},
	}); err != nil {
		return err
	}

	// Count stats.
	animeFound, animeUnmatched := 0, 0
	for _, m := range animeMatches {
		if m.Found {
			animeFound++
		} else {
			animeUnmatched++
		}
	}
	staffFound, staffUnmatched := 0, 0
	for _, m := range staffMatches {
		if m.Found {
			staffFound++
		} else {
			staffUnmatched++
		}
	}

	return stream.Send(&pb.MatchTagsResponse{
		Payload: &pb.MatchTagsResponse_Complete{
			Complete: &pb.MatchTagsComplete{
				AnimeMatched:   int32(animeFound),
				AnimeUnmatched: int32(animeUnmatched),
				StaffMatched:   int32(staffFound),
				StaffUnmatched: int32(staffUnmatched),
				DurationSecs:   time.Since(startTime).Seconds(),
			},
		},
	})
}

// FetchPosts fetches Sakugabooru posts for matched tags.
func (s *Service) FetchPosts(
	ctx context.Context,
	req *connect.Request[pb.FetchPostsRequest],
	stream *connect.ServerStream[pb.FetchPostsResponse],
) error {
	startTime := time.Now()

	tmpDir, err := os.MkdirTemp("", "sakuga-posts-*")
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("temp dir: %w", err))
	}
	defer os.RemoveAll(tmpDir)

	// Write anime tags input.
	animeTagsFile := filepath.Join(tmpDir, "anime_tags.csv")
	writeAnimeTagsCSV(animeTagsFile, req.Msg.GetAnimeTags())

	// Write staff tags input.
	staffTagsFile := filepath.Join(tmpDir, "staff_tags.csv")
	writeStaffTagsCSV(staffTagsFile, req.Msg.GetStaffTags())

	outDir := filepath.Join(tmpDir, "out")
	os.MkdirAll(outDir, 0755)

	binaryPath := findBinary("sakugabooru_posts")
	cmd := exec.CommandContext(ctx, binaryPath,
		fmt.Sprintf("-anime=%s", animeTagsFile),
		fmt.Sprintf("-staff=%s", staffTagsFile),
		fmt.Sprintf("-out=%s", outDir),
	)

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start: %w", err))
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			_ = stream.Send(&pb.FetchPostsResponse{
				Payload: &pb.FetchPostsResponse_Progress{
					Progress: &pb.ProgressUpdate{
						Phase:       "fetching_posts",
						Message:     scanner.Text(),
						ElapsedSecs: time.Since(startTime).Seconds(),
					},
				},
			})
		}
	}()

	if err := cmd.Wait(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("sakugabooru_posts exited: %w", err))
	}

	// Read output CSVs.
	posts, _ := readPostsCSV(filepath.Join(outDir, "sakugabooru_posts.csv"))
	animeLinks, _ := readAnimePostLinks(filepath.Join(outDir, "sakugabooru_anime_posts.csv"))
	staffLinks, _ := readStaffPostLinks(filepath.Join(outDir, "sakugabooru_staff_posts.csv"))

	if err := stream.Send(&pb.FetchPostsResponse{
		Payload: &pb.FetchPostsResponse_Results{
			Results: &pb.FetchPostsBatch{
				Posts:      posts,
				AnimeLinks: animeLinks,
				StaffLinks: staffLinks,
			},
		},
	}); err != nil {
		return err
	}

	return stream.Send(&pb.FetchPostsResponse{
		Payload: &pb.FetchPostsResponse_Complete{
			Complete: &pb.FetchPostsComplete{
				TotalPosts:   int32(len(posts)),
				AnimeLinks:   int32(len(animeLinks)),
				StaffLinks:   int32(len(staffLinks)),
				DurationSecs: time.Since(startTime).Seconds(),
			},
		},
	})
}

// --- File I/O helpers ---

func writeAnimeInputCSV(path string, entries []*pb.AnimeMatchInput) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"anilist_id", "title_english", "title_romaji", "synonyms"})
	for _, e := range entries {
		w.Write([]string{
			fmt.Sprintf("%d", e.AnilistId),
			e.TitleEnglish,
			e.TitleRomaji,
			strings.Join(e.Synonyms, "|"),
		})
	}
	w.Flush()
	return w.Error()
}

func writeStaffInputCSV(path string, entries []*pb.StaffMatchInput) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"staff_id", "name_en", "name_ja", "alternative_names"})
	for _, e := range entries {
		w.Write([]string{
			fmt.Sprintf("%d", e.StaffId),
			e.NameEn,
			e.NameJa,
			strings.Join(e.AlternativeNames, "|"),
		})
	}
	w.Flush()
	return w.Error()
}

func writeAnimeTagsCSV(path string, tags []*pb.AnimeTagInput) {
	f, _ := os.Create(path)
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"anilist_id", "sakugabooru_tag"})
	for _, t := range tags {
		w.Write([]string{fmt.Sprintf("%d", t.AnilistId), t.SakugabooruTag})
	}
	w.Flush()
}

func writeStaffTagsCSV(path string, tags []*pb.StaffTagInput) {
	f, _ := os.Create(path)
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"staff_id", "sakugabooru_tag"})
	for _, t := range tags {
		w.Write([]string{fmt.Sprintf("%d", t.StaffId), t.SakugabooruTag})
	}
	w.Flush()
}

func readAnimeMatches(path string) ([]*pb.AnimeMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	colIdx := makeColIndex(header)

	var matches []*pb.AnimeMatch
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		matches = append(matches, &pb.AnimeMatch{
			AnilistId:      getInt32(record, colIdx, "anilist_id"),
			SakugabooruTag: getStr(record, colIdx, "sakugabooru_tag"),
			PostCount:      getInt32(record, colIdx, "post_count"),
			Found:          getStr(record, colIdx, "found") == "1",
			Method:         getStr(record, colIdx, "method"),
		})
	}
	return matches, nil
}

func readStaffMatches(path string) ([]*pb.StaffMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	colIdx := makeColIndex(header)

	var matches []*pb.StaffMatch
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		matches = append(matches, &pb.StaffMatch{
			StaffId:        getInt32(record, colIdx, "staff_id"),
			SakugabooruTag: getStr(record, colIdx, "sakugabooru_tag"),
			PostCount:      getInt32(record, colIdx, "post_count"),
			TagType:        getStr(record, colIdx, "tag_type"),
			Found:          getStr(record, colIdx, "found") == "1",
			Method:         getStr(record, colIdx, "method"),
		})
	}
	return matches, nil
}

func readPostsCSV(path string) ([]*pb.SakugabooruPost, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	colIdx := makeColIndex(header)

	var posts []*pb.SakugabooruPost
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		posts = append(posts, &pb.SakugabooruPost{
			PostId:     getInt32(record, colIdx, "post_id"),
			FileUrl:    getStr(record, colIdx, "file_url"),
			PreviewUrl: getStr(record, colIdx, "preview_url"),
			Source:     getStr(record, colIdx, "source"),
			Rating:     getStr(record, colIdx, "rating"),
			FileExt:    getStr(record, colIdx, "file_ext"),
			Score:      getInt32(record, colIdx, "score"),
			CreatedAt:  getStr(record, colIdx, "created_at"),
		})
	}
	return posts, nil
}

func readAnimePostLinks(path string) ([]*pb.AnimePostLink, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	reader.Read() // skip header

	var links []*pb.AnimePostLink
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			continue
		}
		anilistID, _ := strconv.Atoi(record[0])
		postID, _ := strconv.Atoi(record[1])
		links = append(links, &pb.AnimePostLink{
			AnilistId: int32(anilistID),
			PostId:    int32(postID),
		})
	}
	return links, nil
}

func readStaffPostLinks(path string) ([]*pb.StaffPostLink, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	reader.Read() // skip header

	var links []*pb.StaffPostLink
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			continue
		}
		staffID, _ := strconv.Atoi(record[0])
		postID, _ := strconv.Atoi(record[1])
		links = append(links, &pb.StaffPostLink{
			StaffId: int32(staffID),
			PostId:  int32(postID),
		})
	}
	return links, nil
}

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
