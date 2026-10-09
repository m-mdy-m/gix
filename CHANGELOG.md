# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [1.1.0] - 2026-10-10

### Added

* One-command install on every platform. Windows (PowerShell):
  `irm https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1 | iex`
  — no download-then-run step. Linux and macOS are unchanged:
  `curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | sh`.
  Options are environment variables (`GIX_VERSION`, `GIX_PREFIX`) so they
  work through the pipe.
* Releases now publish the bare binary next to the archives:
  `gix-linux-x64`, `gix-linux-arm64`, `gix-darwin-x64`, `gix-darwin-arm64`,
  `gix-windows-x64.exe` and `gix-windows-arm64.exe`, each with a
  `.sha256`. Download one file and run it; no unpacking.
* A Windows ARM64 build (`gix-windows-arm64.exe`); `install.ps1` picks it
  on ARM machines.
* The release workflow runs `gix --version` on each binary it can execute
  (linux-x64, darwin-arm64, windows-x64) and fails the release if the
  version does not match the tag.

### Changed

* Both installers download the bare binary and verify its checksum, and
  fall back to the archive for releases that predate the bare binaries.
* `install.sh` finds the latest release through the `/releases/latest`
  redirect instead of the GitHub API, so it no longer fails on the API's
  rate limit, and works with `wget` when `curl` is missing.
* `install.ps1` no longer needs to be saved first, enables TLS 1.2 for
  Windows PowerShell 5.1, and adds `gix` to `PATH` for the current window
  as well as for new ones.

## [1.0.1] - 2026-10-10

### Fixed

* `gix branch sync --dry-run` no longer reports `would merge` when it
  could not actually check for conflicts. `git merge-tree` exits with
  status 1 both for a real conflict and for a ref it cannot find, and the
  other failures (unrelated histories, a Git too old for `--write-tree`)
  were also read as "no conflicts". Only a conflict that names files now
  counts as one; anything else is an error that says why.
* `gix branch prune` no longer lists a branch you just started and have
  not committed on. Such a branch points at the same commit as its merge
  target, so it looked "already merged".
* `gix branch prune` now requires every configured merge target to
  contain a branch, as the documentation already said. Before, one was
  enough, so a hotfix that had reached `main` but not yet `develop` was
  listed, and `--apply` could delete it before the fix reached `develop`.
* The `gix branch sync` error no longer names a branch it skipped on
  purpose (a `rebase` or `squash` `downstream_strategy`) as one that
  "could not take the merge".
* Colors in the classic Windows console: gix turns on ANSI processing
  for it, and prints plain text if the console refuses, instead of raw
  escape codes.

### Added

* A `git` check in `gix doctor` that reports the installed Git version
  and warns when it is older than 2.38, the first Git with
  `merge-tree --write-tree`. `gix doctor` now runs eleven checks.
* `gix branch sync` refuses to start on a Git older than 2.38 with a
  message that says so, instead of failing once per branch. Git 2.38 or
  newer is now listed as a requirement.
* Git engine helpers `InstalledVersion` and `ParseGitVersion`.

### Changed

* A branch that was fast-forwarded into its merge target points at the
  same commit afterwards, so `gix branch prune` keeps it, the same as a
  branch you just started. `gix <kind> finish` deletes such a branch
  itself, so this only matters for branches merged by hand. Listed under
  known limits in `docs/commands.md`.

## [1.0.0] - 2026-10-10

### Added

* Prerelease versions (SemVer 2.0.0) in `gix version`:
  `gix version bump minor --pre rc` starts `0.5.0-rc.1`,
  `gix version bump --pre rc` moves on to `rc.2` (or to a later name
  such as `beta` → `rc`), and `gix version bump --release` promotes
  `1.0.0-rc.2` to `1.0.0`. `patch`, `minor`, and `major` on a prerelease
  go to the next stable version from its core (`1.0.0-rc.1` minor is
  `1.1.0`); they never promote it. A prerelease can only move forward.
  Tags stay `v`-prefixed (`v1.0.0-rc.1`), nothing is overwritten, and
  `--dry-run`, the main-branch rule, and the clean-tree rule all apply.
  Versions are ordered by SemVer 2.0.0 precedence in gix itself rather
  than by Git's tag order, which sorts `v1.0.0-rc.1` above `v1.0.0`.
  Build metadata (`+build`) is not supported, and a tag carrying it is
  ignored.
