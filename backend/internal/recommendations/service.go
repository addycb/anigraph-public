package recommendations

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"connectrpc.com/connect"

	pb "anigraph/backend/gen/anigraph/v1"
)

// Service implements the RecommendationService ConnectRPC handler.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) ComputeRecommendations(
	ctx context.Context,
	req *connect.Request[pb.ComputeRecommendationsRequest],
	stream *connect.ServerStream[pb.ComputeRecommendationsResponse],
) error {
	dataDir := req.Msg.GetDataDir()
	outputDir := req.Msg.GetOutputDir()
	if outputDir == "" {
		outputDir = dataDir
	}
	incremental := req.Msg.GetIncremental()
	workers := req.Msg.GetWorkers()

	startTime := time.Now()

	if err := stream.Send(&pb.ComputeRecommendationsResponse{
		Payload: &pb.ComputeRecommendationsResponse_Progress{
			Progress: &pb.ProgressUpdate{
				Phase:   "starting",
				Message: fmt.Sprintf("Computing recommendations (incremental=%v)", incremental),
			},
		},
	}); err != nil {
		return err
	}

	// Build args.
	args := []string{
		fmt.Sprintf("-dir=%s", dataDir),
		fmt.Sprintf("-output=%s", outputDir),
	}
	if incremental {
		args = append(args, "-incremental")
	}
	if workers > 0 {
		args = append(args, fmt.Sprintf("-workers=%d", workers))
	}

	binaryPath := findBinary("compute_recommendations")
	cmd := exec.CommandContext(ctx, binaryPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("stdout pipe: %w", err))
	}

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start compute_recommendations: %w", err))
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if err := stream.Send(&pb.ComputeRecommendationsResponse{
			Payload: &pb.ComputeRecommendationsResponse_Progress{
				Progress: &pb.ProgressUpdate{
					Phase:       "computing",
					Message:     line,
					ElapsedSecs: time.Since(startTime).Seconds(),
				},
			},
		}); err != nil {
			log.Printf("stream send error: %v", err)
		}
	}

	if err := cmd.Wait(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("compute_recommendations exited: %w", err))
	}

	duration := time.Since(startTime)
	return stream.Send(&pb.ComputeRecommendationsResponse{
		Payload: &pb.ComputeRecommendationsResponse_Complete{
			Complete: &pb.RecommendationsComplete{
				DurationSecs: duration.Seconds(),
				BinaryPath:   fmt.Sprintf("%s/recommendations.bin", outputDir),
				CsvPath:      fmt.Sprintf("%s/recommendations.csv", outputDir),
			},
		},
	})
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
