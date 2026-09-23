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
| `make fmt` / `make fmt-check` | gofmt write / check |
| `make vet` | `go vet ./...` |
| `make check` | fmt-check + vet + test (what CI runs) |
| `make hooks` | `git config core.hooksPath .husky` |
| `make docker` | build the Docker image |
| `make clean` | remove `build/` |

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
3. Tag and push: `git tag vx.y.z && git push --tags`
4. CI (`.github/workflows/release.yml`) builds all binaries, creates the
   GitHub Release with checksums, and pushes the Docker image

Docker Hub credentials needed as secrets: `DOCKER_USERNAME`,
`DOCKER_TOKEN`.

## Project layout

See [Architecture](../architecture/README.md).
