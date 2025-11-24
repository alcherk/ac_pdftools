package pdf

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// CLI operation timeout constants
const (
	DefaultCLITimeout = 30 * time.Second
	AnalysisTimeout   = 60 * time.Second // Longer timeout for analysis operations
)

// execCommandWithTimeout executes a command with a timeout
func execCommandWithTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", timeout)
	}

	if err != nil {
		return output, fmt.Errorf("command failed: %v", err)
	}

	return output, nil
}

