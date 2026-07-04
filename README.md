# actop

> **Language / Sprache**: [Deutsch](#deutsch) | [English](#english)

A ctop-like terminal UI for [Apple Container](https://github.com/apple/container) —
real-time metrics and lifecycle control for the macOS-native container runtime.

Inspired by [ctop](https://github.com/bcicen/ctop) (MIT).

---

## Deutsch

### Was ist actop?

`actop` zeigt alle Apple-Container in einem kompakten Terminal-Grid mit
Live-Metriken (CPU %, Speicher, Netzwerk- und I/O-Raten, PIDs, Uptime, IP)
und steuert ihren Lebenszyklus — wie `ctop`, nur für Apples `container`
statt Docker.

Die Daten kommen per JSON-Polling aus der `container`-CLI (Standard: alle 2 s,
konstant zwei Subprozess-Aufrufe pro Tick). Apple Container bietet keinen
Docker-Socket, keine Events und kein pause/health — actop arbeitet deshalb
bewusst CLI-basiert.

### Voraussetzungen

- macOS 26 (Tahoe) auf Apple Silicon
- [`container`](https://github.com/apple/container) v1.0.0+ (`brew install container`)
- Laufender API-Server: `container system start`

### Installation

```bash
git clone https://github.com/eqms/actop && cd actop
make build
./actop
```

### Tasten

| Taste | Aktion |
|---|---|
| `tab` / `1`–`4` | Tabs: Containers / Volumes / Images / System |
| `enter` | Container-Details (inkl. Env, Secrets maskiert, CPU/Mem-Sparklines) |
| `l` | Logs (follow) |
| `e` | Shell im Container (fish→zsh→bash→sh) |
| `o` | Published Port (localhost:…) bzw. Container-IP im Browser öffnen |
| `c` | Grid-Spalten konfigurieren (Session; persistent via config.toml) |
| `S` / `X` / `K` / `D` | Start / Stop / Kill / Delete (mit Rückfrage) |
| `s` / `r` | Sortierfeld wechseln / Richtung umkehren |
| `f` | Filter (Name/Image) |
| `a` | Gestoppte Container ein-/ausblenden |
| `h` `?` | Hilfe |
| `q` | Beenden bzw. Ansicht schließen |

`r` startet bei getrennter Verbindung außerdem einen sofortigen Reconnect.

### Konfiguration

`~/.config/actop/config.toml` (alle Felder optional):

```toml
pollIntervalMs = 2000
sortField = "name"       # name|state|cpu|mem|net|io|pids|uptime
sortReversed = false
allContainers = true
shell = ""               # leer = Auto-Erkennung; nur sh/bash/zsh/fish erlaubt
columns = []             # leer = alle; z.B. ["state","cpu","hist","mem","ip"]
```

### Sicherheit

- Env-Werte mit Credential-artigen Namen (`PASSWORD`, `TOKEN`, `API_KEY`, `AUTH`,
  `JWT`, …) und Connection-Strings (`…://user:pass@…`) werden als `[REDACTED]`
  maskiert — Apple Container liefert Env-Variablen sonst im Klartext.
- Subprozesse laufen ohne Shell-Interpreter (kein `sh -c` mit Nutzereingaben);
  die Exec-Shell ist whitelisted.

### Neu in v0.3

- **System-Tab** (`4`): API-Server-Status/-Version, Ressourcen-Zähler (Container/Volumes/Images), allokierte Volume-Größe
- **Spalten-Konfiguration**: `columns`-Liste in der config.toml (Auswahl + Reihenfolge) und Toggle-Screen per `c`
- **Browser-Open** (`o`): erste published TCP-Port-URL (localhost) bzw. Container-IP im Standard-Browser öffnen; Ports auch in der Detail-Ansicht

### Roadmap (v0.4)

- Spalten-Persistenz direkt aus der TUI
- Log-Suche im Logs-Viewer
- Multi-Container-Aktionen

---

## English

### What is actop?

`actop` shows all Apple containers in a compact terminal grid with live
metrics (CPU %, memory, network and I/O rates, PIDs, uptime, IP) and controls
their lifecycle — like `ctop`, but for Apple's `container` instead of Docker.

Data is polled from the `container` CLI as JSON (default: every 2 s, a
constant two subprocess calls per tick). Apple Container offers no Docker
socket, no events and no pause/health — actop is deliberately CLI-based.

### Requirements

- macOS 26 (Tahoe) on Apple silicon
- [`container`](https://github.com/apple/container) v1.0.0+ (`brew install container`)
- Running API server: `container system start`

### Installation

```bash
git clone https://github.com/eqms/actop && cd actop
make build
./actop
```

### Keys

| Key | Action |
|---|---|
| `tab` / `1`–`4` | Tabs: containers / volumes / images / system |
| `enter` | Container details (incl. env, secrets masked, CPU/mem sparklines) |
| `l` | Logs (follow) |
| `e` | Shell inside the container (fish→zsh→bash→sh) |
| `o` | Open published port (localhost:…) or container IP in the browser |
| `c` | Configure grid columns (session; persist via config.toml) |
| `S` / `X` / `K` / `D` | Start / Stop / Kill / Delete (with confirmation) |
| `s` / `r` | Cycle sort field / reverse order |
| `f` | Filter (name/image) |
| `a` | Toggle stopped containers |
| `h` `?` | Help |
| `q` | Quit or close view |

`r` also triggers an immediate reconnect while the API server is unreachable.

### Configuration

`~/.config/actop/config.toml` (all fields optional):

```toml
pollIntervalMs = 2000
sortField = "name"       # name|state|cpu|mem|net|io|pids|uptime
sortReversed = false
allContainers = true
shell = ""               # empty = auto-detect; only sh/bash/zsh/fish allowed
columns = []             # empty = all; e.g. ["state","cpu","hist","mem","ip"]
```

### Security

- Env values with credential-like names (`PASSWORD`, `TOKEN`, `API_KEY`, `AUTH`,
  `JWT`, …) and connection strings (`…://user:pass@…`) are masked as
  `[REDACTED]` — Apple Container otherwise exposes env variables in plaintext.
- Subprocesses run without a shell interpreter (no `sh -c` with user input);
  the exec shell is whitelisted.

### New in v0.3

- **System tab** (`4`): API server status/version, resource counters (containers/volumes/images), allocated volume size
- **Column configuration**: `columns` list in config.toml (selection + order) and toggle screen via `c`
- **Browser-open** (`o`): opens the first published TCP port URL (localhost) or the container IP in the default browser; ports also shown in the detail view

### Roadmap (v0.4)

- Column persistence directly from the TUI
- Log search in the logs viewer
- Multi-container actions

---

## License

MIT — see [LICENSE](LICENSE). Inspired by [ctop](https://github.com/bcicen/ctop) (MIT).
