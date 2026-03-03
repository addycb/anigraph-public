package studio

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"connectrpc.com/connect"

	pb "anigraph/backend/gen/anigraph/v1"
)

// Service implements the StudioService ConnectRPC handler.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) FetchStudioImages(
	ctx context.Context,
	req *connect.Request[pb.FetchStudioImagesRequest],
	stream *connect.ServerStream[pb.FetchStudioImagesResponse],
) error {
	startTime := time.Now()
	studios := req.Msg.GetStudios()

	if len(studios) == 0 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("no studios provided"))
	}

	tmpDir, err := os.MkdirTemp("", "studio-images-*")
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("temp dir: %w", err))
	}
	defer os.RemoveAll(tmpDir)

	// Write input CSV.
	inputPath := filepath.Join(tmpDir, "input.csv")
	f, err := os.Create(inputPath)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("create input CSV: %w", err))
	}
	w := csv.NewWriter(f)
	w.Write([]string{"studio_name", "mal_anime_id_1", "mal_anime_id_2", "mal_anime_id_3"})
	for _, studio := range studios {
		row := []string{studio.StudioName}
		for i := 0; i < 3; i++ {
			if i < len(studio.MalAnimeIds) {
				row = append(row, fmt.Sprintf("%d", studio.MalAnimeIds[i]))
			} else {
				row = append(row, "")
			}
		}
		w.Write(row)
	}
	w.Flush()
	f.Close()

	outputPath := filepath.Join(tmpDir, "output.csv")

	binaryPath := findBinary("fetch_studio_images")
	cmd := exec.CommandContext(ctx, binaryPath,
		fmt.Sprintf("-input=%s", inputPath),
		fmt.Sprintf("-output=%s", outputPath),
	)

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start: %w", err))
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			_ = stream.Send(&pb.FetchStudioImagesResponse{
				Payload: &pb.FetchStudioImagesResponse_Progress{
					Progress: &pb.ProgressUpdate{
						Phase:       "fetching_images",
						Message:     scanner.Text(),
						ElapsedSecs: time.Since(startTime).Seconds(),
					},
				},
			})
		}
	}()

	if err := cmd.Wait(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("fetch_studio_images exited: %w", err))
	}

	// Read output CSV.
	results, err := readStudioResults(outputPath)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("read output: %w", err))
	}

	if err := stream.Send(&pb.FetchStudioImagesResponse{
		Payload: &pb.FetchStudioImagesResponse_Results{
			Results: &pb.StudioImageBatch{Entries: results},
		},
	}); err != nil {
		return err
	}

	fetched, failed := 0, 0
	for _, r := range results {
		if r.ImageUrl != "" {
			fetched++
		} else {
			failed++
		}
	}

	return stream.Send(&pb.FetchStudioImagesResponse{
		Payload: &pb.FetchStudioImagesResponse_Complete{
			Complete: &pb.FetchStudioImagesComplete{
				Total:        int32(len(studios)),
				Fetched:      int32(fetched),
				Failed:       int32(failed),
				DurationSecs: time.Since(startTime).Seconds(),
			},
		},
	})
}

func readStudioResults(path string) ([]*pb.StudioImageResult, error) {
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

	var results []*pb.StudioImageResult
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		results = append(results, &pb.StudioImageResult{
			StudioName:  getStr(record, colIdx, "studio_name"),
			MalId:       getInt32(record, colIdx, "mal_id"),
			ImageUrl:    getStr(record, colIdx, "image_url"),
			Description: getStr(record, colIdx, "description"),
		})
	}
	return results, nil
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

