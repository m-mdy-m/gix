# Examples

Real sequences, copy-paste style. Assumes `gix flow init` was run.

## Normal feature

```bash
gix feature start login-form
# hack, then:
git add -A && git commit -m "feat(ui): add login form"
gix feature finish login-form
# → merged into develop, feature/login-form deleted
```

## Bug in develop

```bash
gix bugfix start null-pointer
# fix it, commit...
gix bugfix finish null-pointer
```

## Production is on fire

```bash
gix hotfix start crash-on-startup
# fix on a branch off main...
git commit -am "fix: crash on startup"
gix hotfix finish crash-on-startup --tag v1.0.1
# → merged into main AND develop, tagged on main, branch deleted
```

## Release

```bash
gix release start v1.1.0
# bump version, update CHANGELOG, commit...
gix release finish v1.1.0 --tag v1.1.0
```

## Keep the branch after merging

```bash
gix feature finish login-form --no-delete
```

## Squash instead of merge commit

```bash
gix feature finish login-form --strategy squash
# strategies: merge | rebase | squash
```

## See what's going on

```bash
gix status           # where am I?
gix list             # flat list of open branches
gix list --tree      # tree view of the whole flow
gix config list      # the active flow config
```

## Something looks wrong

```bash
gix status --verbose   # prints every git command gix runs
gix status --debug     # + raw git output and timing
```
