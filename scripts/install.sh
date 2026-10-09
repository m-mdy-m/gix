#!/usr/bin/env sh
# gix installer for Linux and macOS - downloads the prebuilt binary from
# GitHub Releases and puts it on your PATH. One command:
#
#   curl -fsSL https://raw.githubusercontent.com/m-mdy-m/gix/main/scripts/install.sh | sh
#
# Options (environment variables; set them on `sh`, not on curl):
#   GIX_VERSION=v1.2.3   install a specific tag (default: latest release)
#   GIX_PREFIX=~/.local  install prefix (default: /usr/local, or ~/.local
#                        when /usr/local/bin is not writable)
#
#   curl -fsSL .../install.sh | GIX_VERSION=v1.0.1 sh
#
# VERSION and PREFIX are accepted too, for older instructions.
set -eu

REPO="m-mdy-m/gix"
BIN="gix"
VERSION="${GIX_VERSION:-${VERSION:-}}"
PREFIX="${GIX_PREFIX:-${PREFIX:-}}"

say() { printf '%s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

# fetch URL OUTFILE - curl, or wget when curl is missing.
fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$1" -O "$2"
  else
    die "curl or wget is required"
  fi
}

# --- detect platform -------------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in
  linux)  target_os="linux" ;;
  darwin) target_os="darwin" ;;
  msys*|mingw*|cygwin*)
    die "this is the Linux/macOS installer. On Windows use PowerShell:
  irm https://raw.githubusercontent.com/${REPO}/main/scripts/install.ps1 | iex" ;;
  *) die "unsupported OS: $os" ;;
esac

case "$arch" in
  x86_64|amd64) target_arch="x64" ;;
  aarch64|arm64) target_arch="arm64" ;;
  *) die "unsupported architecture: $arch" ;;
esac

target="${target_os}-${target_arch}"

# --- pick version ----------------------------------------------------
# The /releases/latest redirect names the tag without using the GitHub
# API, so it is not subject to the API's rate limit.
if [ -z "$VERSION" ]; then
  if command -v curl >/dev/null 2>&1; then
    final=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest") || final=""
    VERSION="${final##*/}"
  fi
  case "$VERSION" in
    v[0-9]*) ;;
    *) die "could not work out the latest release - set GIX_VERSION=vX.Y.Z" ;;
  esac
fi
version="${VERSION#v}"
base="https://github.com/${REPO}/releases/download/v${version}"

# --- install prefix --------------------------------------------------
if [ -z "$PREFIX" ]; then
  if [ -w /usr/local/bin ] || [ "$(id -u)" = "0" ]; then
    PREFIX="/usr/local"
  else
    PREFIX="$HOME/.local"
  fi
fi
bindir="${PREFIX}/bin"
mkdir -p "$bindir" || die "cannot create ${bindir} - set GIX_PREFIX to a folder you can write to"

# --- download --------------------------------------------------------
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "installing gix v${version} (${target})"

# Newer releases ship the bare binary. Older ones only have a
# .tar.gz, so fall back to that rather than fail.
if fetch "${base}/${BIN}-${target}" "$tmp/${BIN}-${target}" 2>/dev/null; then
  :
else
  fetch "${base}/${BIN}-${version}-${target}.tar.gz" "$tmp/gix.tar.gz" \
    || die "download failed: ${base}/${BIN}-${target} (does v${version} exist?)"
  tar -xzf "$tmp/gix.tar.gz" -C "$tmp"
  [ -f "$tmp/${BIN}-${target}" ] || die "unexpected archive contents"
fi

# --- verify checksum (release publishes <binary>.sha256) -------------
if fetch "${base}/${BIN}-${target}.sha256" "$tmp/sum" 2>/dev/null; then
  want=$(cut -d' ' -f1 "$tmp/sum")
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
  *":${bindir}:"*) ;;
  *) say "note: ${bindir} is not on your PATH. Add it:"
     say "  export PATH=\"${bindir}:\$PATH\"" ;;
esac
say "next: gix flow init"
