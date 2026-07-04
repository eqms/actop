# Release Notes

> **Language / Sprache**: [Deutsch](#deutsche-release-notes) | [English](#english-release-notes)

---

## Deutsche Release Notes

### v0.4.0 (2026-07-04)

#### Neue Features
- **Container-Aktionsmenü** (`enter`, ctop-Stil): zentriertes Panel mit zustandsabhängigen Aktionen — laufend: Details, Logs, Shell, Open URL (falls Port/IP), Stop, Kill, Delete; gestoppt: Details, Logs, Start, Delete. Navigation mit `j/k/↑/↓`, Auswahl per `enter` oder Aktions-Hotkey, `esc` schließt. Destruktive Aktionen behalten den Bestätigungsdialog; die direkten Grid-Hotkeys bleiben erhalten.

---

### v0.3.1 (2026-07-04)

#### Sicherheit
- **Terminal-Escape-Filter für Container-Logs**: Log-Zeilen werden vor dem Rendern von ANSI-/OSC-Sequenzen und Steuerzeichen befreit (neues `redact.Line`); verhindert Clipboard-Manipulation (OSC 52), Titel-Spoofing und Terminal-Abfragen durch bösartige Container-Ausgaben
- **Erweiterte Secret-Redaction**: Env-Variablen wie `DB_PASS`, `MYSQL_PWD`, `STRIPE_KEY`, `SIGNING_KEY`, `KEY_FILE` werden jetzt ebenfalls maskiert (tokenbasierte `PASS`/`PWD`/`KEY`-Erkennung, `COMPASS_URL`/`KEYBOARD_LAYOUT` bleiben sichtbar)
- **Argument-Injection-Härtung**: Alle Container-/Volume-/Image-Namen werden mit POSIX-`--`-Terminator an die `container`-CLI übergeben (inspect, start, stop, kill, delete, logs, exec, volume/image delete); Namen mit führendem `-` können nicht mehr als Flags interpretiert werden

---

### v0.3.0 (2026-07-04)

#### Neue Features
- **System-Tab** (`4`): API-Server-Status und -Version, App-Root, Ressourcen-Zähler (Container mit Running-Anteil, Volumes, Images), allokierte Volume-Gesamtgröße und Host-Infos
- **Spalten-Konfiguration**: `columns`-Liste in der config.toml bestimmt Auswahl und Reihenfolge der Grid-Spalten (`name` ist immer aktiv, unbekannte Keys werden ignoriert); Toggle-Screen per `c` für Session-Änderungen
- **Browser-Open** (`o`): öffnet die erste published TCP-Port-URL (`http://localhost:<hostPort>`, macOS-Port-Forwarding) bzw. als Fallback die Container-IP im Standard-Browser; published Ports werden zusätzlich in der Detail-Ansicht angezeigt

#### Intern
- `publishedPorts`-Schema gegen die echte CLI verifiziert (Wegwerf-Container mit `-p`), Fixture aktualisiert
- Spalten-Registry mit Keys (`name`…`ip`) statt fixer Liste — Grundlage für spätere Persistenz aus der TUI
- Neue Tests: View-Rendering aller Tabs, resolveColumns-Validierung, Spalten-Toggle, WebURL-Konstruktion (tcp/udp/ohne Ports), Ports-Fixture-Parsing

---

### v0.2.0 (2026-07-04)

#### Neue Features
- **Volumes-Tab** (`2`): Alle Named Volumes mit Format, Größe, Erstellungsdatum und Quellpfad; Löschen mit Rückfrage (`D`) — inkl. Warnung, dass Volume-Daten verloren gehen
- **Images-Tab** (`3`): Alle lokalen Images mit Referenz, Digest, Erstellungsdatum und Varianten-Anzahl; Löschen mit Rückfrage (`D`)
- **Tab-Navigation**: `tab` wechselt zyklisch, `1`–`3` springt direkt; Tab-Leiste im Header; Tab-Tasten werden während der Filter-Eingabe nicht abgefangen
- **Sparklines**: Neue `HIST`-Spalte im Container-Grid (CPU-Verlauf, 12 Samples); Detail-Ansicht zeigt CPU- und Speicher-Verlauf der letzten ~2 Minuten (60 Samples, Block-Glyphen `▁▂▃▄▅▆▇█`)

#### Intern
- `metrics.RingBuffer` (fixe Größe, Wrap-around) mit Tests; History übersteht Listen-Refreshes
- Runen-bewusstes Spalten-Padding (Sparkline-Glyphen und Umlaute sind Multi-Byte)
- Generischer Confirm-Dialog (ein Screen für Container-/Volume-/Image-Aktionen)
- Neue Fixtures (`volumes.json`, `images.json`) und UI-Tests für Tab-Wechsel, Filter-Schutz und Volume-Delete

---

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

### v0.4.0 (2026-07-04)

#### Added
- **Container action menu** (`enter`, ctop-style): centered panel with state-filtered actions — running: Details, Logs, Shell, Open URL (if port/IP), Stop, Kill, Delete; stopped: Details, Logs, Start, Delete. Navigate with `j/k/↑/↓`, select via `enter` or the action hotkey, `esc` closes. Destructive actions keep the confirmation dialog; direct grid hotkeys remain available.

---

### v0.3.1 (2026-07-04)

#### Security
- **Terminal escape filtering for container logs**: log lines are stripped of ANSI/OSC sequences and control characters before rendering (new `redact.Line`); prevents clipboard manipulation (OSC 52), title spoofing and terminal queries from malicious container output
- **Extended secret redaction**: env variables such as `DB_PASS`, `MYSQL_PWD`, `STRIPE_KEY`, `SIGNING_KEY`, `KEY_FILE` are now masked as well (token-based `PASS`/`PWD`/`KEY` matching; `COMPASS_URL`/`KEYBOARD_LAYOUT` stay visible)
- **Argument injection hardening**: all container/volume/image names are passed to the `container` CLI behind the POSIX `--` terminator (inspect, start, stop, kill, delete, logs, exec, volume/image delete); names starting with `-` can no longer be parsed as flags

---

### v0.3.0 (2026-07-04)

#### Added
- **System tab** (`4`): API server status and version, app root, resource counters (containers with running share, volumes, images), total allocated volume size and host info
- **Column configuration**: the `columns` list in config.toml controls selection and order of grid columns (`name` is always on, unknown keys are ignored); toggle screen via `c` for session changes
- **Browser-open** (`o`): opens the first published TCP port URL (`http://localhost:<hostPort>`, macOS port forwarding) or, as fallback, the container IP in the default browser; published ports are also shown in the detail view

#### Internal
- `publishedPorts` schema verified against the real CLI (throwaway container with `-p`), fixture refreshed
- Column registry keyed by name (`name`…`ip`) instead of a fixed list — groundwork for future in-TUI persistence
- New tests: view rendering of all tabs, resolveColumns validation, column toggle, WebURL construction (tcp/udp/no ports), ports fixture parsing

---

### v0.2.0 (2026-07-04)

#### Added
- **Volumes tab** (`2`): all named volumes with format, size, creation date and source path; delete with confirmation (`D`) — including a warning that volume data is lost
- **Images tab** (`3`): all local images with reference, digest, creation date and variant count; delete with confirmation (`D`)
- **Tab navigation**: `tab` cycles, `1`–`3` jumps directly; tab bar in the header; tab keys are not hijacked while typing in the filter
- **Sparklines**: new `HIST` column in the container grid (CPU trend, 12 samples); the detail view shows CPU and memory history of the last ~2 minutes (60 samples, block glyphs `▁▂▃▄▅▆▇█`)

#### Internal
- `metrics.RingBuffer` (fixed size, wrap-around) with tests; history survives list refreshes
- Rune-aware column padding (sparkline glyphs and umlauts are multi-byte)
- Generic confirm dialog (one screen for container/volume/image actions)
- New fixtures (`volumes.json`, `images.json`) and UI tests for tab switching, filter guarding and volume delete

---

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
