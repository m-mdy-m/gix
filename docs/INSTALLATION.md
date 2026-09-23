# Installation

Four ways. Pick one.

## 1. Quick install (recommended)

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | sh
```

Installs to `/usr/local/bin`, or `~/.local/bin` if not writable.

```bash
VERSION=v1.2.3 sh install.sh    # specific version
PREFIX=~/.local sh install.sh   # custom prefix
```

**Windows (PowerShell):**

```powershell
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1" -OutFile install.ps1
.\install.ps1
```

Installs to `%LOCALAPPDATA%\Programs\gix` and adds it to your user PATH.

## 2. Prebuilt binary

Download from [Releases](https://github.com/m-mdy-m/gix/releases):

| Asset | Platform |
|-------|----------|
| `gix-<ver>-linux-x64.tar.gz` / `-arm64` | Linux |
| `gix-<ver>-darwin-x64.tar.gz` / `-arm64` | macOS |
| `gix-<ver>-windows-x64.zip` | Windows |

Every asset ships with a `.sha256` checksum.

```bash
tar -xzf gix-1.0.0-linux-x64.tar.gz
sudo mv gix-linux-x64 /usr/local/bin/gix
gix --version
```

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
