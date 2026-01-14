#!/bin/bash
set -e

VIDEO="${1:-tests/1minute.mp4}"
BNAME=$(basename "$VIDEO" .mp4)
OUTPUT_DIR=$(dirname "$VIDEO")

VIDEO_DNA="$OUTPUT_DIR/${BNAME}_video.png"
AUDIO_DNA="$OUTPUT_DIR/${BNAME}_audio.png"
COMBINED="$OUTPUT_DIR/${BNAME}_combined.png"

echo "Processing: $VIDEO"

# Run videodna
echo "Generating video DNA..."
./bin/videodna -input "$VIDEO" -output "$VIDEO_DNA" -resize 1920x800

# Run audiodna (via Docker)
echo "Generating audio DNA..."
docker run -v "$(pwd)/$OUTPUT_DIR:/data" audiodna \
    -input "/data/$(basename "$VIDEO")" \
    -output "/data/${BNAME}_audio.png" \
    -resize "1920x200"

# Stack images vertically (video on top, audio below)
echo "Combining images..."
ffmpeg -y -i "$VIDEO_DNA" -i "$AUDIO_DNA" -filter_complex vstack=inputs=2 "$COMBINED" 2>/dev/null

echo "Done. Output files:"
ls -lh "$VIDEO_DNA" "$AUDIO_DNA" "$COMBINED"
