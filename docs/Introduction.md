# Introduction

## What gix is

gix automates one specific Git branching flow:

```
main
 └── develop
      ├── feature/*   from develop → back to develop
      ├── bugfix/*    from develop → back to develop
      ├── hotfix/*    from main → main + develop, taggable
      └── release/*   from develop → main + develop, taggable
```

Instead of five Git commands per branch, you run two: `start` and
`finish`. The exact names, prefixes and merge rules live in
`.gix/config` — see [Configuration](configuration.md).

## What it is not

Not a Git replacement, not a general Git UI, not a workflow engine for
arbitrary teams. It automates this one flow and nothing else.

## Relationship with Git

gix shells out to the real `git` binary. Every repo it manages is an
ordinary Git repository — `git log`, `git merge`, your IDE's Git
integration all keep working with or without gix.

## Next

- First time? → [Getting started](getting-start.md)
- Day-to-day use → [Examples](examples.md)
- All commands → [Commands](api/README.md)
