# Commands

Every command, one line each. `gix <command> --help` has the full text.

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
| `gix flow init` | create `.gix/config`, commit it, create `develop` |

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

`gix config set` flags: `--prefix`, `--upstream-strategy`,
`--downstream-strategy`, `--tag`, `--delete-on-finish`.
