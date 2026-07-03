package ui

import (
	"time"

	"github.com/eqms/actop/cli"
)

type tickMsg time.Time

type listResultMsg struct {
	items []cli.ListItem
	err   error
}

type statsResultMsg struct {
	entries []cli.StatsEntry
	ts      time.Time
	err     error
}

type systemStatusMsg struct {
	status *cli.SystemStatus
	err    error
}

type actionDoneMsg struct {
	action string
	id     string
	err    error
}

type execFinishedMsg struct{ err error }

type logLineMsg struct{ line string }

type logStreamEndMsg struct{ err error }
