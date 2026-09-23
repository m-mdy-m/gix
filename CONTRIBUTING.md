# Contributing

Thanks for considering a contribution.

**Before you start:** gix automates *one* branching flow (main →
develop → feature/bugfix/hotfix/release). PRs that add a second
workflow, a plugin system, or general Git features are out of scope —
check [the intro](docs/Introduction.md) first. Bug fixes, docs, tests
and flow-related flags are always welcome.

## Setup

```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
make build        # → build/gix
make hooks        # enable git hooks, once per clone
```

Needs Go 1.25+ and `git` on PATH.

## Workflow

```bash
gix feature start my-change
# ...work, commit normally...
gix feature finish my-change
```

The repo uses its own flow. Branch from `develop`, finish back into it.

## Commits

[Conventional Commits](https://www.conventionalcommits.org/), enforced
by the `commit-msg` hook:

```
feat(config): support custom branch kinds
fix(flow): don't delete branch when merge fails
docs: shorten getting-started
```

Types: `feat` `fix` `docs` `style` `refactor` `perf` `test` `build`
`ci` `chore` `revert`.

## Before opening a PR

```bash
make check    # fmt-check + vet + test — same as CI
```

Also:

- one logical change per PR (small beats big)
- update docs if behavior changed (`docs/`)
- add a `CHANGELOG.md` entry under `[Unreleased]` for user-facing changes

## Where things live

| Path | Contents |
|------|----------|
| `internal/flow` | start/finish logic — the heart of gix |
| `internal/git` | exec + parse wrapper around the `git` CLI |
| `internal/config` | `.gix/config` read/write/validate |
| `internal/commands` | cobra definitions (thin) |
| `internal/ui` | output helpers |
| `docs/` | user-facing docs |
| `quality/` | lint/format tool configs |
| `.husky/` | git hooks |

Details: [docs/architecture](docs/architecture/README.md).

## Reporting bugs

Use [Issues](https://github.com/m-mdy-m/gix/issues) with the bug report
template. Security issues → [SECURITY.md](SECURITY.md), not a public
issue.
