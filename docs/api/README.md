# Commands

Every command, one line each. `gix <command> --help` has the full text,
and [Commands in depth](../commands.md) walks through what each one does
step by step.

## Global flags

| Flag | Effect |
|------|--------|
| `--verbose` | print each `git` command gix runs |
| `--debug` | verbose + raw git output and timing |
| `--no-color` | disable colors (also honors `NO_COLOR`) |
| `--version` | print gix version |

## Setup

| Command | What it does |
|---------|--------------|
| `gix init` | create a Git repository, ready for `gix flow init` |
| `gix flow init` | create `.gix/config`, commit it, create `develop` |

`gix init` flags: `--initial-branch`, `--no-commit`,
`--author-name` + `--author-email` (commit without a global git
identity). Safe to re-run: on a repository that already has commits it
does nothing.

`gix flow init` flags: `--main`, `--develop`, `--no-commit`,
`--author-name` + `--author-email` (commit the config without a global
git identity).

## Branch kinds

Same shape for `feature`, `bugfix`, `hotfix`, `release`:

| Command | What it does |
|---------|--------------|
| `gix <kind> start <name>` | create branch from its parent, switch to it |
| `gix <kind> finish <name>` | merge into configured targets, tag?, delete? |

`finish` flags:

| Flag | Effect |
|------|--------|
| `--strategy <s>` | `merge` \| `rebase` \| `squash` (overrides config) |
| `--no-delete` | keep the branch after merging |
| `--tag <name>` | annotated tag on `main` after merging (hotfix/release only) |

Both require a **clean working tree** and no unfinished
merge/rebase in progress.

## Inspection

| Command | What it does |
|---------|--------------|
| `gix status` | current branch, its role in the flow, ahead/behind, dirty state |
| `gix list` | open branches per kind (`--tree` for tree view) |
| `gix config list` | print the active flow config |
| `gix config set <kind> ...` | change one kind's settings |
| `gix doctor` | check the repository for anything that would stop gix working |
| `gix signing status` | how commit/tag signing is configured |

`gix config set` flags: `--prefix`, `--upstream-strategy`,
`--downstream-strategy`, `--tag`, `--delete-on-finish`, `--autosync`.

## Releases

| Command | What it does |
|---------|--------------|
| `gix version show` | the current release version and how many commits since |
| `gix version bump <level>` | create the next release tag |

There is no version file to keep in step with: the tags are the record.
A tag gix cannot read as an exact SemVer version (`major.minor.patch`,
optionally `-prerelease`) is ignored rather than guessed at, so `v1.2`,
`nightly`, and `v1.2.3+build` are skipped. The latest version is chosen
by SemVer precedence: `alpha.1 < beta.1 < rc.1 < rc.2 < rc.10 < 1.0.0`.

| Level | From | To |
|-------|------|----|
| `patch` | `0.4.3` | `0.4.4` |
| `minor` | `0.9.4` | `0.10.0` |
| `major` | `0.4.3` | `1.0.0` |

With no tags yet, the first release is `0.1.0`.

### Prereleases

| Command | From | To |
|---------|------|----|
| `gix version bump minor --pre rc` | `0.4.3` | `0.5.0-rc.1` |
| `gix version bump --pre rc` | `1.0.0-rc.1` | `1.0.0-rc.2` |
| `gix version bump --pre rc` | `1.0.0-beta.2` | `1.0.0-rc.1` |
| `gix version bump --release` | `1.0.0-rc.2` | `1.0.0` |
| `gix version bump minor` | `1.0.0-rc.1` | `1.1.0` |
| `gix version bump patch` | `1.0.0-rc.1` | `1.0.1` |
| `gix version bump major` | `1.0.0-rc.1` | `2.0.0` |

- `--pre <name>` takes a name such as `alpha`, `beta`, or `rc`. With a
  level it starts a prerelease; without one it continues the current
  prerelease. It is refused on a stable version.
- `--release` promotes the current prerelease as it is. It takes no
  level and cannot be combined with `--pre`.
- A level on its own never promotes: it makes the next stable version
  from the core of the current one.
- A prerelease can only move forward, so `--pre beta` on `1.0.0-rc.1` is
  refused.
- Build metadata (`+build`) is not supported.

```bash
gix version bump minor --dry-run   # see it first
gix version bump minor             # create v0.3.0
```

`gix version bump` does one thing: it calculates the number and creates
one annotated tag.

It does **not**:

- edit `CHANGELOG.md`
- make a release commit
- sign the tag
- push anything

Those are yours to decide. Make your release commit, tag it, push the
tag:

```bash
git push origin v0.3.0
```

It refuses to run on a feature branch, a detached HEAD, a dirty working
tree, during an unfinished merge, or when the tag already exists. Run it
on `main`.

## Updating gix

| Command | What it does |
|---------|--------------|
| `gix update` (alias `gix upgrade`) | replace this gix with the latest release |

Flags: `--check` (only report), `--to <tag>` (a specific release),
`--force` (reinstall, or replace a development build).

## Diagnostics

`gix doctor` inspects the repository and reports what it finds:
whether the flow is set up, whether HEAD and the working tree are in a
state gix can work with, whether a remote and an upstream exist,
whether a Git identity is configured, how signing is set up, and
whether the base branches `.gix/config` names actually exist.

It only reports. Nothing is rewritten, no commits are touched, and no
history is rewritten.

