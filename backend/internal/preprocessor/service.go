package preprocessor

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

// Service implements the PreprocessorService ConnectRPC handler.
type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) PreprocessData(
	ctx context.Context,
	req *connect.Request[pb.PreprocessDataRequest],
	stream *connect.ServerStream[pb.PreprocessDataResponse],
) error {
	dataDir := req.Msg.GetDataDir()
	if dataDir == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("data_dir is required"))
	}

	startTime := time.Now()

	if err := stream.Send(&pb.PreprocessDataResponse{
		Payload: &pb.PreprocessDataResponse_Progress{
			Progress: &pb.ProgressUpdate{
				Phase:   "starting",
				Message: fmt.Sprintf("Preprocessing CSVs in %s", dataDir),
			},
		},
	}); err != nil {
		return err
	}

	// Run the preprocessor binary.
	binaryPath := findBinary("preprocess_csv")
	cmd := exec.CommandContext(ctx, binaryPath, fmt.Sprintf("-dir=%s", dataDir))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("stdout pipe: %w", err))
	}

	if err := cmd.Start(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("start preprocessor: %w", err))
	}

	// Stream stdout lines as progress.
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if err := stream.Send(&pb.PreprocessDataResponse{
			Payload: &pb.PreprocessDataResponse_Progress{
				Progress: &pb.ProgressUpdate{
					Phase:       "preprocessing",
					Message:     line,
					ElapsedSecs: time.Since(startTime).Seconds(),
				},
			},
		}); err != nil {
			log.Printf("stream send error: %v", err)
		}
	}

	if err := cmd.Wait(); err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("preprocessor exited: %w", err))
	}

	duration := time.Since(startTime)
	return stream.Send(&pb.PreprocessDataResponse{
		Payload: &pb.PreprocessDataResponse_Complete{
			Complete: &pb.PreprocessComplete{
				DurationSecs: duration.Seconds(),
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
