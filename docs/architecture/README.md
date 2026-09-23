# Architecture

Small codebase, one job per package.

```
cmd/gix/            entry point — builds the cobra root, runs it
internal/
  cli/              command context + global flags (--verbose, --debug…)
  commands/         cobra command definitions (thin: parse → call → print)
  config/           read/write/validate .gix/config
  flow/             THE flow logic: init, start, finish, strategies, health
  git/              thin wrapper that shells out to the `git` binary
  ui/               colored output helpers
  logger/           --verbose / --debug logging
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
| config schema/validation | `internal/config` |
| git command output parsing | `internal/git` |
| terminal output style | `internal/ui` |
