#!/bin/bash
set -e

VIDEO="tests/1minute.mp4"
BIN="bin/videodna-arm64"
BNAME=$(basename "$VIDEO" .mp4)
for mode in average ; do
    echo "Running $mode mode..."
    "$BIN" -input "$VIDEO" -output "tests/$BNAME.$mode.png"   -mode "$mode"
    "$BIN" -input "$VIDEO" -output "tests/$BNAME.$mode.v.png" -mode "$mode" -vertical
done

echo "Done. Output files:"
ls -lh tests/$BNAME.*.png
