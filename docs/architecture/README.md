# Architecture

Small codebase, one job per package.

```
cmd/gix/            entry point — builds the cobra root, runs it
internal/
  cli/              command context + global flags (--verbose, --debug…)
  commands/         cobra command definitions (thin: parse → call → print)
  config/           read/write/validate .gix/config
  flow/             THE flow logic: init, start, finish, strategies, health, doctor, signing, branch health, sync, version
  git/              thin wrapper that shells out to the `git` binary
  ui/               colored output helpers
  logger/           --verbose / --debug logging
  testutil/         throwaway git repos for tests (test-only)
```

## The one rule

**`internal/flow` never touches os/exec directly — it calls
`internal/git`.** `internal/git` is the only package that runs `git`.

Why: gix doesn't reimplement Git. It composes real `git` commands, so a
gix-managed repo is a plain Git repo.

## Data flow

```
cobra command (internal/commands)
   → loads .gix/config (internal/config)
   → calls a flow function (internal/flow)
      → runs git commands (internal/git)
   → prints a result (internal/ui)
```

## Where to change what

| Change | Where |
|--------|-------|
| new command/flag | `internal/commands` + register in `registrables` (root.go) |
| merge/tag/delete behavior | `internal/flow/branch.go`, `flow/strategy.go` |
| branch state, prune planning | `internal/flow/branchhealth.go` |
| syncing a source branch into topic branches | `internal/flow/sync.go` |
| release versions and tags | `internal/flow/version.go` |
| config schema/validation | `internal/config` |
| git command output parsing | `internal/git` |
| terminal output style | `internal/ui` |

## Reading Git, not re-deriving it

Where Git already answers a question, gix asks Git and parses the
answer rather than recomputing it:

| Question | Source |
|----------|--------|
| identity, signing, scopes | `git config --show-scope --list` |
| ahead/behind, gone upstream | `git for-each-ref %(upstream:track)` |
| would two branches conflict | `git merge-tree --write-tree` |
| highest release version | `git tag --sort=-v:refname` |
| branch in the flow, HEAD state, clean tree | `git rev-parse`, `git status` |
| branch already merged | `git merge-base --is-ancestor` |

Reading the whole config as a list, rather than one
`git config --get` per key, is deliberate: a missing key exits
non-zero exactly like a genuine failure, so the two can't be told
apart from the error alone.

## Adding a command

One feature is one self-contained file (or two: command + logic) plus
one line in `registrables`:

```
internal/commands/<feature>.go    cobra command(s), thin: parse → call → print
internal/flow/<feature>.go        the logic, if there is any
internal/git/…                    new git wrappers, if needed
internal/config/…                 new config fields, if needed
internal/commands/root.go         +1 line in registrables
```

Rules that keep features independent, so one can be deleted without
disturbing the others:

- a feature file never imports another feature's symbols
- shared code goes in `cli` (flags), `ui` (output), `git` (wrappers),
  `config` (schema) — nothing else is a shared layer
- private helpers live in their own feature file, so deleting the file
  can't break the build
- unknown YAML keys are ignored on load, so removing a config key never
  breaks an older `.gix/config`

## Tests

gix shells out to the real `git`, so tests use real repositories in a
temp directory (`internal/testutil`) with Git's config isolated from
whatever the machine has set. No mocking of git.

| Test | What it needs |
|------|---------------|
| `go test ./...` | `git` on `PATH` |
| `go test -race ./...` | a C compiler (CI only) |
