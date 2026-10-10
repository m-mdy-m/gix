# Commands in depth

[Commands](api/README.md) is the quick reference: one table, one line per
flag. This page is the long version. For each command it says **what it
checks, what it does, in what order, and what it never touches**, taken
from the code rather than from intentions.

- [How every command behaves](#how-every-command-behaves)
- [Setting up](#setting-up): `gix init`, `gix flow init`
- [Working on branches](#working-on-branches): `gix <kind> start`, `gix <kind> finish`
- [Looking around](#looking-around): `gix status`, `gix list`
- [Changing the config](#changing-the-config): `gix config list`, `gix config set`
- [Checking health](#checking-health): `gix doctor`, `gix signing status`, `gix branch doctor`
- [Keeping branches current and tidy](#keeping-branches-current-and-tidy): `gix branch sync`, `gix branch prune`
- [Releases](#releases): `gix version show`, `gix version bump`
- [Updating gix](#updating-gix): `gix update`
- [Known limits](#known-limits)

## How every command behaves

**One engine.** gix does not reimplement Git. Every command ends up
running the real `git` binary through `internal/git`, so what gix does
is always something you could have typed yourself. `--verbose` prints
each git command gix runs; `--debug` adds raw git output and timing.
`--no-color` turns colors off. These three flags work on every command.

**Config.** Commands that need the flow read `.gix/config` from the
repository root. Without it they stop with `gix is not initialized in
this repository — run gix flow init first`. A config that fails
validation is also an error, with the file path and the reason.

**Safe to refuse.** Commands that change branches (`start`, `finish`,
`sync`, `version bump`) first check two things and refuse if either
fails:

1. no Git operation is in progress (a merge, rebase, cherry-pick, and so
   on): `repository has an unfinished git operation in progress`
2. the working tree is clean: `working tree is not clean — commit or
   stash your changes first`

Read-only commands (`status`, `list`, `doctor`, `branch doctor`,
`signing status`, `version show`, and any `--dry-run`) skip these checks.
A preview that refused to run on a dirty tree would hide exactly what you
wanted to look at.

**Exit codes.** `0` on success, `1` on any error, printed as one line.
`gix doctor` always exits `0` unless you pass `--strict`. `gix branch
sync` exits `1` when any branch could not be merged, even though the
other branches were handled.

**Never.** No command force-pushes, resets, or pushes at all. gix never
talks to a remote on its own: pushing, pulling, and fetching are yours.

## Setting up

### `gix init`

Gets a directory to the point where `gix flow init` can run.

1. If the directory is not a Git repository, runs `git init` (with `-b
   <name>` when `--initial-branch` is given). If it already is one,
   `--initial-branch` is ignored with a warning.
2. If the repository has commits, stops there: `already has commits,
   nothing more to do`. Re-running is always safe.
3. If there are no commits yet, creates an empty `initial commit`, because
   `gix flow init` needs a base branch to build on. That needs a Git
   identity. With none configured, `init` leaves the repository
   commit-less and prints the exact commands to finish.

| Flag | Effect |
|------|--------|
| `--initial-branch <name>` | branch name for a newly created repository |
| `--no-commit` | create the repository only, no initial commit |
| `--author-name`, `--author-email` | identity for the initial commit, this run only; must be given together |

It never changes your Git config, and it never touches a repository that
already has history.

### `gix flow init`

Sets gix up in an existing repository, once.

1. Refuses if the repository has no commits (run `gix init` first).
2. Refuses if `.gix/config` already exists **and is committed**: `gix is
   already initialized`. If the file exists but was never committed (an
   earlier run stopped half-way), it is reused and the run resumes.
3. Validates `--main` and `--develop`: not empty, no `..`, no leading or
   trailing `/`, and different from each other.
4. Writes `.gix/config` from the built-in default. The main branch is
   `--main` if given; otherwise the branch you are on when it is named
   `main`, `master`, or `trunk`; otherwise `main`.
5. Checks that the main branch really exists. If not, it lists your
   branches and suggests `--main`.
6. Commits the config as `chore(gix): initialize gix flow config`. That
   needs a Git identity; without one the config stays written but
   uncommitted, and the output says how to finish. `--no-commit` always
   skips this step.
7. Creates the develop branch from main, unless it already exists, in
   which case it is left as it is.

It only ever touches `.gix/config` and the develop branch. It does not
need a clean working tree, because it does not touch anything else.

| Flag | Effect |
|------|--------|
| `--main <name>` | name of the main branch |
| `--develop <name>` | name of the develop branch |
| `--no-commit` | write `.gix/config`, do not commit it |
| `--author-name`, `--author-email` | identity for the config commit, this run only |

## Working on branches

`feature`, `bugfix`, `hotfix`, and `release` are four command groups
with the same two subcommands. Nothing about them is hard-coded per kind:
the parent, the merge targets, tagging, and strategy all come from
`.gix/config` at run time. A kind you add to the config gets its own
group the same way (`gix wip start api`). A kind can never shadow a
built-in command such as `config` or `version`.

### `gix <kind> start <name>`

Creates a branch from the kind's parent and switches to it.

1. Refuses on an unfinished Git operation or a dirty tree.
2. Builds the full name: the kind's prefix plus `<name>`. If you already
   typed the prefix (`feature/api`) it is not doubled.
3. Validates the full name against Git's ref rules and refuses if the
   branch exists.
4. Checks that the parent branch exists (`run gix flow init first`
   otherwise).
5. Runs `git checkout -b <full> <parent>`.

Default parents: `feature` and `bugfix` and `release` start from
`develop`; `hotfix` starts from `main`.

### `gix <kind> finish <name>`

Merges a branch back into its targets and cleans up.

1. Refuses if the branch does not exist, on an unfinished Git operation,
   or on a dirty tree.
2. `--tag` is refused unless the kind has `tag: true`, the main branch
   exists, and the tag does not exist yet. An existing tag is never
   overwritten.
3. Resolves the strategy: `--strategy` if given, else the kind's
   `upstream_strategy`, else `merge`.
4. Resolves the merge targets: the kind's `merge_into`, else its
   `parent`. Every target must exist, checked **before** anything is
   merged.
5. For each target, in order:
   - already contains the branch? Counted as "already up to date" and
     skipped, so a re-run after a partial failure is safe.
   - `merge`: switch to the target, `git merge --no-ff` (always a merge
     commit).
   - `rebase`: rebase the topic branch onto the target, then
     fast-forward the target. This **rewrites the topic branch's
     history**, which is what rebase is.
   - `squash`: `git merge --squash`, one commit on the target.
6. With `--tag <name>`: switches to main and creates an annotated tag
   there (message `<Kind> <name>`).
7. Deletes the branch if the kind has `delete_on_finish: true` and you
   did not pass `--no-delete`. Because a squash or rebase makes `git
   branch -d` refuse, finish deletes with `git branch -D` **after** the
   merges succeeded. If the delete fails, that is a warning and not an
   error: the merge and tag already happened.

If a merge conflicts half-way through several targets, the error says
which targets were already merged. Those merges are **not** undone and
the branch is **not** deleted; resolve the conflict and re-run finish.

| Flag | Effect |
|------|--------|
| `--strategy merge\|rebase\|squash` | override the strategy for this run |
| `--no-delete` | keep the branch after merging |
| `--tag <name>` | annotated tag on main after merging (only for kinds with `tag: true` in the built-in defaults, see [Known limits](#known-limits)) |

With the default config, `hotfix` and `release` merge into both `main`
and `develop` and may tag; `feature` and `bugfix` merge into `develop`.

## Looking around

### `gix status`

Read-only. Shows where you are in the flow.

- the current branch, or `detached HEAD at <commit>`, or `no commits yet`
- its role: `the main branch`, `the develop branch`, `a <kind> branch,
  based on <parent>`, or `not part of the configured flow`
- whether the working tree is clean
- how many commits you are ahead of and behind develop
- a warning if a Git operation is in progress
- a warning for each configured base branch that does not exist, with a
  hint that it may have been renamed

### `gix list`

Read-only. Lists the topic branches of every configured kind, marking
the current one with `*`.

`--tree` draws the flow as a tree: `main` at the root, `develop` under
it, and each kind's branches under its parent. A configured base branch
that does not exist is flagged.

## Changing the config

`.gix/config` is plain YAML (see [Configuration](configuration.md)) and
you may edit it by hand. These two commands are for the common cases.

### `gix config list`

Prints the config path, the remote, the main and develop names, and for
each kind: `prefix`, `parent`, `merge-into`, `upstream`, `downstream`,
`tag`, `autosync`, `delete`. Defaults are filled in, so you see what gix
will actually use.

### `gix config set <kind>`

Changes one kind. Only the flags you pass change; everything else is left
alone. The result is validated before it is written, so you cannot save
a config that gix would refuse to load. It writes the file and does
**not** commit it.

| Flag | Values |
|------|--------|
| `--prefix` | a valid branch-name prefix, such as `feat/` |
| `--upstream-strategy` | `merge`, `rebase`, `squash` |
| `--downstream-strategy` | `merge`, `rebase` |
| `--tag` | `true` or `false` |
| `--delete-on-finish` | `true` or `false` |
| `--autosync` | `true` or `false` |

A base branch (`main`, `develop`) accepts only `--autosync`. With no
flags it fails with `no changes given`.

## Checking health

### `gix doctor`

Read-only report with eleven checks, always in the same order, so two runs
print the same thing. Each check is `ok`, a warning, or a failure, with a
reason and the exact command that fixes it.

| Check | Fails or warns when |
|-------|---------------------|
| git | the installed Git is older than 2.38 (warning), which `gix branch sync` needs for merge previews |
| repository | not inside a Git repository |
| gix config | missing (warning) or invalid (failure) |
| HEAD | detached, or no commits yet |
| working tree | uncommitted changes |
| in-progress operation | an unfinished merge, rebase, etc. |
| identity | `user.name` or `user.email` missing (failure); set in different scopes (warning) |
| remote | the configured remote does not exist |
| upstream | the current branch tracks nothing |
| signing | configured, but git cannot use it |
| flow | a configured base branch does not exist (failure) |

`gix doctor` run outside a repository stops earlier with `not a git
repository`.

`--fix` changes only what you name, which today means the Git identity:

```bash
gix doctor --fix --set-user-name "Your Name" --set-user-email "you@example.com"
gix doctor --fix --scope global --set-user-name "..." --set-user-email "..."
```

`--scope` is `local` (this repository, the default) or `global`. Name and
email must be given together. gix never guesses an identity and never
sets one you did not type. `--fix` with no values changes nothing and
prints the command for each fixable problem.

`--strict` exits `1` when any check is not ok, for CI.

### `gix signing status`

Read-only. Reports the signing format (`openpgp` by default, `ssh`, or
`x509`), whether commits and tags are signed (`commit.gpgsign`,
`tag.gpgsign`), the key identifier and the config scope it came from, and
the program the format needs. Then one verdict: `git can sign with this
setup`, or `git cannot sign` with the reason (program not on `PATH`, or an
unknown `gpg.format`). With signing off and no key, it says so and stops.

It reads configuration and looks for the program on `PATH`. It never
signs, never creates keys, and never prints key material.

### `gix branch doctor`

Read-only. One row per local branch, in Git's ref order: its config kind,
upstream, ahead and behind counts, how long since its last commit, and a
state.

| State | Meaning |
|-------|---------|
| `tracked` | has an upstream and matches it |
| `ahead` | has commits the upstream does not |
| `behind` | the upstream has commits it does not |
| `diverged` | both |
| `untracked` | no upstream configured |
| `gone` | had an upstream whose ref no longer exists |

By default only branches that need attention are shown; `--all` shows
every branch. A branch already merged into all its merge targets gets a
hint that `gix branch prune` lists it. gix reports the age of the last
commit and does not decide what counts as stale.

The data comes from `git for-each-ref` (`%(upstream:track)`), so it
reflects your last fetch. gix does not fetch.

## Keeping branches current and tidy

### `gix branch sync`

Merges a source branch into every eligible topic branch, so a change that
landed on `develop` reaches work in progress.

1. Unless `--dry-run`, refuses on an unfinished operation or dirty tree.
2. Picks the source: `--from`, else the base branch you are standing on,
   else the first configured base. The source must exist.
3. Picks the candidates: every topic branch except the source; with
   `--auto`, only branches marked `autosync: true` (their own or their
   kind's, and base branches that say so themselves).
4. For each candidate, in Git's ref order:
   - already contains the source: `up to date`, nothing done
   - kind's `downstream_strategy` is not `merge`: `skipped`, because rebase
     would rewrite history and squash would discard the commits being
     brought in
   - no common ancestor: `failed`
   - asks `git merge-tree` whether the merge would conflict, **without
     touching the repository**. A conflict is reported with the files and
     the branch is left alone. If `merge-tree` itself fails (an unknown
     ref, or a Git older than 2.38), that is reported as an error, never
     as a clean merge.
   - otherwise checks the branch out and runs `git merge --no-ff
     --no-edit`. If that still fails, the merge is aborted so the next
     branch is not blocked.
5. Returns you to the branch you started on.

One branch failing never stops the others. Any conflict or failure makes
the exit code `1`. The source and base branches are never modified, except
a base that is marked `autosync` and selected by `--auto`.

`--dry-run` runs the same decisions and prints `would merge`, `conflict`,
and so on, changing nothing. The source is a single branch for the whole
run: it is not looked up per candidate from each branch's `parent`.

### `gix branch prune`

Lists branches that could be cleaned up. Without `--apply` it deletes
nothing.

A branch is listed when every one of its merge targets already contains
it and it has commits of its own, or when its upstream is gone. A branch
you just started, with no commits yet, points at the same commit as its
target and is not listed. A hotfix that has reached `main` but not yet
`develop` is not listed either. Base branches and the branch you are
standing on are never listed.

`--apply` deletes only the fully merged ones, with `git branch -d`, never
`-D`: Git's own refusal to delete unmerged work is the safety net. A
branch whose upstream merely disappeared is listed but kept, because a
missing upstream does not prove it was merged. A kept branch makes the
exit code `1`.

## Releases

Releases are tags. There is no version file: the highest `v*` tag that
gix can read as SemVer **is** the version.

### `gix version show`

Read-only. Prints the current version, its tag, and how many commits
there are since it. With no tags: `no release tags yet`, and what the
first release would be (`v0.1.0`).

The current version is chosen by SemVer precedence, not by Git's tag
order: `1.0.0-rc.1` is below `1.0.0`, and `rc.2` is below `rc.10`. A tag
gix cannot read exactly (`v1.2`, `nightly`, `v1.2.3+build`) is ignored.

### `gix version bump [level] [--pre <name> | --release]`

Works out the next version from the highest existing tag and creates one
annotated tag, `v<version>`, on the current commit.

Before tagging it requires: not a detached HEAD; on the main branch; no
unfinished operation; a clean tree; and a tag that does not exist yet.
`--dry-run` skips the clean-tree check and creates nothing.

| Command | From | To |
|---------|------|----|
| `bump patch` | `0.4.3` | `0.4.4` |
| `bump minor` | `0.4.3` | `0.5.0` |
| `bump major` | `0.4.3` | `1.0.0` |
| `bump minor --pre rc` | `0.4.3` | `0.5.0-rc.1` |
| `bump --pre rc` | `1.0.0-rc.1` | `1.0.0-rc.2` |
| `bump --pre rc` | `1.0.0-beta.2` | `1.0.0-rc.1` |
| `bump --release` | `1.0.0-rc.2` | `1.0.0` |
| `bump minor` | `1.0.0-rc.1` | `1.1.0` |

With no tags, the first release is `0.1.0`.

Rules, in the order the code applies them:

- `--release` promotes the current prerelease as it is. It takes no level
  and no `--pre`, and it is refused when the current version is stable.
- `--pre <name>` with a level starts a prerelease of the next stable
  version, numbered `.1`.
- `--pre <name>` alone continues the current prerelease: the number goes
  up in the same series, or the series starts again at `.1` under a new
  name. It is refused on a stable version, and refused if the result would
  not be higher (`--pre beta` on `1.0.0-rc.1`).
- A level alone makes a stable version from the core of the current one.
  It never promotes: `minor` on `1.0.0-rc.1` is `1.1.0`.
- A prerelease name is letters, digits, and hyphens, and not only digits.

What `bump` does not do: edit `CHANGELOG.md`, make a commit, sign the
tag, or push. Those stay yours:

```bash
gix version bump minor --dry-run
gix version bump minor
git push origin v0.5.0
```

Pushing a tag starts the release workflow. For a tag containing `-` it
publishes the GitHub Release as a prerelease and does not move the Docker
`latest` tag.

## Updating gix

### `gix update` (alias `gix upgrade`)

Replaces the gix you are running with a published release. It needs no
repository and never touches `.gix/config`.

1. Finds the latest release through GitHub's `/releases/latest`
   redirect (no API, no rate limit, no token).
2. Compares it with the running version using SemVer precedence. If you
   are already on it or newer: `gix X is up to date`, nothing happens.
3. Downloads `gix-<os>-<arch>` (`.exe` on Windows) into the folder gix is
   installed in.
4. Checks it against the release's `.sha256`. A release with no checksum
   is refused.
5. Runs the download once with `--version` and expects the release's
   version in the output, so a broken file never replaces a working one.
6. Swaps it in. On Linux and macOS that is a rename over the old file. On
   Windows a running `.exe` can be renamed but not overwritten, so the old
   one is moved to `gix.exe.old` and removed on the next update.

If any step fails, the gix you have is left as it was and the temporary
file is removed.

| Flag | Effect |
|------|--------|
| `--check` | only say whether a newer release exists |
| `--to v1.0.1` | install that release instead, older or newer |
| `--force` | reinstall, or replace a development build |

A gix built from source (`make install`, `go install`) reports `dev` or a
`git describe` string such as `1.1.1-3-gabc123`. That is a development
build, not a release, so `gix update` says so and leaves it alone unless
you pass `--force`.

gix never checks for updates on its own. If it is installed somewhere you
cannot write (for example `/usr/local/bin`), run `sudo gix update`.

## Known limits

These are what the code does today, listed so the docs do not promise
more.

- **`--tag` on `finish`** is registered from the built-in defaults, so it
  exists on `hotfix` and `release` only. Setting `tag: true` on another
  kind in the config does not add the flag, and neither does a kind you
  add yourself.
- **`finish` deletes with `-D`**, after the merges succeeded, so that
  squash and rebase work. `branch prune --apply` is the command that
  never uses `-D`.
- **`gix status`** picks a branch's kind by the first matching prefix,
  while `branch doctor`, `sync`, and `prune` use the longest. With
  overlapping prefixes the two can disagree.
- **`autosync` on a base branch** is read by the branch's own name, so a
  renamed base branch (say `main` called `trunk`) does not pick up its
  `autosync` flag in `branch doctor`.
- **A branch fast-forwarded into its target** points at the same commit as
  that target afterwards, so `branch prune` cannot tell it from a branch
  you just started and keeps it. `finish` deletes such a branch itself.
- **`gix branch sync` needs Git 2.38 or newer** for `git merge-tree
  --write-tree`. Older versions get an error up front, and `gix doctor`
  warns about it.
- **Nothing fetches.** Ahead and behind counts, and `gone` upstreams, are
  as of your last `git fetch`.
