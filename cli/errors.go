package cli

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrAPIServerDown indicates the container API server is not reachable;
	// the user needs `container system start`.
	ErrAPIServerDown = errors.New("container API server not running")
	// ErrNotFound indicates the referenced container does not exist.
	ErrNotFound = errors.New("container not found")
)

// CLIError wraps a non-zero exit of the container CLI with its stderr.
type CLIError struct {
	Args     []string
	Stderr   string
	ExitCode int
}

func (e *CLIError) Error() string {
	return fmt.Sprintf("container %s: exit %d: %s",
		strings.Join(e.Args, " "), e.ExitCode, strings.TrimSpace(e.Stderr))
}

// stderr fragments that identify a stopped/unreachable API server. The CLI
// wording varies between operations, so match loosely and case-insensitively.
var apiDownPatterns = []string{
	"xpc connection",
	"connection invalid",
	"connection interrupted",
	"system is not running",
	"container system start",
	"apiserver",
}

var notFoundPatterns = []string{
	"not found",
	"no such container",
	"does not exist",
}

func classifyStderr(stderr string) error {
	s := strings.ToLower(stderr)
	for _, p := range apiDownPatterns {
		if strings.Contains(s, p) {
			return ErrAPIServerDown
		}
	}
	for _, p := range notFoundPatterns {
		if strings.Contains(s, p) {
			return ErrNotFound
		}
	}
	return nil
}
