# scripts

Installers for gix. Both download the prebuilt binary from
[GitHub Releases](https://github.com/m-mdy-m/gix/releases), verify the
platform, and put `gix` on your PATH.

## install.sh - Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | sh
```

Options (env vars, set on `sh`, not on `curl`):

| Var | Default | Meaning |
|-----|---------|---------|
| `GIX_VERSION` | latest | install a specific tag, e.g. `GIX_VERSION=v1.2.3` |
| `GIX_PREFIX` | `/usr/local` (or `~/.local` if not writable) | install prefix -> `$PREFIX/bin/gix` |

```bash
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | GIX_VERSION=v1.0.1 sh
```

`VERSION` and `PREFIX` still work. Needs `curl` or `wget`.

## install.ps1 - Windows

```powershell
irm https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1 | iex
```

Options are environment variables, so they work through `| iex`:

```powershell
$env:GIX_VERSION = "v1.0.1"                      # default: latest
$env:GIX_PREFIX  = "$env:LOCALAPPDATA\Programs"   # gix.exe goes in <prefix>\gix
irm https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1 | iex
```

Picks `windows-arm64` on ARM machines. Adds the folder to your user PATH,
for the current window and for new ones.

## Uninstall

```bash
rm "$(command -v gix)"            # Linux/macOS
Remove-Item "$env:LOCALAPPDATA\Programs\gix\gix.exe"   # Windows
```

## Notes

- Checksums (`.sha256`) are published next to every release binary and archive; both installers verify them.
- Prefer building from source? `make install` — see
  [docs/development](../docs/development/README.md).
