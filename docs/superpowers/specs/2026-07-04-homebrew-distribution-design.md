# actop Homebrew Distribution — Design

Date: 2026-07-04
Status: approved
Target: actop v0.4.0 (and every future release)

## Goal

`brew tap eqms/tap && brew install actop` (or `brew install eqms/tap/actop`)
installs actop on macOS. Future releases update the formula automatically
from actop's release CI.

## Scope

- New public GitHub repo `eqms/homebrew-tap` with `Formula/actop.rb` and a
  short README.
- Source-build formula (Go build from the tag tarball) — no pre-built
  binaries, no bottles.
- New `homebrew` job in actop's `.github/workflows/ci.yml` that updates the
  formula on tag releases.
- README (DE + EN) in actop gains Homebrew as the recommended install path.
- Out of scope: homebrew-core submission (notability requirements not met),
  bottles/signed binaries, Linux support (actop is macOS-only by nature).

## Components

### 1. Tap repo `eqms/homebrew-tap`

- Created via `gh repo create eqms/homebrew-tap --public`.
- Layout:
  - `Formula/actop.rb`
  - `README.md` (English; install commands, list of formulas)
- Naming follows Homebrew's convention (`homebrew-` prefix), so the short
  form `brew install eqms/tap/actop` works.

### 2. Formula `Formula/actop.rb`

```ruby
class Actop < Formula
  desc "Top-like terminal UI for Apple containers (like ctop)"
  homepage "https://github.com/eqms/actop"
  url "https://github.com/eqms/actop/archive/refs/tags/v0.4.0.tar.gz"
  sha256 "<sha256 of the tag tarball>"
  license "MIT"
  head "https://github.com/eqms/actop.git", branch: "main"

  depends_on "go" => :build
  depends_on :macos

  def install
    ldflags = "-w -X main.version=#{version} -X main.build=homebrew"
    system "go", "build", *std_go_args(ldflags: ldflags)
  end

  test do
    assert_match "actop version #{version}", shell_output("#{bin}/actop -v")
  end
end
```

- `main.build` is set to `homebrew` — the tag tarball carries no git SHA.
- `actop -v` already exists (main.go:22) and prints
  `actop version <version>, build <build>`.
- Initial `sha256` is computed from the real v0.4.0 tarball at
  implementation time (`curl -sL <url> | shasum -a 256`).

### 3. CI auto-update job (actop repo)

New job in `.github/workflows/ci.yml`, appended after `release`:

- Trigger: same as `release` (`startsWith(github.ref, 'refs/tags/v')`),
  `needs: release`.
- Steps:
  1. Derive `VERSION` from the tag (`${GITHUB_REF_NAME#v}`).
  2. Download the tag tarball from GitHub and compute its sha256.
  3. Clone `eqms/homebrew-tap` using the `HOMEBREW_TAP_TOKEN` secret.
  4. Rewrite the `url` and `sha256` lines in `Formula/actop.rb` (sed).
  5. Commit `[CHG] actop <tag>: update formula` and push.
- Conventions: actions SHA-pinned with version comment, job-level
  `permissions: contents: read` (push auth comes from the PAT, not
  `GITHUB_TOKEN`).
- Failure mode: job failure does not affect the already-published release;
  the formula can be fixed manually.

### 4. Secret (manual, one-time — Captain)

Fine-grained PAT, repository access limited to `eqms/homebrew-tap`,
permission `Contents: Read and write`. Stored in the actop repo as the
actions secret `HOMEBREW_TAP_TOKEN`. The CI job fails with a clear error
if the secret is missing.

### 5. actop README update

Both language sections get Homebrew as the primary install method:

```
brew tap eqms/tap
brew install actop
```

The existing `make build` path stays as the "from source" alternative.

## Error handling

- Tap clone/push failures surface in the Actions log; the release itself is
  unaffected (separate job).
- `sed` rewrite is anchored to the exact `url`/`sha256` line prefixes; if
  the pattern does not match (formula manually diverged), the job fails
  loudly rather than committing a broken formula.

## Testing / Verification

1. `brew install --build-from-source ./Formula/actop.rb` locally (or
   `brew install eqms/tap/actop` after push) — binary runs, `actop -v`
   prints the version.
2. `brew audit --strict --online eqms/tap/actop` — no errors.
3. `brew test actop` — the formula test passes.
4. CI job verified with the next real tag (or by re-pushing a test tag on a
   throwaway branch is NOT possible for tags — instead: manual
   `workflow_dispatch` is out of scope; the job's sed/sha logic is verified
   locally by running the same commands before committing).

## Release

- actop side: `[ADD]` commit (CI job + README); no VERSION bump — the
  change is release-infrastructure, not a feature release. It rides along
  with the next regular release.
- Tap side: initial commit with formula for v0.4.0.
