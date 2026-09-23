# scripts

Installers for gix. Both download the prebuilt binary from
[GitHub Releases](https://github.com/m-mdy-m/gix/releases), verify the
platform, and put `gix` on your PATH.

## install.sh — Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | sh
```

Options (env vars):

| Var | Default | Meaning |
|-----|---------|---------|
| `VERSION` | latest | install a specific tag, e.g. `VERSION=v1.2.3` |
| `PREFIX` | `/usr/local` (or `~/.local` if not writable) | install prefix → `$PREFIX/bin/gix` |

## install.ps1 — Windows

```powershell
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1" -OutFile install.ps1
.\install.ps1
```

Parameters: `-Version vX.Y.Z`, `-Prefix <dir>` (default
`%LOCALAPPDATA%\Programs\gix`). Adds the folder to your user PATH.

## Uninstall

```bash
rm "$(command -v gix)"            # Linux/macOS
Remove-Item "$env:LOCALAPPDATA\Programs\gix\gix.exe"   # Windows
```

## Notes

- Checksums (`.sha256`) are published next to every release archive.
- Prefer building from source? `make install` — see
  [docs/development](../docs/development/README.md).
