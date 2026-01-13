#!/usr/bin/env bash
# Download medium resolution mp4 from YouTube

set -euo pipefail

if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <youtube-url>"
    exit 1
fi

URL="$1"
OUTPUT_DIR="$(dirname "$0")/tests"

mkdir -p "$OUTPUT_DIR"

yt-dlp \
    -f "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/best[height<=720][ext=mp4]/best[height<=720]" \
    --merge-output-format mp4 \
    -o "$OUTPUT_DIR/%(title)s.%(ext)s" \
    "$URL"
