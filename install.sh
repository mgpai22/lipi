#!/usr/bin/env bash
set -euo pipefail

# install.sh — download & install the right lipi binary for this machine.
# Usage: ./install.sh [--local] [--tag vX.Y.Z]
#   env: BIN_DIR=~/.local/bin (default)
#
# Downloads from: https://github.com/mgpai22/lipi/releases

BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
LOCAL_INSTALL=false
TAG_OVERRIDE=""
REPO="mgpai22/lipi"
GITHUB_API="https://api.github.com/repos/${REPO}/releases"
GITHUB_DOWNLOAD="https://github.com/${REPO}/releases/download"

die() { echo "error: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "missing dependency: $1"; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --local)
      LOCAL_INSTALL=true
      shift
      ;;
    --tag|-t)
      shift
      [[ $# -gt 0 ]] || die "--tag requires a value like v1.2.3"
      TAG_OVERRIDE="$1"
      shift
      ;;
    -h|--help)
      cat <<EOF
Usage: $0 [--local] [--tag vX.Y.Z]

Downloads the correct archive for this machine and installs to:
  BIN_DIR=${BIN_DIR} (or current directory if --local is used)

Options:
  --local        Download to current directory instead of installing to PATH
  --tag, -t      Install a specific tag instead of the latest (e.g. v0.1.0)

Env:
  BIN_DIR        Installation directory (default: ${BIN_DIR})
EOF
      exit 0
      ;;
    *)
      die "unknown arg: $1 (use --help)"
      ;;
  esac
done

need curl
need tar
need grep

if [[ "$LOCAL_INSTALL" == "true" ]]; then
  BIN_DIR="."
else
  mkdir -p "$BIN_DIR"
fi

uname_s=$(uname -s)
uname_m=$(uname -m)

# Determine OS target
case "$uname_s" in
  Linux)   os="linux" ;;
  Darwin)  os="darwin" ;;
  MINGW*|MSYS*|CYGWIN*) os="windows" ;;
  *)       die "unsupported OS: $uname_s" ;;
esac

# Determine arch (normalize common aliases)
case "$uname_m" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) die "unsupported CPU arch: $uname_m" ;;
esac

# Validate supported platform combinations
case "${os}-${arch}" in
  linux-amd64|linux-arm64|darwin-amd64|windows-amd64) ;;
  darwin-arm64)
    echo "info: darwin-arm64 not available, falling back to darwin-amd64 (Rosetta)"
    arch="amd64"
    ;;
  *) die "unsupported platform: ${os}-${arch}" ;;
esac

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

# Determine version tag
if [[ -n "$TAG_OVERRIDE" ]]; then
  tag="$TAG_OVERRIDE"
  echo "info: using tag override: $tag"
else
  echo "info: fetching latest release..."
  latest_url="${GITHUB_API}/latest"
  tag=$(curl -fsSL "$latest_url" | grep -o '"tag_name"[[:space:]]*:[[:space:]]*"[^"]*"' | head -n1 | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
  [[ -n "$tag" ]] || die "could not determine latest version"
  echo "info: latest version: $tag"
fi

# Construct asset name
if [[ "$os" == "windows" ]]; then
  asset_file="lipi-${os}-${arch}.zip"
else
  asset_file="lipi-${os}-${arch}.tar.gz"
fi

url="${GITHUB_DOWNLOAD}/${tag}/${asset_file}"
echo "info: downloading ${url}"

archive="$tmpdir/${asset_file}"
curl -fL --retry 3 --retry-delay 2 -o "$archive" "$url" \
  || die "failed to download: $url"

echo "info: extracting ${asset_file}"

if [[ "$os" == "windows" ]]; then
  need unzip
  unzip -q "$archive" -d "$tmpdir"
  bin_name="lipi.exe"
else
  tar -xzf "$archive" -C "$tmpdir"
  bin_name="lipi"
fi

# Find the extracted binary
bin_src="$(find "$tmpdir" -maxdepth 2 -type f -name "${bin_name}" | head -n1 || true)"
[[ -n "$bin_src" ]] || die "could not locate ${bin_name} inside archive"

install_path="${BIN_DIR}/${bin_name}"
mv "$bin_src" "$install_path"
chmod +x "$install_path"

echo "success: installed lipi -> ${install_path}"

# Verify installation
if "${install_path}" version >/dev/null 2>&1; then
  echo "info: $("${install_path}" version | head -n1)"
fi

# PATH hint (skip for local install)
if [[ "$LOCAL_INSTALL" != "true" ]]; then
  case ":$PATH:" in
    *:"$BIN_DIR":*) ;;
    *)
      echo ""
      echo "note: ${BIN_DIR} is not in PATH. Add this to your shell rc:"
      echo "      export PATH=\"\$PATH:$BIN_DIR\""
      ;;
  esac
fi
