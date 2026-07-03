# Release Notes

> **Language / Sprache**: [Deutsch](#deutsche-release-notes) | [English](#english-release-notes)

---

## Deutsche Release Notes

### v0.1.0 (2026-07-03)

Erstes Release — ctop-ähnliche Terminal-UI für Apple Container (macOS 26, Apple Silicon).

#### Neue Features
- **Kompakt-Grid** mit Live-Metriken: Name, State, CPU %, Speicher (Nutzung/Limit), Netzwerk- und Block-I/O-Raten, PIDs, Uptime, Container-IP — per JSON-Polling der `container`-CLI (konstant 2 Subprozess-Aufrufe pro Tick, Intervall konfigurierbar)
- **Lifecycle-Aktionen** mit Bestätigungsdialog: Start, Stop, Kill, Delete
- **Logs-Viewer** (`container logs --follow` als Stream in der TUI, 2000-Zeilen-Puffer)
- **Exec-Shell** in den Container (Auto-Erkennung fish→zsh→bash→sh, Shell-Whitelist)
- **Detail-Ansicht** mit Konfiguration und Env-Variablen — Credential-artige Werte und Connection-Strings werden als `[REDACTED]` maskiert
- **Sortieren** (8 Felder), **Filtern** (Name/Image), gestoppte Container ein-/ausblendbar
- **Verbindungs-Resilienz**: Erkennung „API-Server down" mit exponentiellem Backoff (2s→30s), Stale-Daten-Anzeige und manuellem Retry (`r`)
- **TOML-Konfiguration** unter `~/.config/actop/config.toml`

#### Intern
- Stack: Go 1.26 + Bubble Tea / Bubbles / Lipgloss (Charm)
- CI: GitHub Actions mit SHA-gepinnten Actions, blockierendem govulncheck, Dependabot (gomod/github-actions)
- Tests: Metrics-Delta-Engine, Redaction, JSON-Parsing gegen echte (anonymisierte) CLI-Fixtures, UI-Model-Tests mit Mock-Client

---

## English Release Notes

### v0.1.0 (2026-07-03)

Initial release — ctop-like terminal UI for Apple Container (macOS 26, Apple silicon).

#### Added
- **Compact grid** with live metrics: name, state, CPU %, memory (usage/limit), network and block I/O rates, PIDs, uptime, container IP — via JSON polling of the `container` CLI (a constant 2 subprocess calls per tick, configurable interval)
- **Lifecycle actions** with confirmation dialog: start, stop, kill, delete
- **Logs viewer** (`container logs --follow` streamed into the TUI, 2000-line buffer)
- **Exec shell** into the container (auto-detection fish→zsh→bash→sh, shell whitelist)
- **Detail view** with configuration and env variables — credential-like values and connection strings are masked as `[REDACTED]`
- **Sorting** (8 fields), **filtering** (name/image), toggle for stopped containers
- **Connection resilience**: "API server down" detection with exponential backoff (2s→30s), stale-data display and manual retry (`r`)
- **TOML configuration** at `~/.config/actop/config.toml`

#### Internal
- Stack: Go 1.26 + Bubble Tea / Bubbles / Lipgloss (Charm)
- CI: GitHub Actions with SHA-pinned actions, blocking govulncheck, Dependabot (gomod/github-actions)
- Tests: metrics delta engine, redaction, JSON parsing against real (anonymized) CLI fixtures, UI model tests with mock client
