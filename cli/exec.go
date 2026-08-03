package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

const defaultBinary = "container"

const (
	// defaultMaxOutput caps the JSON a single CLI call may produce. Labels,
	// env and process arguments come from whoever built the image, so an
	// oversized response has to fail the poll rather than grow the process
	// until the OS kills it.
	defaultMaxOutput = 64 << 20 // 64 MiB
	// maxStderr bounds the diagnostic text kept for error classification.
	maxStderr = 64 << 10 // 64 KiB
	// waitDelay bounds Wait once the process itself is done or killed. A
	// child that spawned its own children leaves them holding the inherited
	// pipes; without this, Wait blocks on those descendants indefinitely and
	// takes the calling goroutine -- for log sessions, the UI -- with it.
	waitDelay = time.Second
)

// ErrOutputTooLarge indicates a CLI call produced more output than allowed.
var ErrOutputTooLarge = errors.New("container CLI output exceeds limit")

// ExecClient runs the real container CLI. The zero value is usable; New
// exists for symmetry with the other constructors.
type ExecClient struct {
	// binary and maxOutput are overridden in tests; empty/zero means default.
	binary    string
	maxOutput int
}

func New() *ExecClient { return &ExecClient{} }

func (c *ExecClient) bin() string {
	if c.binary == "" {
		return defaultBinary
	}
	return c.binary
}

func (c *ExecClient) limit() int {
	if c.maxOutput <= 0 {
		return defaultMaxOutput
	}
	return c.maxOutput
}

// run executes the CLI and returns stdout, bounded by limit(). Errors are
// mapped to ErrAPIServerDown / ErrNotFound where stderr allows it.
func (c *ExecClient) run(ctx context.Context, args ...string) ([]byte, error) {
	// #nosec G204 -- fixed binary, args passed as an array (never a shell);
	// caller-supplied names are guarded by the POSIX "--" terminator below
	cmd := exec.CommandContext(ctx, c.bin(), args...)
	cmd.WaitDelay = waitDelay
	stderr := &capBuffer{cap: maxStderr}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("container %v: %w", args, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("container %v: %w", args, err)
	}

	// Read one byte past the cap so "exactly at the limit" stays valid and
	// only genuine overflow is rejected.
	limit := c.limit()
	out, readErr := io.ReadAll(io.LimitReader(stdout, int64(limit)+1))
	if readErr == nil && len(out) > limit {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("container %v: %w", args, ErrOutputTooLarge)
	}

	err = cmd.Wait()
	if readErr != nil {
		return nil, fmt.Errorf("container %v: read output: %w", args, readErr)
	}
	if err == nil {
		return out, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		// ee.Stderr is only populated by Output(); use the captured buffer
		s := stderr.String()
		if mapped := classifyStderr(s); mapped != nil {
			return nil, mapped
		}
		return nil, &CLIError{Args: args, Stderr: s, ExitCode: ee.ExitCode()}
	}
	// binary missing, context timeout, ...
	return nil, fmt.Errorf("container %v: %w", args, err)
}

// capBuffer keeps at most cap bytes and discards the rest, reporting every
// write as successful so a chatty process is truncated, not aborted.
type capBuffer struct {
	buf bytes.Buffer
	cap int
}

func (b *capBuffer) Write(p []byte) (int, error) {
	if room := b.cap - b.buf.Len(); room > 0 {
		if len(p) > room {
			b.buf.Write(p[:room])
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *capBuffer) String() string { return b.buf.String() }

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
	out, err := c.run(ctx, "inspect", "--", id)
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

func (c *ExecClient) Volumes(ctx context.Context) ([]Volume, error) {
	out, err := c.run(ctx, "volume", "ls", "--format", "json")
	if err != nil {
		return nil, err
	}
	var vols []Volume
	if err := json.Unmarshal(out, &vols); err != nil {
		return nil, fmt.Errorf("parse volume ls output: %w", err)
	}
	return vols, nil
}

func (c *ExecClient) Images(ctx context.Context) ([]Image, error) {
	out, err := c.run(ctx, "image", "ls", "--format", "json")
	if err != nil {
		return nil, err
	}
	var imgs []Image
	if err := json.Unmarshal(out, &imgs); err != nil {
		return nil, fmt.Errorf("parse image ls output: %w", err)
	}
	return imgs, nil
}

// Names and IDs are user-chosen and could start with "-"; the POSIX "--"
// terminator keeps the CLI from parsing them as flags.

func (c *ExecClient) VolumeDelete(ctx context.Context, name string) error {
	_, err := c.run(ctx, "volume", "delete", "--", name)
	return err
}

func (c *ExecClient) ImageDelete(ctx context.Context, ref string) error {
	_, err := c.run(ctx, "image", "delete", "--", ref)
	return err
}

func (c *ExecClient) Start(ctx context.Context, id string) error {
	_, err := c.run(ctx, "start", "--", id)
	return err
}

func (c *ExecClient) Stop(ctx context.Context, id string) error {
	_, err := c.run(ctx, "stop", "--", id)
	return err
}

func (c *ExecClient) Kill(ctx context.Context, id string) error {
	_, err := c.run(ctx, "kill", "--", id)
	return err
}

func (c *ExecClient) Delete(ctx context.Context, id string) error {
	_, err := c.run(ctx, "delete", "--", id)
	return err
}

func (c *ExecClient) LogsCmd(id string, follow bool) *exec.Cmd {
	args := []string{"logs"}
	if follow {
		args = append(args, "--follow")
	}
	args = append(args, "--", id)
	// #nosec G204 -- fixed binary, args as an array, id behind "--"
	cmd := exec.Command(c.bin(), args...)
	// the logs screen kills this from the update loop; see waitDelay
	cmd.WaitDelay = waitDelay
	return cmd
}

func (c *ExecClient) ExecCmd(id string, shell ...string) *exec.Cmd {
	args := append([]string{"exec", "-it", "--", id}, shell...)
	// #nosec G204 -- fixed binary, args as an array; shell comes from the
	// config allowlist (config.ShellAllowed) or a fixed probe string
	cmd := exec.Command(c.bin(), args...)
	cmd.WaitDelay = waitDelay
	return cmd
}