| Flag | Effect |
|------|--------|
| `--fix` | apply the fixes you asked for by name, or print the exact command for each problem |
| `--set-user-name <name>` | set `user.name` (requires `--set-user-email`) |
| `--set-user-email <email>` | set `user.email` (requires `--set-user-name`) |
| `--scope <scope>` | `local` (this repository, default) or `global` (all repositories) |
| `--strict` | exit non-zero when anything needs attention, for use in CI |

gix never guesses a name or email, and never overwrites an identity you
didn't ask it to:

```bash
gix doctor --fix --set-user-name "Your Name" --set-user-email "you@example.com"
gix doctor --fix --scope global --set-user-name "Your Name" --set-user-email "you@example.com"
```

Run `--fix` with no name or email and it prints the exact command for
each problem without changing anything.

`gix doctor` exits 0 even when it finds problems: it reports, and
deciding what should block is yours. `--strict` makes it exit non-zero
instead, which is what you want in CI:

```bash
gix doctor --strict || exit 1
```

## Branch inspection

| Command | What it does |
|---------|--------------|
| `gix branch doctor` | one row per branch: kind, upstream, ahead/behind, state |
| `gix branch prune` | list the branches that could be cleaned up, or delete them |
| `gix branch sync` | bring a source branch into every topic branch |

`doctor` only reports. `prune` only reports unless you pass `--apply`.
`sync` merges.

`gix branch doctor` reports a state per branch:

| State | Meaning |
|-------|---------|
| `tracked` | tracks a remote branch and matches it |
| `ahead` | has commits its upstream doesn't — a push would be needed |
| `behind` | its upstream moved on — those commits are not merged in |
| `diverged` | ahead and behind at once — a plain push would be rejected |
| `untracked` | has no upstream configured |
| `gone` | had an upstream, but that ref no longer exists locally |

| Flag | Effect |
|------|--------|
| `--all` | include branches that are in sync and healthy |

The states come from Git's own `%(upstream:track)`, so `gone` is
genuinely distinguished from "never had an upstream". `behind` does not
mean anything is wrong with your commits — it means the remote branch
moved on.

`gix branch prune` lists branches whose configured merge targets
**already contain their work**, plus branches whose upstream is gone —
the leftovers after a finished feature.

```bash
gix branch prune
```

It is a preview by default: it deletes nothing.

```bash
gix branch prune             # show what could be cleaned up
gix branch prune --apply     # delete what is fully merged
```

`--apply` deletes only branches whose configured merge targets already
contain their work, and it uses `git branch -d` — never `-D`. Git itself
refuses to delete a branch whose commits are not reachable from its
upstream, and that refusal is the safety net this relies on.

A branch whose upstream merely disappeared is listed but **not**
deleted: a missing upstream does not mean its work was merged. Remove it
yourself with `git branch -D` if you are sure.

Base branches and the branch you are standing on are never candidates,
and a branch with unmerged work is never a candidate. `--apply` refuses
to run on a dirty tree or during an unfinished merge.

### `gix branch sync`

Brings a source branch into every topic branch, so a change that landed
on `develop` reaches work in progress.

| Flag | Effect |
|------|--------|
| `--from <branch>` | branch to bring in (default: the base branch you are on, or the first configured base) |
| `--dry-run` | report what would happen without changing anything |
| `--auto` | only update branches `.gix/config` marks `autosync: true` |

```text
$ gix branch sync --dry-run
→ dry run — nothing was changed
BRANCH              WOULD
feature/clean       would merge
feature/conflicted  ✗ conflict
  • feature/conflicted: conflicting files: shared.txt

this was a preview — gix has not changed anything
```

Sync is deliberately conservative:

- it refuses to run on a dirty working tree, or during an unfinished
  merge or rebase
- branches that already contain the source are skipped, so running it
  twice changes nothing the second time
- conflicts are detected **before** anything is merged, using
  `git merge-tree` — no temporary branch, no checkout
- a conflict is reported and left alone; gix never resolves one for you
- one branch failing does not stop the rest, and the command exits
  non-zero so CI notices
- base branches and the source are never modified
- only merging is used. A kind whose `downstream_strategy` is `rebase`
  or `squash` is **skipped** with a message, because rebase would
  rewrite that branch's history
- you're put back on the branch you started on
- `--dry-run` works even on a dirty tree: a preview that refuses to run
  would hide the situation you wanted to look at

#### `--auto`

`--auto` narrows a run to the branches `.gix/config` marks
`autosync: true`, so you can update one kind or one base branch instead
of all of them:

```bash
gix branch sync --from=main --auto --dry-run
```

```text
BRANCH   WOULD
develop  would merge
```

On a base branch, `autosync: true` means "take updates from my
`parent`" — which is how the default config already describes
`develop` following `main`. A base branch that is not marked is never
merged into.

`--auto` is a filter, not a switch: without it `gix branch sync`
already covers every topic branch. And nothing runs by itself — there
are no hooks and no background process. See
[Configuration](configuration.md#auto-sync).

## Signing

`gix signing status` reports the signing format (`openpgp`, `ssh`, or
`x509`), the configured key, whether commits and tags are signed, and
whether git has what it needs to do it.

gix only reads configuration and looks for the signing program on
`PATH`. It never signs anything, never creates keys, and never prints
key material.

```bash
gix signing status
```
