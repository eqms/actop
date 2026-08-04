# actop Update Check — Design

Date: 2026-08-04
Status: approved
Target: actop v0.7.0

## Goal

On start, actop tells the user when a newer version of Apple Container or of
actop itself is available, and which command installs it. actop never performs
the update.

Apple Container ships no update notification of its own, so a user can sit on
an outdated runtime for months without noticing. actop already watches that
runtime every two seconds — it is the natural place to surface the hint.

## Scope

- New package `update/`: HTTP check against the GitHub releases API, semver
  comparison, 24 h on-disk cache.
- New `cli.Client` method `Version(ctx)` (`container --version`), implemented by
  `ExecClient` and `MockClient`.
- New config key `updateCheck` (default `true`) and env override
  `ACTOP_NO_UPDATE_CHECK`.
- UI: a badge in the header, an `UPDATES` block in the system tab.
- Out of scope: downloading or installing anything, privilege escalation,
  prerelease channels, checking any project other than `apple/container` and
  `eqms/actop`.

## Non-goals, explicitly

actop must not run `update-container.sh`, `brew upgrade`, `sudo`, or an
installer. Updating Apple Container stops every running container; that is the
user's decision to make outside actop, with actop closed. The feature is a
notification, not an updater — the package name `update/` describes what it
watches, not what it does.

## Components

### 1. `update/semver.go`

```go
type Version struct{ Major, Minor, Patch int }

func Parse(s string) (Version, bool)   // "v1.2.0" and "1.2.0" both parse
func (v Version) Less(o Version) bool
func (v Version) String() string       // canonical "1.2.0", no "v"
```

`Parse` accepts an optional leading `v` followed by exactly three
dot-separated non-negative integers, nothing else — no prerelease suffix, no
build metadata, no fourth component. Anything else returns `false`. This is
both the version parser and the primary input filter (see Security).

A hand-written parser rather than a dependency: three integers do not justify a
new module in a tool whose dependency list is currently five entries.

### 2. `update/check.go`

```go
type Component struct {
    Name    string // "container" | "actop"
    Current string // canonical, e.g. "1.1.0"
    Latest  string // canonical, e.g. "1.2.0"
    Hint    string // the command the user should run
}

type Result struct {
    Components []Component // outdated components only; up-to-date ones are dropped
    CheckedAt  time.Time
}

func (r Result) Count() int  // len(r.Components); 0 for the zero value

type Checker struct {
    HTTP     *http.Client // nil → package default (5 s timeout)
    BaseURL  string       // "" → https://api.github.com, overridden in tests
    CacheDir string       // "" → resolved from XDG_CACHE_HOME
    Now      func() time.Time // nil → time.Now, for cache-expiry tests
}

func (c *Checker) Run(ctx context.Context, local map[string]string) Result
```

`local` maps component name → locally installed version string. A component
whose local version does not `Parse` is skipped entirely — this is how a
development build (`version == "dev"`) drops out without a special case.

`Run` never returns an error. A failed DNS lookup, a 403 from a rate limit, a
timeout, malformed JSON, a garbage tag: each yields a `Result` with no
components. A monitoring tool must not report on its own update checker.

Per component the flow is: read cache → if fresh (< 24 h) use it → otherwise
`GET {BaseURL}/repos/{repo}/releases/latest`, take `tag_name`, `Parse` it,
write the cache. `/releases/latest` is used deliberately: GitHub excludes
drafts and prereleases from that endpoint, so no client-side filtering of
`1.3.0-beta.1` style tags is needed.

Repos are a package-level constant map, not configurable — a user-supplied URL
would turn the checker into a request forwarder.

Hints:

| Component   | Hint                                                       |
|-------------|------------------------------------------------------------|
| `container` | `/usr/local/bin/update-container.sh` if that file exists    |
| `container` | `https://github.com/apple/container/releases` otherwise     |
| `actop`     | `brew upgrade actop`                                        |

The `container` hint is chosen with a single `os.Stat`. The script is installed
by Apple's signed package; on a machine that got `container` some other way it
does not exist, and pointing at a missing path would be worse than pointing at
the release page.

### 3. `update/cache.go`

`$XDG_CACHE_HOME/actop/update.json`, falling back to `~/.cache/actop`
(consistent with `config.Path()`, which resolves `XDG_CONFIG_HOME` the same
way).

```json
{
  "checkedAt": "2026-08-04T09:12:44Z",
  "latest": { "container": "1.2.0", "actop": "0.7.0" }
}
```

Written with `0o600`, directory `0o700`. Read errors, a missing file and
malformed JSON are all treated as a cache miss. A `checkedAt` in the future
(clock moved backwards) also counts as a miss rather than pinning the cache for
a decade. The cache stores only the fetched remote version, never the local
one, so a `brew upgrade` between two starts takes effect immediately.

Writing the cache is best-effort: on a read-only home directory the check
degrades to one request per start, which is acceptable and silent.

### 4. `cli.Client.Version`

```go
// Version returns the container CLI version, e.g. "1.2.0".
Version(ctx context.Context) (string, error)
```

`ExecClient` runs `container --version` and extracts the first
`\d+\.\d+\.\d+` match from the output (`container CLI version 1.2.0 (build:
release, commit: 6e65319)`). No shell, argument array as everywhere else, and
the existing bounded-I/O `run` helper.

`apiServerVersion` from `system status` is deliberately not reused: it is empty
whenever the API server is stopped — which is exactly the state a user is in
right after `container system stop`, i.e. immediately before updating.

`MockClient` gains a settable `VersionValue` (default `"1.0.0"`) and records
the call like every other method.

### 5. Config

