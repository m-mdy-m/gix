# gix

A personal wrapper around Git for one specific branching flow. Not a
general-purpose Git replacement, not a configurable workflow engine —
it automates the flow below, so branch creation, merging, and cleanup
are one command instead of five.

```
main
 └── develop
      ├── feature/*   branched from develop, merged back into develop
      └── release/*   branched from develop, merged into main + develop, tagged
```

## Why

Every feature and release in this flow needs the same sequence of Git
commands: check out develop, branch, work, come back, merge with
`--no-ff`, delete the branch, tag if it's a release. gix turns that
sequence into `gix feature start`, `gix feature finish`,
`gix release start`, `gix release finish`.

## Install

```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
make install
```

This builds `gix` and installs it to your `$GOPATH/bin` (or `go install`'s
default). Make sure that's on your `PATH`.

## Usage

Initialize gix in a repository (creates `.gix/config`, commits it, and
creates the `develop` branch if it doesn't exist yet):

```bash
gix flow init
```

Start a feature:

```bash
gix feature start api-auth
# → creates and switches to feature/api-auth, from develop
```

Work, commit normally with plain `git commit`, then finish:

```bash
gix feature finish api-auth
# → merges feature/api-auth into develop (--no-ff)
# → deletes the feature branch
```

Releases work the same way, but merge into both `main` and `develop`,
and can be tagged:

```bash
gix release start v0.1.0
# ...commit release prep...
gix release finish v0.1.0 --tag v0.1.0
# → merges release/v0.1.0 into main, then develop
# → tags v0.1.0 on main
# → deletes the release branch
```

Check where you are:

```bash
gix status   # current branch, base, ahead/behind, clean/dirty
gix list     # active feature/ and release/ branches
```

## Configuration

`gix flow init` writes `.gix/config` at the repo root:

```
version = 1
branch.main = main
branch.develop = develop
prefix.feature = feature/
prefix.release = release/
remote = origin
```

It's a plain committed file, not a personal dotfile — everyone working
in the repo shares the same branch model. Edit it by hand if you need
different branch names or prefixes.

## What this is not (yet)

> This is an early MVP. It intentionally does not do commit message
generation, hooks, safety backups, or anything beyond the branch flow
itself — those were planned in earlier drafts of this project but
weren't implemented, so they've been dropped from scope rather than
left as empty stubs. If they come back, they'll be built the same way
the flow commands were: implemented before they're documented.

## Development

```bash
make build   # build ./build/gix
make test    # go test ./...
make clean   # remove build/
```

## License

MIT. See [LICENSE](./LICENSE).