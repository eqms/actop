# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`actop` is a ctop-like terminal UI for [Apple Container](https://github.com/apple/container),
the macOS-native container runtime. Go, Bubble Tea v2, macOS-only (CI builds
darwin/arm64 and darwin/amd64 exclusively).

Apple Container has no Docker socket, no event stream and no pause/health
support, so actop deliberately polls the `container` CLI as JSON — a constant
two subprocess calls per tick (default 2 s). Any feature request that assumes
an event-driven runtime has to be re-thought as polling.

## Commands

```bash
make build            # CGO_ENABLED=0, version/build stamped from VERSION + git
make test             # go test -race ./...
make cover            # race + coverage profile, prints the total
make lint             # golangci-lint (standard preset + gosec + gocritic)
make fixtures         # re-record cli/testdata/*.json from live containers
make clean

go test ./ui/ -run TestProgramRendersAndQuits -v      # single test
go test ./ui/ -run 'Escape|Columns' -race             # pattern
go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./... # as CI runs it
```

The test suite needs no `container` binary and no running API server:
`cli.MockClient` covers the UI, and `ExecClient` tests point the unexported
`binary` field at a generated shell script. `make fixtures` is the exception —
it talks to real containers and its output must be anonymized before committing.

## Architecture

Strictly layered, no upward dependencies:

```
cli/      subprocess + JSON → Go types. Knows nothing about the UI.
metrics/  cumulative counters → per-interval rates (Engine keeps the baseline)
model/    Store: containers merged from list+stats polls, sorting, filtering
ui/       Bubble Tea root model (App) + screen stack
upgrade/  `actop update-container`: stop service, Apple's updater, restart, report
```

`upgrade` is the only code that stops the container service
(`cli.Client.SystemStop`/`SystemStart`); the TUI never calls either. Its
rule: fail before the service stop, carry on after it — a failed or
interrupted update must still bring the service and containers back.

**Menus and grid keys follow the real ctop.** Menu entries show their key
(`[l] log view`) and fire on it; only actions valid for the current state are
listed; start/restart need no confirmation. The grid uses ctop's `o`/`→`,
`←`, `w`, `S` (save config), `H`; `X`/`K` are actop extras ctop leaves free,
and `D` is deliberately unbound on the containers tab. v0.5.0 got this wrong
("ctop-faithful" meant a fixed, dimmed list without hotkeys); v0.8.0 and
v0.9.0 fixed it on the user's request — check `menus.go` and `grid.go` in
the ctop checkout before changing either.

**Two different "model" concepts.** `App` is the Bubble Tea root model
(`Init/Update/View`). Everything visible is a `screen` — actop's own interface
in `ui/screen.go` with lowercase `update`/`view(width, height) string`. Screens
live on a stack: the tab is the permanent base at index 0, and detail, logs,
confirm, menu and help get pushed on top. A screen's `update` returning `nil`
pops it. Only `App` implements `tea.Model`; screens never do.

**Polling and state.** `App.Update` handles the tick, fans out fetch commands
and folds results into the store. `model.Store` and `metrics.Engine` are
deliberately lock-free because they are only ever touched from the update loop —
keep it that way; anything concurrent must go through a `tea.Msg`.

**Client seam.** `cli.Client` is the interface everything depends on;
`ExecClient` is the real one, `MockClient` the test double. `MockClient` is
mutex-guarded because concurrent Bubble Tea commands hit it simultaneously —
read recorded calls via `CallList()`, never a field.

## Invariants worth preserving

**Container data is untrusted.** Names, image references, volume sources, env
values and log lines are chosen by whoever built the image and can carry OSC 52
(clipboard write), title-set or query sequences the host terminal would execute.
Everything rendered goes through `safe()` — or through `pad()` and
`displayWidth()`, which apply it. When adding a render path that prints a string
originating from `cli`, route it through one of those. Confirmation prompts
sanitize before formatting, so an escape-padded name cannot obscure what is
about to be deleted.

**Subprocesses never see a shell.** Arguments are passed as arrays, and
caller-supplied names go behind the POSIX `--` terminator. The exec shell comes
from `config.ShellAllowed`, the only validation layer in front of `exec` — keep
it tested. `gosec` G204 findings at these sites carry `#nosec` comments stating
why; new ones need the same justification, not a blanket exclusion.

**Bounded subprocess I/O.** `ExecClient.run` caps stdout at 64 MiB and stderr at
64 KiB, and every command sets `WaitDelay`. Without it, a child that spawns its
own children keeps the inherited pipes open and `Wait` blocks forever — in the
log path that runs inside the update loop and freezes the UI.

## Bubble Tea v2 specifics

Migrated from v1 in v0.6.0; module paths are `charm.land/{bubbletea,bubbles,lipgloss}/v2`.
Points that bite silently, because the code compiles either way:

- Match `tea.KeyPressMsg`, not `tea.KeyMsg` — the latter is now an interface
  that also covers key *releases*, which would double-fire every binding.
- The space bar reports as `"space"`; a literal `case " "` never matches.
- `View()` returns `tea.View`, and terminal modes are view fields
  (`v.AltScreen = true`), not `tea.NewProgram` options. `App.Body()` exists so
  tests can assert on frame text without unwrapping a `tea.View`.
- `viewport.New()` takes no dimensions; use `SetWidth`/`SetHeight`.

`ultraviolet` is pinned to whatever bubbletea v2.0.8 selects. It is the
rendering backend — do not bump it independently.

## Conventions

- Version lives in `VERSION` (stamped into the binary via ldflags); bump it
  together with an entry in both the German and the English half of
  `RELEASE_NOTES.md`, dated ISO (`### v0.6.0 (2026-08-03)`) like the existing ones.
- Design specs and implementation plans go under `docs/superpowers/`.
- Tagging `v*` triggers release + Homebrew formula update in `eqms/homebrew-tap`.