```go
UpdateCheck *bool `toml:"updateCheck"`
```

A pointer, because `Default()` must yield `true` and a bare `false` in the TOML
file has to be distinguishable from an absent key. Exposed as:

```go
func (c Config) UpdateCheckEnabled() bool
```

which returns `false` if `ACTOP_NO_UPDATE_CHECK` is set to any non-empty value
other than `0`, then the configured value, then `true`. The env variable wins
over the file so a user can suppress the check for a single run.

### 6. UI wiring

`App` gains one field, `updates update.Result`, and one message type:

```go
type updatesMsg struct{ result update.Result }
```

`App.Init` returns `checkUpdatesCmd(a.client, a.cfg, a.version)` alongside the
existing commands; the command reads `container --version`, calls
`Checker.Run` and delivers `updatesMsg`. `App.Update` stores the result. That
keeps the whole feature inside the update loop — `model.Store` and
`metrics.Engine` stay lock-free, per the architecture rule that anything
concurrent goes through a `tea.Msg`.

If `UpdateCheckEnabled()` is false, `checkUpdatesCmd` returns `nil` and no HTTP
client is constructed.

**Header.** When `a.updates.Count() > 0`, append `⬆ N updates` in `styleWarn`
to the info segment. Suppressed when `!a.connOK` (the error banner owns that
space) and when the rendered line would exceed the terminal width — an update
hint must never push the tab bar out of view.

**System tab.** Below `RESOURCES`, only when there is something to report:

```
  UPDATES
  container    1.1.0 → 1.2.0 available
               /usr/local/bin/update-container.sh
  actop        0.6.1 → 0.7.0 available
               brew upgrade actop
```

Rendered with the existing `line()` helper and `styleColHead`, so it inherits
the tab's layout. The version numbers and the hint go through `safe()` like
every other externally sourced string.

## Data flow

```
App.Init
  └─ checkUpdatesCmd (goroutine, off the update loop)
       ├─ client.Version(ctx)            → "1.1.0"
       ├─ Checker.Run(ctx, {...})
       │    ├─ cache read  (fresh? → done)
       │    └─ GET api.github.com/repos/{repo}/releases/latest
       │         └─ tag_name → Parse → compare → cache write
       └─ updatesMsg → App.Update → a.updates
                                       ├─ header badge
                                       └─ system tab block
```

At most two HTTP requests per day per user. Nothing in the polling path
changes: still two subprocess calls per tick.

## Security

GitHub is a new untrusted input source, and the existing invariants extend to
it unchanged.

- **Bounded I/O.** 5 s timeout on the client, request context cancelled with
  the program, response body read through an `io.LimitReader` at 1 MiB. Mirrors
  the caps `ExecClient.run` already applies to stdout and stderr.
- **Strict validation before rendering.** `tag_name` must satisfy the three-
  integer semver grammar; everything else is discarded. A tag containing an
  OSC 52 sequence never reaches a `Component`, and the canonical `String()`
  form is what gets rendered — not the bytes GitHub sent.
- **`safe()` regardless.** Both render paths sanitize, per the rule that any
  render path printing an externally sourced string routes through `safe()`.
  The parser is the first line, `safe()` the second.
- **No credentials.** Unauthenticated requests only; no token is read from the
  environment, so none can leak. 60 requests/hour is ample for two per day.
- **No user-controlled URLs.** Repos are constants. `Checker.BaseURL` exists
  solely so tests can point at `httptest.Server` and is never set from config.
- **Cache file permissions.** `0o600` / `0o700`; the content is public
  information, but there is no reason to be looser than the config file.
- **Opt-out.** A user who wants actop to make no network connections at all
  sets `updateCheck = false` or exports `ACTOP_NO_UPDATE_CHECK=1`; in that case
  no HTTP client is created and no cache file is touched.

## Testing

No test touches the network; `Checker.BaseURL` points at an `httptest.Server`
and `Checker.Now` at a fixed clock.

`update/`:
- `Parse` on `"1.2.0"`, `"v1.2.0"`, `"1.2"`, `"1.2.0.1"`, `"v1.2.0-beta.1"`,
  `""`, and a tag carrying an escape sequence — only the first two parse.
- `Less` across major/minor/patch boundaries, including `1.10.0 > 1.9.0`.
- `Run`: update available, up to date, local version newer than remote
  (nothing reported), unparseable local version (component skipped).
- Failure modes: server returns 500, returns 403, returns malformed JSON,
  returns a 2 MiB body, connection refused — each yields an empty `Result` and
  no panic.
- Cache: miss writes the file, fresh hit performs zero requests (asserted via a
  request counter on the test server), entry older than 24 h refetches,
  `checkedAt` in the future refetches, unwritable cache dir still returns a
  correct result.

`config/`: `updateCheck` absent → `true`, `= false` → `false`,
`ACTOP_NO_UPDATE_CHECK=1` overrides `= true`, `ACTOP_NO_UPDATE_CHECK=0` does
not.

`cli/`: `Version` against the generated shell script fixture, covering the real
output format and a garbage line that yields an error.

`ui/`: header badge appears with a non-empty result and is absent when
`connOK == false`, when the result is empty, and when the width is too small;
the system tab renders the `UPDATES` block and omits the whole section when
there is nothing to report. Assertions run against `App.Body()`.

## Release

- `VERSION` → `0.7.0`.
- `RELEASE_NOTES.md`: one entry in the German half and one in the English half,
  dated `### v0.7.0 (2026-08-04)`, mentioning the config key and the env
  variable so the opt-out is discoverable.
- README (DE + EN): a short note that actop checks for updates once a day and
  how to switch it off.
