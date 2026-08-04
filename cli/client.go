// Package cli is a thin wrapper around Apple's `container` command line
// tool. It shells out with --format json and maps results to Go types.
// It has no dependency on the UI layer.
package cli

import (
	"context"
	"os/exec"
)

type Client interface {
	// List returns all containers; with all=true stopped ones are included.
	List(ctx context.Context, all bool) ([]ListItem, error)
	// Stats returns resource counters for all running containers.
	Stats(ctx context.Context) ([]StatsEntry, error)
	// Inspect returns full details for a single container.
	Inspect(ctx context.Context, id string) (*ListItem, error)
	// SystemStatus reports whether the container API server is running.
	SystemStatus(ctx context.Context) (*SystemStatus, error)
	// Version reports the container CLI version, e.g. "1.2.0". Unlike
	// SystemStatus it also answers while the API server is stopped.
	Version(ctx context.Context) (string, error)
	// Volumes returns all named volumes.
	Volumes(ctx context.Context) ([]Volume, error)
	// Images returns all local images.
	Images(ctx context.Context) ([]Image, error)

	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string) error
	Kill(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	VolumeDelete(ctx context.Context, name string) error
	ImageDelete(ctx context.Context, ref string) error

	// LogsCmd builds the (not yet started) subprocess for log streaming.
	LogsCmd(id string, follow bool) *exec.Cmd
	// ExecCmd builds the interactive shell subprocess for tea.ExecProcess.
	ExecCmd(id string, shell ...string) *exec.Cmd
}
