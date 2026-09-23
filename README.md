# gix

A small command-line wrapper around Git for one specific branching flow. gix is not a Git replacement or a general workflow engine; it turns a repeatable branch/merge sequence into a few commands.

```text
main
 └── develop
      ├── feature/*
      ├── bugfix/*
      ├── hotfix/*
      └── release/*
```

## Features

- **Flow-aware** — branch parents and merge targets come from `.gix/config`.
- **Simple** — `start` and `finish` cover the common flow.
- **Safe by default** — refuses dirty working trees and unfinished Git operations.
- **Taggable releases** — release/hotfix branches can create annotated tags.
- **Cross-platform** — Linux, macOS, and Windows binaries are published.

## Installation

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | sh
```

### Windows PowerShell

```powershell
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1" -OutFile install.ps1
.\install.ps1
```

### From source

Requirements: Go 1.25+ and Git.

```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
make install
```

### Docker

```bash
docker pull bitsgenix/gix:latest
docker run --rm -v "$PWD":/workspace -w /workspace bitsgenix/gix:latest status
```

See [Installation](docs/INSTALLATION.md) for all options.

## Quick Start

Initialize gix in an existing Git repository:

```bash
gix flow init
```

Start a feature:

```bash
gix feature start api-auth
# work and commit normally
gix feature finish api-auth
```

Create a release:

```bash
gix release start v0.2.0
# release work + commits
gix release finish v0.2.0 --tag v0.2.0
```

Inspect the flow:

```bash
gix status
gix list --tree
gix config list
```

## Development

```bash
make setup       # enable Git hooks
make build       # build/gix
make test        # tests
make quality     # fmt-check + test + vet
make lint        # golangci-lint
make ci          # quality + lint
make docker      # build Docker image
```

## Documentation

See [docs/](docs/README.md) for commands, configuration, installation, architecture, and release notes.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Acknowledgements

Project standards (readme, changelog, docs) are enforced with
[psx](https://github.com/m-mdy-m/psx) — see [psx.yml](psx.yml).

## License

MIT — see [LICENSE](LICENSE).
