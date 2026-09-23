# .husky — git hooks

Plain git hooks (no husky/npm install needed). Enable them once per
clone:

```bash
git config core.hooksPath .husky    # or: make hooks
```

On macOS/Linux also make them executable once:

```bash
chmod +x .husky/pre-commit .husky/commit-msg .husky/pre-push
```

## Hooks

| Hook | Runs | What it does |
|------|------|--------------|
| `pre-commit` | `git commit` (before) | `gofmt` + `go vet` |
| `commit-msg` | `git commit` | Conventional Commits format ([rules](../quality/commitlint/)) |
| `pre-push` | `git push` | `go test ./...` |

All are `sh` scripts — they work with Git Bash / WSL on Windows.

## Skip a hook (rarely needed)

```bash
git commit --no-verify
```

## Disable

```bash
git config --unset core.hooksPath
```
