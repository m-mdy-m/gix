# Development

Working on gix itself (using gix? see [CONTRIBUTING](../../CONTRIBUTING.md)).

## Requirements

- Go 1.25+
- `git` on PATH (gix shells out to it)
- `make`

## Setup

```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
make build          # → build/gix
make hooks          # enable git hooks (once per clone)
```

## Make targets

| Target | Does |
|--------|------|
| `make build` | build → `build/gix` |
| `make install` | `go install` to `$GOPATH/bin` |
| `make test` | `go test ./...` |
| `make cover` | `go test -cover ./...` |
| `make fmt` / `make fmt-check` | gofmt write / check |
| `make vet` | `go vet ./...` |
| `make check` | fmt-check + vet + test (what CI runs) |
| `make hooks` | `git config core.hooksPath .husky` |
| `make docker` | build the Docker image |
| `make clean` | remove `build/` |

## Tests

Tests use real Git repositories in a temp directory — see
`internal/testutil`. `git` must be on `PATH`; without it the tests skip
rather than fail. Git's config is isolated per test, so a developer's own
`user.name`, `core.hooksPath`, or signing setup cannot change results, and
tests pass the same way on Linux, macOS, and Windows.

```bash
make test                          # everything
go test ./internal/config/         # one package
go test -run TestStart ./...       # one test
```

## Before pushing

Hooks do most of it automatically (see [.husky/README](../../.husky/README.md)):

```bash
make check
```

Commits must follow Conventional Commits — the `commit-msg` hook
enforces it: `feat(ui): add login form`.

## Release flow

1. Update `CHANGELOG.md` — add `## [x.y.z] - YYYY-MM-DD`
2. Commit on `main`
3. Tag and push: `gix version bump patch|minor|major`, then
   `git push origin vX.Y.Z`
4. CI (`.github/workflows/release.yml`) builds all binaries, creates the
   GitHub Release with checksums, and pushes the Docker image

`gix version bump` only picks the number and creates the annotated tag —
it does not edit the changelog, commit, sign, or push.

Docker Hub credentials needed as secrets: `DOCKER_USERNAME`,
`DOCKER_TOKEN`.

## Project layout

See [Architecture](../architecture/README.md).
