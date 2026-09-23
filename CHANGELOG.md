# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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

[Unreleased]: https://github.com/m-mdy-m/gix/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/m-mdy-m/gix/releases/tag/v0.1.0