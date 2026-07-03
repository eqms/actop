# actop — Design (2026-07-03)

A ctop-like terminal UI for Apple Container (`container` CLI v1.0.0, macOS 26, Apple silicon).
Approved design; implementation follows this spec.

## Goal

Real-time monitoring and lifecycle control of Apple containers in a compact terminal grid,
mirroring the ctop experience. Apple Container exposes no Docker socket, no event stream,
no pause/health — data comes from polling the CLI as JSON.

## Data source (verified on this machine)

- `container ls -a --format json` — full configuration **including status** (state,
  startedDate, networks with IP) and `initProcess.environment` (contains plaintext
  secrets → redaction from day one).
- `container stats --no-stream --format json` — cumulative counters: `cpuUsageUsec`,
  `memoryUsageBytes/LimitBytes`, `networkRx/TxBytes`, `blockRead/WriteBytes`, `numProcesses`.
- One poll tick = exactly 2 subprocess calls (ls + stats), independent of container count.
  `container inspect` only on demand for the detail view.
- Lifecycle: start/stop/kill/delete; `logs --follow` (no `--tail`); `exec -it`;
  `container system status` for API-server diagnostics.

## Stack

Go 1.26 + Charm: Bubble Tea (Elm architecture), Bubbles, Lipgloss; BurntSushi/toml for config.
actop doubles as the proving ground for the planned ctop Bubble Tea migration.
Concepts from ctop (reconnect loop, column registry, shell whitelist, redaction pattern)
are re-implemented, no code copied.

## Architecture

```
cli/      thin exec.Command("container", ...) wrapper + JSON DTOs + error mapping + mock
metrics/  delta engine: rates from cumulative counters (per-ID prev cache)
redact/   env value masking (secret-like keys + ://user:pass@ values)
model/    store, domain container type, sort, filter — no UI dependency
config/   TOML at ~/.config/actop/config.toml (poll interval, sort, shell, toggles)
ui/       Bubble Tea root model + screen stack (grid base; detail/logs/confirm pushed)
```

Rules: `cli` knows nothing about Bubble Tea; `ui` only consumes the `cli.Client`
interface (mockable); CLI calls never run synchronously in `Update` — always `tea.Cmd`.

## Key mechanics

- **Polling**: `tea.Tick` (default 2s) → `tea.Batch(fetchList, fetchStats, nextTick)`.
  Real `dt` from response timestamps.
- **Metrics**: first tick per container shows no rates; negative deltas (counter reset)
  clamp to 0; nCPU from `configuration.resources.cpus`, fallback host cores; prev cache
  pruned 60s after a container disappears.
- **API server down**: stderr pattern → header banner, exponential backoff 2s→30s,
  grid keeps last data marked stale, `r` retries immediately.
- **Exec shell**: `tea.ExecProcess` with shell whitelist (fish→zsh→bash→sh fallback).
- **Logs**: in-TUI streaming (StdoutPipe → scanner goroutine → channel → msg-per-line),
  2000-line ring buffer in a viewport; subprocess killed on screen exit.
- **Security**: exec.Command with arg slices only (never `sh -c`); container IDs come
  only from parsed ls output; filter input never reaches exec; env redacted in detail
  view; fixtures anonymized before commit.

## Keybindings

q quit/close · h/? help · s sort cycle · f filter · enter detail · e exec shell ·
l logs · S start · X stop · K kill · D delete (confirm) · a all containers ·
r reverse sort / retry · j/k navigate

## Milestones

- **v0.1.0**: grid (name, state, cpu%, mem, net rate, io rate, pids, uptime, ip),
  sort/filter, actions with confirm, logs, exec, detail view with redacted env,
  header with API status, TOML config, tests (metrics/redact/parsing/model with mock).
- **v0.2** (roadmap): volumes/images/system tabs, column toggle menu, sparklines
  (ring buffer placeholder exists in v0.1), browser-open for published ports.
