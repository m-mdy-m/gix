#!/usr/bin/env sh
# gix installer — downloads the prebuilt binary from GitHub Releases.
#
#   curl -sSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | bash
#
# Options (env vars):
#   VERSION=v1.2.3   install a specific tag (default: latest)
#   PREFIX=~/.local  install prefix (default: /usr/local, falls back to ~/.local)
set -eu

REPO="m-mdy-m/gix"
BIN="gix"
VERSION="${VERSION:-}"

say() { printf '%s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

# --- detect platform -------------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in
  linux)  target_os="linux" ;;
  darwin) target_os="darwin" ;;
  msys*|mingw*|cygwin*) die "use install.ps1 on Windows (see README)" ;;
  *) die "unsupported OS: $os" ;;
esac

case "$arch" in
  x86_64|amd64) target_arch="x64" ;;
  aarch64|arm64) target_arch="arm64" ;;
  *) die "unsupported architecture: $arch" ;;
esac

target="${target_os}-${target_arch}"

# --- pick version ----------------------------------------------------
if [ -z "$VERSION" ]; then
  command -v curl >/dev/null 2>&1 || die "curl is required"
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -o '"tag_name": *"[^"]*"' | head -n1 | cut -d'"' -f4) || true
  [ -n "$VERSION" ] || die "could not resolve latest release (rate limit? set VERSION=vX.Y.Z)"
fi
version="${VERSION#v}"

# --- install prefix --------------------------------------------------
if [ -z "${PREFIX:-}" ]; then
  if [ -w /usr/local/bin ] || [ "$(id -u)" = "0" ]; then
    PREFIX="/usr/local"
  else
    PREFIX="$HOME/.local"
  fi
fi
bindir="${PREFIX}/bin"
mkdir -p "$bindir"

# --- download --------------------------------------------------------
url="https://github.com/${REPO}/releases/download/v${version}/gix-${version}-${target}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "installing gix v${version} (${target})"
command -v curl >/dev/null 2>&1 || die "curl is required"
curl -fsSL "$url" -o "$tmp/gix.tar.gz" \
  || die "download failed: $url (does v${version} exist?)"

tar -xzf "$tmp/gix.tar.gz" -C "$tmp"
[ -f "$tmp/${BIN}-${target}" ] || die "unexpected archive contents"

# --- verify checksum (release publishes <binary>.sha256) ------------
sum_url="https://github.com/${REPO}/releases/download/v${version}/${BIN}-${target}.sha256"
if curl -fsSL "$sum_url" -o "$tmp/${BIN}-${target}.sha256" 2>/dev/null; then
  want=$(cut -d' ' -f1 "$tmp/${BIN}-${target}.sha256")
  if command -v sha256sum >/dev/null 2>&1; then
    got=$(sha256sum "$tmp/${BIN}-${target}" | cut -d' ' -f1)
  elif command -v shasum >/dev/null 2>&1; then
    got=$(shasum -a 256 "$tmp/${BIN}-${target}" | cut -d' ' -f1)
  else
    got=""
  fi
  if [ -n "$got" ] && [ "$want" != "$got" ]; then
    die "checksum mismatch for ${BIN}-${target} (corrupted download?)"
  fi
fi

install -m 0755 "$tmp/${BIN}-${target}" "${bindir}/${BIN}"

# --- done ------------------------------------------------------------
say "installed ${bindir}/${BIN} (v${version})"
case ":$PATH:" in
  *":${bindir}:"*) say "ok: ${bindir} is already on PATH" ;;
  *) say "note: add ${bindir} to your PATH:" ;
     say "  export PATH=\"${bindir}:\$PATH\"" ;;
esac
say "next: gix flow init"
