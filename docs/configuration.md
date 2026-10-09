# Configuration

gix keeps its settings in `.gix/config` at the repository root — a
plain YAML file you can read, edit, or change with `gix config set`.
Nothing else is stored outside your repository.

```bash
gix config list     # print the active config
gix config set <kind> [flags]   # change one branch kind
```

`gix flow init` writes the file, commits it, and creates the develop
branch. `gix doctor` tells you when it's missing.

## Top-level keys

| Key | Meaning |
|-----|---------|
| `gix.version` | Schema version of this file. Written automatically; checked loosely on load. |
| `remote` | Remote name gix expects to find. Defaults to `origin`. `gix doctor` reports if it doesn't exist. |
| `branch` | The branch model: one entry per base branch and per topic kind. |

## Base and topic kinds

Every entry under `branch` has a `type`:

- **`base`** — a long-lived branch (`main`, `develop`). Base branches are
  what topic branches branch from and merge back into.
- **`topic`** — a short-lived branch (`feature/*`, `bugfix/*`, `hotfix/*`,
  `release/*`), created with `gix <kind> start` and finished with
  `gix <kind> finish`.

| Key | Applies to | Meaning |
|-----|-----------|---------|
| `type` | both | `base` or `topic`. |
| `name` | base | Real branch name. Defaults to the key, so `main: {name: trunk}` points gix at a branch called `trunk`. |
| `prefix` | topic | Branch name prefix. `feature/` turns `gix feature start api` into branch `feature/api`. Required. |
| `parent` | topic | Config key of the branch this kind branches from. |
| `merge_into` | topic | Config keys to merge into on finish. Falls back to `parent`. |
| `upstream_strategy` | topic | How `finish` folds the branch in: `merge`, `rebase`, or `squash`. Defaults to `merge`. |
| `downstream_strategy` | topic | How changes flow back out. Defaults to `merge`. |
| `tag` | topic | `true` lets `finish` accept `--tag`. |
| `autosync` | both | Marks the branch as one that should receive updates from its parent. See [Auto sync](#auto-sync). |
| `delete_on_finish` | both | Whether `finish` removes the branch after merging. |

### What validation rejects

`config set` and every load refuse to save a file that:

- has no branches at all, or an unknown `type`
- has a topic kind with no `prefix`, or a prefix Git won't accept as a
  branch name component
- has an `upstream_strategy`/`downstream_strategy` other than `merge`,
  `rebase`, or `squash`
- has a topic kind with no `parent` and no `merge_into`
- merges into itself, into an undefined branch, or into another topic kind
- has circular `parent` dependencies

## A default config

```yaml
# gix flow config — see `gix config list`
gix:
  version: "1"
remote: origin
branch:
  main:
    name: main
    type: base
  develop:
    name: develop
    type: base
    parent: main
    autosync: true
  feature:
    type: topic
    prefix: feature/
    parent: develop
    upstream_strategy: merge
    downstream_strategy: merge
    delete_on_finish: true
  bugfix:
    type: topic
    prefix: bugfix/
    parent: develop
    upstream_strategy: merge
    downstream_strategy: merge
    delete_on_finish: true
  hotfix:
    type: topic
    prefix: hotfix/
    parent: main
    merge_into:
    - main
    - develop
    upstream_strategy: merge
    downstream_strategy: merge
    tag: true
    delete_on_finish: true
  release:
    type: topic
    prefix: release/
    parent: develop
    merge_into:
    - main
    - develop
    upstream_strategy: merge
    downstream_strategy: merge
    tag: true
    delete_on_finish: true
```

## Adding your own kind

A kind is just an entry. Unknown YAML keys are ignored on load, so a
config written by a newer gix still loads in an older one.

```yaml
branch:
  wip:
    type: topic
    prefix: wip/
    parent: develop
    upstream_strategy: squash
    delete_on_finish: true
```

Once defined, `gix config list` shows it and `gix list` includes its
branches. Getting a `gix wip start|finish` command group takes one more
step: the built-in command groups are registered from a fixed list in
`internal/commands/root.go`, so a new kind needs to be added there.

## Auto sync

`autosync: true` marks a branch as one that should receive updates from
its `parent`. `gix branch sync --auto` updates only the branches marked
this way.

Both roles can carry the flag:

```yaml
branch:
  develop:
    name: develop
    type: base
    parent: main
    autosync: true      # gix branch sync --from=main --auto merges main into develop

  feature:
    type: topic
    prefix: feature/
    parent: develop
    autosync: true      # gix branch sync --from=develop --auto merges develop into feature branches
```

```bash
gix branch sync --from=develop --auto --dry-run   # preview
gix branch sync --from=develop --auto             # do it
```

What `--auto` is, and is not:

- It is a **filter**. Without it, `gix branch sync` already covers every
  topic branch, so marking a kind does not change the default run.
- Nothing happens automatically. There are no hooks, no background
  process, and no CI integration — gix only acts when you run a command.
- It merges only. A branch is never rebased, force-pushed, reset, or
  deleted, and a conflict is reported rather than resolved.
- A base branch is only updated when its own entry says `autosync: true`.
  `main` has no parent and is never a target.

`gix config set <kind> --autosync=true|false` works for topic kinds.
Base branches have to be edited in the file, because `gix config set`
only handles topic kinds.

## Changing a kind

```bash
gix config set release --upstream-strategy=squash
gix config set feature --prefix=feat/
gix config set hotfix --tag=false
```

Only the flags you pass change; the rest of the file is left alone.
`gix config set` writes the file but does not commit it — commit it
yourself when you're happy with the change.

| Flag | Value |
|------|-------|
| `--prefix` | new branch name prefix |
| `--upstream-strategy` | `merge`, `rebase`, or `squash` |
| `--downstream-strategy` | `merge` or `rebase` |
| `--tag` | `true` or `false` |
| `--delete-on-finish` | `true` or `false` |

## Related

- [Commands](api/README.md) — every command and flag
- [Architecture](architecture/README.md) — where config is read and written