* `gix doctor` — inspects the repository and reports what would stop gix
  working: flow setup, HEAD state, working tree, unfinished Git
  operations, Git identity, remote, upstream, signing, and whether the
  base branches `.gix/config` names exist. Read-only; it never rewrites
  anything, and no check ever suggests a command that could discard work
  or history.
* `gix doctor --fix` — applies only what you name explicitly:
  `--set-user-name` + `--set-user-email`, written to `--scope local`
  (default) or `global`. gix never guesses an identity and never
  overwrites one you didn't ask it to. With no identity flags it prints
  the exact command for each problem instead.
* `gix doctor --strict` — exits non-zero when anything needs attention,
  so CI can fail on it. Without it `doctor` still always exits 0: it
  reports, and what counts as blocking is the caller's call.
* `gix signing status` — reports the signing format (`openpgp`, `ssh`,
  `x509`), the configured key and its scope, whether commits and tags are
  signed, and whether the signing program is on `PATH`. Detection only:
  gix never signs anything and never prints key material.
* `gix branch doctor` — one row per local branch showing its config kind,
  the upstream it tracks, how far ahead or behind it is, and a state:
  `tracked`, `ahead`, `behind`, `diverged`, `untracked`, or `gone`.
  `--all` includes healthy branches. Read-only.
* `gix branch prune` — lists branches whose configured merge targets
  already contain their work, plus branches whose upstream is gone.
  `--apply` deletes the ones that are fully merged, using
  `git branch -d` and never `-D`, so git's own refusal to delete an
  unmerged branch is the safety net. A branch whose upstream merely
  disappeared is listed but left alone, because a missing upstream does
  not mean its work was merged. Base branches, the current branch, and
  anything unmerged are never candidates. Without the flag it is a
  preview and deletes nothing.
* A `LAST COMMIT` column in `gix branch doctor` showing how long ago each
  branch tip moved. gix reports the age rather than deciding what counts
  as stale.
* `gix branch sync` — brings a source branch into every topic branch, so a
  change on `develop` reaches work in progress. `--from` picks the source
  (default: the base branch you are on). Conflicts are detected before
  anything is merged using `git merge-tree`, reported per branch, and
  never resolved automatically. One branch failing does not stop the
  rest, and the command exits non-zero so CI notices. Base branches and
  the source are never modified, only merging is used (never rebase), and
  you are put back on the branch you started on.
* `gix branch sync --auto` — narrows a sync run to the branches
  `.gix/config` marks `autosync: true`, on a topic kind or on a base
  branch (which is how the default config already describes `develop`
  following `main`). The source is the one branch given by `--from`, not
  looked up per branch from its `parent`. This makes the `autosync` key,
  which was previously written but never read, meaningful.
* `gix version show` — the current release version, read from the
  repository's tags, and how many commits have landed since.
* `gix version bump <patch|minor|major>` (and the prerelease forms above) — calculates the next version
  and creates one annotated tag on `main`, with `--dry-run` to preview
  it. The tags are the version record: there is no version file to keep
  in step. It deliberately does not edit `CHANGELOG.md`, make a commit,
  sign the tag, or push, because the repository has no convention for any
  of those. It refuses to run anywhere but `main`, on a detached HEAD, on
  a dirty tree, or during an unfinished merge.
* A topic kind added to `.gix/config` now gets its own start/finish
  command group, so `gix wip start api` works with no code change. The
  built-in kinds stay registered unconditionally, and a config key can
  never shadow an existing command.
* `gix config set <kind> --autosync=true|false`, `autosync` in
  `gix config list`, and `gix config set <base> --autosync` so a base
  branch's one setting is reachable without editing the file. Topic-only
  flags are still rejected for a base branch.
* `ui.Table` — aligned columns for output that compares several
  attributes per row.
* Git engine helpers: `ListBranchTracking` (one `for-each-ref` call
  returning every branch with its upstream and Git's own
  `%(upstream:track)` summary, including the `[gone]` case),
  `ListBranchCommits`, `ListTags`, `MergeTreeConflicts`, `RunStatus`
  (keeps stdout on a non-zero exit), `MergeNoFF`, `AbortMerge`.
