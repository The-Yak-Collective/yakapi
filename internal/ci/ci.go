package ci

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"go.uber.org/zap"
)

func Accept(ctx context.Context, log *zap.SugaredLogger, runner string, r io.Reader) (string, error) {
	cmd := exec.CommandContext(ctx, runner)
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	// put stdout in a buffer and log it
	stdout, err := cmd.StdoutPipe()

	log.Infow("running ci adapter", "runner", runner)

	err = cmd.Start()
	if err != nil {
		return "", fmt.Errorf("failed to start ci adapter: %w", err)
	}

	output, err := io.ReadAll(stdout)
	if err != nil {
		return "", fmt.Errorf("failed to read ci adapter output: %w", err)
	}

	err = cmd.Wait()
	if err != nil {
		return "", fmt.Errorf("failed to wait for ci adapter: %w", err)
	}

	log.Infow("complete", "runner", runner, "exit", cmd.ProcessState.ExitCode(), "output_size", len(output))

	return string(output), nil
}
