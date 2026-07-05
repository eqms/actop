# Homebrew Distribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `brew install eqms/tap/actop` works on macOS, and every future actop tag updates the formula automatically.

**Architecture:** New public repo `eqms/homebrew-tap` holds a source-build formula (Go build from the GitHub tag tarball). actop's release CI gains a `homebrew` job that rewrites the formula's `url`/`sha256` on tag pushes and commits to the tap repo via a PAT secret.

**Tech Stack:** Homebrew formula (Ruby DSL), GitHub Actions, gh CLI, Go 1.26 (build-time dependency of the formula).

**Spec:** `docs/superpowers/specs/2026-07-04-homebrew-distribution-design.md` (in the actop repo)

## Global Constraints

- Tap repo name is exactly `eqms/homebrew-tap` (Homebrew's `homebrew-` prefix convention, enables `brew install eqms/tap/actop`).
- Formula builds from source: `depends_on "go" => :build`, `depends_on :macos`; ldflags set `main.version=#{version}` and `main.build=homebrew`.
- Formula test asserts `actop -v` output; `main.go:22` prints `actop version <version>, build <build>`.
- CI job: actions SHA-pinned with version comment; job-level `permissions: contents: read`; push auth via `HOMEBREW_TAP_TOKEN` secret (PAT), never `GITHUB_TOKEN`; failure must not affect the published release (separate job, no `needs` from it).
- The sed rewrite must be anchored so a non-matching formula fails the job loudly instead of committing garbage.
- actop repo conventions: commit prefixes `[ADD]`/`[CHG]`/`[FIX]`; English code/docs.
- Manual prerequisite (Captain, before the next tag — NOT needed for Tasks 1-3 to complete): fine-grained PAT with `Contents: Read and write` on `eqms/homebrew-tap` only, saved as actions secret `HOMEBREW_TAP_TOKEN` in `eqms/actop`.

---

### Task 1: Tap repo with actop formula

**Files:**
- Create: repo `eqms/homebrew-tap`, cloned to `/Users/picard/gitbase/homebrew-tap`
- Create: `/Users/picard/gitbase/homebrew-tap/Formula/actop.rb`
- Create: `/Users/picard/gitbase/homebrew-tap/README.md`

**Interfaces:**
- Consumes: existing GitHub release tag `v0.4.0` of `eqms/actop` (tarball URL `https://github.com/eqms/actop/archive/refs/tags/v0.4.0.tar.gz`).
- Produces: formula file `Formula/actop.rb` whose `url`/`sha256` lines Task 2's CI job rewrites — the lines MUST keep the exact two-space-indented `  url "` / `  sha256 "` prefixes.

- [ ] **Step 1: Create and clone the tap repo**

```bash
gh repo create eqms/homebrew-tap --public --description "Homebrew tap for Equitania tools" --clone
```

Run from `/Users/picard/gitbase/` so the clone lands at `/Users/picard/gitbase/homebrew-tap`. (If run elsewhere, move the clone there.)

- [ ] **Step 2: Compute the v0.4.0 tarball checksum**

```bash
curl -sL https://github.com/eqms/actop/archive/refs/tags/v0.4.0.tar.gz | shasum -a 256
```

Note the 64-hex-char output — it replaces `SHA256_HERE` in the next step.

- [ ] **Step 3: Write the formula**

Create `/Users/picard/gitbase/homebrew-tap/Formula/actop.rb` (replace `SHA256_HERE` with the Step 2 value):

```ruby
class Actop < Formula
  desc "Top-like terminal UI for Apple containers (like ctop)"
  homepage "https://github.com/eqms/actop"
  url "https://github.com/eqms/actop/archive/refs/tags/v0.4.0.tar.gz"
  sha256 "SHA256_HERE"
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

- [ ] **Step 4: Write the tap README**

Create `/Users/picard/gitbase/homebrew-tap/README.md`:

```markdown
# Equitania Homebrew Tap

Homebrew formulas for Equitania Software GmbH tools.

## Install

```bash
brew tap eqms/tap
brew install actop
```

or in one step:

```bash
brew install eqms/tap/actop
```

## Formulas

| Formula | Description |
|---|---|
| [actop](https://github.com/eqms/actop) | Top-like terminal UI for Apple containers (like ctop) |
```

- [ ] **Step 5: Verify the formula locally (RED→GREEN equivalent)**

```bash
brew install --build-from-source /Users/picard/gitbase/homebrew-tap/Formula/actop.rb
brew test actop
actop -v
```

Expected: install succeeds; `brew test` passes; `actop -v` prints `actop version 0.4.0, build homebrew`.

- [ ] **Step 6: Audit the formula**

```bash
brew audit --strict --formula /Users/picard/gitbase/homebrew-tap/Formula/actop.rb
```

Expected: no errors (style warnings that require the formula to live in a pushed tap may appear — rerun `brew audit --strict eqms/tap/actop` after Step 7 and fix anything it reports).

- [ ] **Step 7: Commit and push the tap**

```bash
cd /Users/picard/gitbase/homebrew-tap
git add Formula/actop.rb README.md
git commit -m "[ADD] actop 0.4.0 formula and tap README"
git push origin main
```

- [ ] **Step 8: End-to-end check via the published tap**

```bash
brew uninstall actop
brew install eqms/tap/actop
actop -v
```

Expected: installs from the tap; `actop version 0.4.0, build homebrew`.

---

### Task 2: CI auto-update job in actop

**Files:**
- Modify: `/Users/picard/gitbase/actop/.github/workflows/ci.yml` (append a job after `release`)

**Interfaces:**
- Consumes: formula line format from Task 1 (`  url "..."` / `  sha256 "..."`, two-space indent).
- Produces: on every `v*` tag push, a commit `[CHG] actop v<X.Y.Z>: update formula` in `eqms/homebrew-tap`.

- [ ] **Step 1: Append the homebrew job to ci.yml**

Append at the end of `.github/workflows/ci.yml` (same indentation level as the `release:` job):

```yaml
  homebrew:
    if: startsWith(github.ref, 'refs/tags/v')
    needs: release
    runs-on: ubuntu-latest
    timeout-minutes: 5
    permissions:
      contents: read
    steps:
      - name: Update formula in tap
        env:
          TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
          TAG: ${{ github.ref_name }}
        run: |
          set -euo pipefail
          if [ -z "${TAP_TOKEN}" ]; then
            echo "::error::HOMEBREW_TAP_TOKEN secret is missing — formula not updated"
            exit 1
          fi
          URL="https://github.com/eqms/actop/archive/refs/tags/${TAG}.tar.gz"
          SHA=$(curl -fsSL "${URL}" | sha256sum | cut -d' ' -f1)
          git clone "https://x-access-token:${TAP_TOKEN}@github.com/eqms/homebrew-tap.git" tap
          cd tap
          # anchored checks: fail loudly if the formula diverged
          grep -q '^  url "' Formula/actop.rb
          grep -q '^  sha256 "' Formula/actop.rb
          sed -i "s|^  url \".*\"|  url \"${URL}\"|" Formula/actop.rb
          sed -i "s|^  sha256 \".*\"|  sha256 \"${SHA}\"|" Formula/actop.rb
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add Formula/actop.rb
          git commit -m "[CHG] actop ${TAG}: update formula"
          git push
```

Note: `needs: release` orders the job after the GitHub release exists, but a
`homebrew` failure cannot un-publish the release — that satisfies the spec's
"failure must not affect the published release".

- [ ] **Step 2: Validate the workflow syntax**

```bash
cd /Users/picard/gitbase/actop
actionlint .github/workflows/ci.yml 2>/dev/null || python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); print('YAML OK')"
```

Expected: `YAML OK` (or actionlint silence if installed).

- [ ] **Step 3: Dry-run the update logic locally (the job's test)**

Run the job's core logic against a throwaway clone — no push:

```bash
cd "$(mktemp -d)"
TAG=v0.4.0
URL="https://github.com/eqms/actop/archive/refs/tags/${TAG}.tar.gz"
SHA=$(curl -fsSL "${URL}" | shasum -a 256 | cut -d' ' -f1)
git clone https://github.com/eqms/homebrew-tap.git tap
cd tap
grep -q '^  url "' Formula/actop.rb && grep -q '^  sha256 "' Formula/actop.rb && echo "anchors OK"
sed -i '' "s|^  url \".*\"|  url \"${URL}\"|" Formula/actop.rb
sed -i '' "s|^  sha256 \".*\"|  sha256 \"${SHA}\"|" Formula/actop.rb
git diff --stat
grep -E '^  (url|sha256) ' Formula/actop.rb
```

Expected: `anchors OK`; `git diff --stat` shows 0 or 2 changed lines (0 if the formula already points at v0.4.0 — both prove the rewrite is idempotent and anchored); the grep shows the correct URL and SHA. (Note: macOS `sed -i ''` vs. Linux `sed -i` in the workflow — the workflow runs on ubuntu.)

- [ ] **Step 4: Commit**

```bash
cd /Users/picard/gitbase/actop
git add .github/workflows/ci.yml
git commit -m "[ADD] CI: auto-update Homebrew formula on tag release"
```

---

### Task 3: actop README — Homebrew as primary install path

**Files:**
- Modify: `/Users/picard/gitbase/actop/README.md` (both the `### Installation` blocks — German section around line 30-38, English section around line 110-118)

**Interfaces:**
- Consumes: published tap from Task 1 (`brew install eqms/tap/actop`).

- [ ] **Step 1: Update the German install section**

The German section (README.md:32-38) currently reads:

````markdown
### Installation

```bash
git clone https://github.com/eqms/actop && cd actop
make build
./actop
```
````

Replace it with:

````markdown
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
````

- [ ] **Step 2: Update the English install section**

The English section contains the identical `### Installation` block (around README.md:110). Replace it with:

````markdown
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
````

- [ ] **Step 3: Verify rendering and consistency**

```bash
grep -n "brew install eqms/tap/actop" /Users/picard/gitbase/actop/README.md
```

Expected: exactly 2 matches (DE + EN).

- [ ] **Step 4: Commit**

```bash
cd /Users/picard/gitbase/actop
git add README.md
git commit -m "[CHG] README: Homebrew install instructions (DE/EN)"
```

---

## Completion notes

- No VERSION bump in actop (release infrastructure, rides along with the next regular release — per spec).
- Remind the Captain about the manual prerequisite: fine-grained PAT (`Contents: Read and write`, repo scope `eqms/homebrew-tap` only) as secret `HOMEBREW_TAP_TOKEN` in `eqms/actop` — without it the `homebrew` CI job fails on the next tag (loudly, release unaffected).
- Full verification story: Task 1 Step 8 proves the user-facing install; Task 2 Step 3 proves the update logic; the CI job itself gets its live verification on the next real tag.
