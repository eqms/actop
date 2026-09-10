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

Via Homebrew (empfohlen):

```bash
brew install eqms/tap/actop
```

Aus dem Quellcode:

```bash
git clone https://github.com/eqms/actop && cd actop
make build
./actop
```

### Tasten

| Taste | Aktion |
|---|---|
| `tab` / `1`–`4` | Tabs: Containers / Volumes / Images / System |
| `enter` | Aktionsmenü wie in ctop: nur die Aktionen, die im aktuellen Zustand gehen, jede mit ihrer Taste (`[l] log view`, `[s] stop`, …) — auch in Volumes/Images |
| `o` / `→` | Detailansicht (single view) |
| `l` / `←` | Logs (follow) |
| `e` | Shell im Container (fish→zsh→bash→sh) |
| `w` | Published Port (localhost:…) bzw. Container-IP im Browser öffnen |
| `c` | Grid-Spalten konfigurieren |
| `S` | Einstellungen speichern (Sortierung, Richtung, gestoppte Container, Spalten → config.toml) |
| `X` / `K` | Stop / Kill (mit Rückfrage) |
| `s` / `r` | Sortierfeld aus einer Liste wählen / Richtung umkehren |
| `f` | Filter (Name/Image) |
| `a` | Gestoppte Container ein-/ausblenden |
| `H` | Kopfzeile ein-/ausblenden (Verbindungswarnung und Meldungen bleiben) |
| `D` | Volume bzw. Image löschen (Tabs 2 und 3, mit Rückfrage) |
| `h` `?` | Hilfe |
| `q` | Beenden bzw. Ansicht schließen |

`r` startet bei getrennter Verbindung außerdem einen sofortigen Reconnect.
Die Übersicht folgt ctop; `X` und `K` sind actop-Zusätze, die ctop nicht
belegt. Starten und Löschen von Containern laufen über das Aktionsmenü.
`S` überschreibt die `config.toml`: Poll-Intervall, Shell und Update-Check
bleiben erhalten, Kommentare in der Datei nicht.

Im Aktionsmenü eines laufenden Containers stehen `[o] single view`,
`[l] log view`, `[s] stop`, `[r] restart`, `[X] stop + remove`,
`[e] exec shell`, `[w] open in browser` (nur mit Port oder IP), `[K] kill`
und `[c] cancel`; bei einem gestoppten `[o]`, `[l]`, `[s] start`,
`[R] remove` und `[c]`. Ein Tastendruck löst die Aktion aus, `j`/`k` und
`enter` gehen weiterhin. Start und Restart laufen sofort, Stop, Kill und
Remove fragen nach.

### Apple Container aktualisieren

```bash
actop update-container        # fragt, bevor laufende Container gestoppt werden
actop update-container -y     # ohne Rückfrage
```

Apples Updater `/usr/local/bin/update-container.sh` läuft nur bei gestopptem
Container-Dienst, und das Stoppen hält alle Container an. `actop
update-container` erledigt die Schritte am Stück: Es prüft auf GitHub, ob
eine neuere Version vorliegt (sonst endet es, ohne etwas anzuhalten), merkt
sich die laufenden Container, stoppt den Dienst, ruft Apples Updater auf (die
sudo-Passwortabfrage erscheint wie gewohnt), startet den Dienst wieder und
danach die zuvor laufenden Container. Zum Schluss steht ein Statusbericht:
Version vorher und nachher, Zustand des Dienstes und je Container
„restarted“, „start failed“ oder „no longer exists“ — letzteres für Container,
die mit `--rm` liefen und von Apple beim Stoppen gelöscht werden.

Schlägt der Updater fehl oder wird er mit Ctrl+C abgebrochen, startet actop
Dienst und Container trotzdem wieder. Der Exit-Code ist `0`, wenn alles
zurück ist, sonst `1`.

### Konfiguration

`~/.config/actop/config.toml` (alle Felder optional):

```toml
pollIntervalMs = 2000
sortField = "name"       # name|state|cpu|mem|net|io|pids|uptime
sortReversed = false
allContainers = true
shell = ""               # leer = Auto-Erkennung; nur sh/bash/zsh/fish erlaubt
columns = []             # leer = alle; z.B. ["state","cpu","hist","mem","ip"]
updateCheck = true       # täglicher Versionscheck; ACTOP_NO_UPDATE_CHECK=1 deaktiviert ihn
```

### Sicherheit