* `internal/testutil` — throwaway Git repositories with isolated config
  for tests, and tests that skip rather than fail when `git` is absent.
* `docs/configuration.md` — documents every `.gix/config` key and what
  validation rejects. Fixes a broken link from `docs/README.md`.
* `docs/commands.md` — every command step by step, as the code runs it:
  what it checks, what it changes, what it never touches, plus a list of
  known limits.

### Changed

* The release workflow publishes a tag containing `-` (such as
  `v1.0.0-rc.1`) as a GitHub prerelease and no longer moves the Docker
  `latest` tag for it. Stable tags behave as before.
* The `.gix/config` header comment now points at
  `docs/configuration.md` instead of a page that didn't exist.
* A `rebase` or `squash` `downstream_strategy` is now respected by
  `gix branch sync`: the branch is skipped with a message, because a sync
  that rebased would rewrite that branch's history.

### Fixed

* `docs/examples.md` and `docs/getting-start.md` no longer document
  `gix release` for version bumping or imply autocommit exists.
* `gix doctor`'s check headings now go to stdout alongside their
  findings, instead of the heading going to stderr and the details to
  stdout. A redirected report (`gix doctor > report.txt`) was missing the
  names of the checks that had problems.
* `gix version show` no longer fails with git's raw "ambiguous argument
  'HEAD'" when the repository has no commits yet.
* `gix version bump` refuses to run anywhere but the main branch, which is
  where gix already puts release tags, so a develop or feature commit
  cannot be tagged as a release.
* `gix branch sync` reports a branch it cannot check out — because
  another worktree holds it — as a failure for that branch and carries on
  with the rest, instead of leaving it out of the report.
* Tests skip, rather than fail, when `git` is not on `PATH`.

## [0.2.0] - 2026-09-25

### Added

* `gix init` — initialize a Git repository in the current directory.
* Support for specifying the initial branch name during repository initialization.
* Automatic creation of an initial empty commit for a newly initialized repository.
* Git engine helpers for repository initialization and empty commits.

### Changed

* Registered the `init` command in the root command registry.

## [0.1.0] - 2026-09-23

First public release. `gix` automates one git-flow-style branching
model (`main` → `develop` → `feature`/`bugfix`/`hotfix`/`release`)
on top of the real `git` binary.

### Added

- `gix flow init` — write `.gix/config`, commit it, create `develop`
- `gix <feature|bugfix|hotfix|release> start|finish <name>` — create,
  merge (merge/rebase/squash), tag, and clean up topic branches
- `gix status` — branch, flow role, ahead/behind, dirty state
- `gix list` (`--tree`) — open branches per kind, flat or as a tree
- `gix config list|set` — inspect and change the flow without
  editing YAML
- Safety by default: refuses dirty trees and unfinished
  merge/rebase operations
- Global flags: `--verbose`, `--debug`, `--no-color`, `--version`;
  shell completion via `gix completion`
- Prebuilt binaries for every release: linux-x64/arm64,
  darwin-x64/arm64, windows-x64, each with a `.sha256` checksum
- Quick installers: `scripts/install.sh` (Linux/macOS) and
  `scripts/install.ps1` (Windows)
- Docker image `bitsgenix/gix` (multi-arch, `git` included at
  runtime since gix shells out to it)
- GitHub Actions: `build.yml` (fmt-check, vet, tests, 3-OS build)
  and `release.yml` (binaries + GitHub Release + Docker push on
  `v*.*.*` tags)
- Git hooks in `.husky/` (gofmt + vet, Conventional Commits,
  tests) enabled with `make setup`
- Short practical docs under `docs/` (install, getting started,
  examples, commands, configuration, architecture, development)

[Unreleased]: https://github.com/m-mdy-m/gix/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/m-mdy-m/gix/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/m-mdy-m/gix/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/m-mdy-m/gix/compare/v0.2.0...v1.0.0
[0.2.0]: https://github.com/m-mdy-m/gix/releases/tag/v0.2.0
[0.1.0]: https://github.com/m-mdy-m/gix/releases/tag/v0.1.0