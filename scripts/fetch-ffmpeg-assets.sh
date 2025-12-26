#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ASSET_DIR="$ROOT_DIR/internal/ffmpeg/assets"
VERSION="6.1"
BASE_URL="https://github.com/ffbinaries/ffbinaries-prebuilt/releases/download"

ASSETS=(
  "ffmpeg-${VERSION}-linux-64.zip"
  "ffprobe-${VERSION}-linux-64.zip"
  "ffmpeg-${VERSION}-linux-arm-64.zip"
  "ffprobe-${VERSION}-linux-arm-64.zip"
  "ffmpeg-${VERSION}-macos-64.zip"
  "ffprobe-${VERSION}-macos-64.zip"
  "ffmpeg-${VERSION}-win-64.zip"
  "ffprobe-${VERSION}-win-64.zip"
)

mkdir -p "$ASSET_DIR"

for asset in "${ASSETS[@]}"; do
  url="${BASE_URL}/v${VERSION}/${asset}"
  echo "Downloading ${asset}..."
  curl -fsSL -o "$ASSET_DIR/$asset" "$url"
  echo "Saved to $ASSET_DIR/$asset"
done