- Env-Werte mit Credential-artigen Namen (`PASSWORD`, `TOKEN`, `API_KEY`, `AUTH`,
  `JWT`, …) und Connection-Strings (`…://user:pass@…`, auch ohne Benutzernamen
  wie `redis://:pass@…`) werden als `[REDACTED]` maskiert — Apple Container
  liefert Env-Variablen sonst im Klartext.
- Subprozesse laufen ohne Shell-Interpreter (kein `sh -c` mit Nutzereingaben);
  die Exec-Shell ist whitelisted.
- Container-Logs werden vor dem Rendern von ANSI-/OSC-Escape-Sequenzen und
  Steuerzeichen befreit (kein Clipboard-Hijack/Titel-Spoofing durch bösartige
  Container-Ausgaben); Namen gehen mit POSIX-`--`-Terminator an die CLI.
- Der Update-Check fragt einmal täglich `api.github.com` nach den neuesten
  Releases — ohne Token, ohne Telemetrie, und nur nach Versionsnummern.
  Abschaltbar per `updateCheck = false` oder `ACTOP_NO_UPDATE_CHECK=1`.

### Neu in v0.9

- **Übersicht mit ctops Tasten**: `o`/`→` Detailansicht, `←` Logs, `w` Browser, `S` Einstellungen speichern, `H` Kopfzeile; Container starten und löschen über das Aktionsmenü

### Neu in v0.8

- **Aktionsmenü wie in ctop**: Tasten stehen im Menü und lösen direkt aus, nur passende Aktionen werden angeboten, neu sind `restart` und `stop + remove`
- **Sortierauswahl**: `s` öffnet eine Liste der Sortierfelder statt blind weiterzuschalten
- **Weniger Rückfragen**: Start und Restart laufen ohne Bestätigung
- **`actop update-container`**: Update von Apple Container inklusive Dienst-Stopp, Neustart der Container und Statusbericht

### Neu in v0.5

- **ctop-treues Aktionsmenü**: feste Einträge (Details, Logs, Shell, Open URL, Start, Stop, Kill, Delete, Cancel) — nicht anwendbare ausgegraut statt ausgeblendet, schlichte Liste ohne Hotkey-Spalte, expliziter Cancel-Eintrag
- **Menüs in allen Tabs**: `enter` öffnet auch bei Volumes und Images ein Aktionsmenü (Delete mit Rückfrage, Cancel)

### Neu in v0.4

- **Container-Aktionsmenü** (`enter`, ctop-Stil): zentriertes Panel mit zustandsabhängigen Aktionen — Navigation `j/k/↑/↓`, Auswahl per `enter` oder Hotkey, `esc` schließt; destruktive Aktionen weiterhin mit Rückfrage
- **Security-Härtung (v0.3.1)**: Terminal-Escape-Filter für Logs, erweiterte Secret-Redaction (`DB_PASS`, `STRIPE_KEY`, …), Argument-Injection-Schutz

Ältere Versionen: siehe [RELEASE_NOTES.md](RELEASE_NOTES.md).

### Roadmap (v0.6)

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

Via Homebrew (recommended):

```bash
brew install eqms/tap/actop
```

From source:

```bash
git clone https://github.com/eqms/actop && cd actop
make build
./actop
```

### Keys

| Key | Action |
|---|---|
| `tab` / `1`–`4` | Tabs: containers / volumes / images / system |
| `enter` | Action menu as in ctop: only the actions that apply to the current state, each with its key (`[l] log view`, `[s] stop`, …) — also on volumes/images |
| `o` / `→` | Single view (details) |
| `l` / `←` | Logs (follow) |
| `e` | Shell inside the container (fish→zsh→bash→sh) |
| `w` | Open published port (localhost:…) or container IP in the browser |
| `c` | Configure grid columns |
| `S` | Save settings (sort field, order, stopped containers, columns → config.toml) |
| `X` / `K` | Stop / Kill (with confirmation) |
| `s` / `r` | Pick the sort field from a list / reverse order |
| `f` | Filter (name/image) |
| `a` | Toggle stopped containers |
| `H` | Toggle the header (connection warning and messages stay) |
| `D` | Delete a volume or image (tabs 2 and 3, with confirmation) |
| `h` `?` | Help |
| `q` | Quit or close view |

`r` also triggers an immediate reconnect while the API server is unreachable.
The grid follows ctop; `X` and `K` are actop extras that ctop leaves unbound.
Containers are started and deleted through the action menu. `S` overwrites
`config.toml`: poll interval, shell and update check are kept, comments in
the file are not.

