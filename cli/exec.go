package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

const binary = "container"

// ExecClient runs the real container CLI.
type ExecClient struct{}

func New() *ExecClient { return &ExecClient{} }

// run executes the CLI and returns stdout. Errors are mapped to
// ErrAPIServerDown / ErrNotFound where stderr allows it.
func (c *ExecClient) run(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, binary, args...).Output()
	if err == nil {
		return out, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		stderr := string(ee.Stderr)
		if mapped := classifyStderr(stderr); mapped != nil {
			return nil, mapped
		}
		return nil, &CLIError{Args: args, Stderr: stderr, ExitCode: ee.ExitCode()}
	}
	// binary missing, context timeout, ...
	return nil, fmt.Errorf("container %v: %w", args, err)
}

func (c *ExecClient) List(ctx context.Context, all bool) ([]ListItem, error) {
	args := []string{"ls", "--format", "json"}
	if all {
		args = []string{"ls", "-a", "--format", "json"}
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var items []ListItem
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parse ls output: %w", err)
	}
	return items, nil
}

func (c *ExecClient) Stats(ctx context.Context) ([]StatsEntry, error) {
	out, err := c.run(ctx, "stats", "--no-stream", "--format", "json")
	if err != nil {
		return nil, err
	}
	var entries []StatsEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		return nil, fmt.Errorf("parse stats output: %w", err)
	}
	return entries, nil
}

func (c *ExecClient) Inspect(ctx context.Context, id string) (*ListItem, error) {
	out, err := c.run(ctx, "inspect", id)
	if err != nil {
		return nil, err
	}
	// inspect returns a single-element array
	var items []ListItem
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parse inspect output: %w", err)
	}
	if len(items) == 0 {
		return nil, ErrNotFound
	}
	return &items[0], nil
}

func (c *ExecClient) SystemStatus(ctx context.Context) (*SystemStatus, error) {
	out, err := c.run(ctx, "system", "status", "--format", "json")
	if err != nil {
		return nil, err
	}
	var st SystemStatus
	if err := json.Unmarshal(out, &st); err != nil {
		return nil, fmt.Errorf("parse system status: %w", err)
	}
	return &st, nil
}

func (c *ExecClient) Start(ctx context.Context, id string) error {
	_, err := c.run(ctx, "start", id)
	return err
}

func (c *ExecClient) Stop(ctx context.Context, id string) error {
	_, err := c.run(ctx, "stop", id)
	return err
}

func (c *ExecClient) Kill(ctx context.Context, id string) error {
	_, err := c.run(ctx, "kill", id)
	return err
}

func (c *ExecClient) Delete(ctx context.Context, id string) error {
	_, err := c.run(ctx, "delete", id)
	return err
}

func (c *ExecClient) LogsCmd(id string, follow bool) *exec.Cmd {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	args = append(args, id)
	return exec.Command(binary, args...)
}

func (c *ExecClient) ExecCmd(id string, shell ...string) *exec.Cmd {
	args := append([]string{"exec", "-it", id}, shell...)
	return exec.Command(binary, args...)
}
