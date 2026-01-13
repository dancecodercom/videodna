# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
go build -o bin/videodna ./cmd/videodna    # Build binary
go run ./cmd/videodna -input video.mp4 -output dna.png  # Run directly
```

## Dependencies

Requires ffmpeg installed on the system:
- macOS: `brew install ffmpeg`
- Linux: `sudo apt-get install ffmpeg`

No Go dependencies (pure standard library).

## Usage

```bash
./bin/videodna -input <video> -output <image.png> -mode <average|min|max|common> [-silent] [-timeout 60]
```

- `-mode average` (default): averages RGB per row
- `-mode common`: finds most frequent color per row (slowest)
- `-mode min/max`: darkest/brightest color per row
- `-silent`: suppress stdout output
- `-timeout`: max seconds before abort (default 60)

## Project Structure

```
cmd/videodna/       # Main entrypoint
internal/dna/       # DNA generation and color extraction
internal/video/     # Video probing via ffprobe
bin/                # Compiled binaries
```

## Architecture

1. Uses ffprobe to get video dimensions and frame count
2. Pipes raw RGB frames from ffmpeg
3. For each frame, extracts one representative color per horizontal row
4. Outputs PNG where width=frames, height=video height, each column=one frame's color profile