The action menu of a running container offers `[o] single view`,
`[l] log view`, `[s] stop`, `[r] restart`, `[X] stop + remove`,
`[e] exec shell`, `[w] open in browser` (only with a port or IP), `[K] kill`
and `[c] cancel`; a stopped one offers `[o]`, `[l]`, `[s] start`,
`[R] remove` and `[c]`. One key press runs the action; `j`/`k` and `enter`
still work. Start and restart run at once, stop, kill and remove ask first.

### Updating Apple Container

```bash
actop update-container        # asks before stopping running containers
actop update-container -y     # no question
```

Apple's updater `/usr/local/bin/update-container.sh` only runs while the
container service is stopped, and stopping it halts every container.
`actop update-container` does the whole sequence: it checks GitHub for a
newer release (and ends without stopping anything if there is none), notes
the running containers, stops the service, runs Apple's updater (the sudo
password prompt appears as usual), starts the service again and then the
containers that were running. It finishes with a status report: version
before and after, the service state, and per container "restarted", "start
failed" or "no longer exists" — the latter for containers that ran with
`--rm`, which Apple deletes when the service stops.

If the updater fails or is interrupted with Ctrl+C, actop still brings the
service and the containers back. The exit code is `0` when everything is
back, `1` otherwise.

### Configuration

`~/.config/actop/config.toml` (all fields optional):

```toml
pollIntervalMs = 2000
sortField = "name"       # name|state|cpu|mem|net|io|pids|uptime
sortReversed = false
allContainers = true
shell = ""               # empty = auto-detect; only sh/bash/zsh/fish allowed
columns = []             # empty = all; e.g. ["state","cpu","hist","mem","ip"]
updateCheck = true       # daily release check; ACTOP_NO_UPDATE_CHECK=1 turns it off
```

### Security

- Env values with credential-like names (`PASSWORD`, `TOKEN`, `API_KEY`, `AUTH`,
  `JWT`, …) and connection strings (`…://user:pass@…`, including user-less
  forms such as `redis://:pass@…`) are masked as `[REDACTED]` — Apple Container
  otherwise exposes env variables in plaintext.
- Subprocesses run without a shell interpreter (no `sh -c` with user input);
  the exec shell is whitelisted.
- Container logs are stripped of ANSI/OSC escape sequences and control
  characters before rendering (no clipboard hijacking/title spoofing via
  malicious container output); names are passed to the CLI behind the POSIX
  `--` terminator.
- The update check queries `api.github.com` once a day for the latest
  releases — no token, no telemetry, version numbers only. Disable it with
  `updateCheck = false` or `ACTOP_NO_UPDATE_CHECK=1`.

### New in v0.9

- **Grid with ctop's keys**: `o`/`→` single view, `←` logs, `w` browser, `S` save settings, `H` header; containers are started and deleted through the action menu

### New in v0.8

- **Action menu as in ctop**: keys are shown in the menu and fire directly, only applicable actions are offered, `restart` and `stop + remove` are new
- **Sort selection**: `s` opens a list of sort fields instead of cycling blindly
- **Fewer confirmations**: start and restart run without one
- **`actop update-container`**: updates Apple Container including the service stop, the container restarts and a status report

### New in v0.5

- **ctop-faithful action menu**: fixed entries (Details, Logs, Shell, Open URL, Start, Stop, Kill, Delete, Cancel) — inapplicable ones dimmed instead of hidden, plain list without hotkey column, explicit Cancel entry
- **Menus on all tabs**: `enter` also opens an action menu on volumes and images (delete with confirmation, Cancel)

### New in v0.4

- **Container action menu** (`enter`, ctop-style): centered panel with state-filtered actions — navigate with `j/k/↑/↓`, select via `enter` or hotkey, `esc` closes; destructive actions still require confirmation
- **Security hardening (v0.3.1)**: terminal escape filtering for logs, extended secret redaction (`DB_PASS`, `STRIPE_KEY`, …), argument injection protection

Older versions: see [RELEASE_NOTES.md](RELEASE_NOTES.md).

### Roadmap (v0.6)

- Column persistence directly from the TUI
- Log search in the logs viewer
- Multi-container actions

---

## License

MIT — see [LICENSE](LICENSE). Inspired by [ctop](https://github.com/bcicen/ctop) (MIT).
