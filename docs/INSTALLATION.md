# Installation

Four ways. Pick one.

## 1. Quick install (recommended)

One command, no admin rights.

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | sh
```

Installs to `/usr/local/bin`, or `~/.local/bin` if that is not writable.

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1 | iex
```

Installs to `%LOCALAPPDATA%\Programs\gix` and adds it to your user PATH,
for the current window and for new ones.

Both verify the SHA-256 checksum before installing.

**Options** are environment variables, so they work through the pipe:

```bash
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | GIX_VERSION=v1.0.1 sh
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | GIX_PREFIX=$HOME/.local sh
```

```powershell
$env:GIX_VERSION = "v1.0.1"; irm https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1 | iex
```

Set the variable on `sh`, not on `curl`: `GIX_VERSION=v1 curl ... | sh`
sets it for curl only.

## 2. Prebuilt binary

Download one file from [Releases](https://github.com/m-mdy-m/gix/releases)
and run it. Each is a single static binary:

| Asset | Platform |
|-------|----------|
| `gix-linux-x64` / `gix-linux-arm64` | Linux |
| `gix-darwin-x64` / `gix-darwin-arm64` | macOS (Intel / Apple silicon) |
| `gix-windows-x64.exe` / `gix-windows-arm64.exe` | Windows |

Every binary has a `.sha256` next to it. Archives
(`gix-<ver>-<target>.tar.gz` / `.zip`) are still published.

```bash
curl -fsSLO https://github.com/m-mdy-m/gix/releases/latest/download/gix-linux-x64
chmod +x gix-linux-x64
sudo mv gix-linux-x64 /usr/local/bin/gix
gix --version
```

On Windows, download `gix-windows-x64.exe`, rename it to `gix.exe`, and
put it in a folder on your PATH.

## 3. From source

Requires Go 1.25+ and Git.

```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
make install        # → $GOPATH/bin
```

Make sure `$GOPATH/bin` is on your PATH (`go env GOPATH`).

## 4. Docker

```bash
docker pull bitsgenix/gix:latest
```

gix changes files in your repo, so pass your user to keep ownership
correct:

```bash
docker run --rm -u "$(id -u):$(id -g)" \
  -v "$PWD":/work -w /work bitsgenix/gix:latest status
```

(Windows: drop `-u`, Docker Desktop handles ownership.)

## Verify

```bash
gix --version
gix --help
```

## Uninstall

```bash
rm "$(command -v gix)"                                  # Linux/macOS
Remove-Item "$env:LOCALAPPDATA\Programs\gix\gix.exe"    # Windows
docker rmi bitsgenix/gix:latest                         # Docker
```
