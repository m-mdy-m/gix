# Contributing

Thanks for helping improve gix.

## Setup

Requirements: Go 1.25+, Git, and Make.

```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
make setup
```

## Workflow

Use gix for the repository flow:

```bash
gix feature start my-change
# work and commit normally
gix feature finish my-change
```

Run checks before opening a PR:

```bash
make quality
make lint
```

## Commits

Use Conventional Commits, for example:

```text
feat(flow): add hotfix support
fix(config): validate branch targets
```

## Pull requests

Keep a PR focused on one logical change. Include what changed, why, and any user-visible behavior.

The CI must pass before merge.

## Code layout

- `internal/git` — Git process boundary.
- `internal/flow` — branching and merge logic.
- `internal/config` — `.gix/config` model and validation.
- `internal/commands` — CLI commands.
- `internal/ui` — terminal output.
- `cmd/gix` — application entry point.
