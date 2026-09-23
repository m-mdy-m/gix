# Getting started

## 1. Install

**Quick install (Linux/macOS):**

```bash
curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | sh
```

**Windows (PowerShell):**

```powershell
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.ps1" -OutFile install.ps1; .\install.ps1
```

**From source** (needs Go 1.25+):

```bash
git clone https://github.com/m-mdy-m/gix.git
cd gix
make install
```

**Docker:**

```bash
docker pull bitsgenix/gix:latest
```

Prebuilt binaries: [Releases](https://github.com/m-mdy-m/gix/releases)
(linux/mac/windows, x64 + arm64).

## 2. Initialize (once per repo)

```bash
cd your-project
gix flow init
```

This writes `.gix/config`, commits it, and creates `develop` if
missing. Nothing else is touched.

## 3. Work a feature

```bash
gix feature start api-auth    # creates feature/api-auth from develop
# ...code, commit with plain git...
gix feature finish api-auth   # merges into develop, deletes the branch
```

`start`/`finish` require a **clean working tree** — commit or stash
first.

## 4. Ship a release

```bash
gix release start v0.2.0
# ...bump version, update CHANGELOG...
gix release finish v0.2.0 --tag v0.2.0
# → merges into main + develop, tags v0.2.0, deletes the branch
```

## Where am I?

```bash
gix status   # current branch, role, ahead/behind, dirty state
gix list     # active branches   (gix list --tree for a tree view)
```

Next: [Examples](examples.md) · [Commands](api/README.md)
