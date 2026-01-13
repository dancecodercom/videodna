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
videodna -input <video> [options]

Options:
  -input string    Input video file (required)
  -output string   Output PNG file (default "output.png")
  -mode string     Color mode: average, min, max, common (default "average")
  -vertical        Vertical output (width=video width, height=frames)
  -resize string   Resize output: 'WxH' or 'input' for video dimensions
  -silent          Suppress stdout output
  -timeout int     Timeout in seconds (default 60)

Modes:
  average  Average RGB per row/column (default, fastest)
  min      Darkest color per row/column
  max      Brightest color per row/column
  common   Most frequent color per row/column (slowest)

Examples:
  videodna -input video.mp4 -output dna.png
  videodna -input video.mp4 -output dna.png -mode max
  videodna -input video.mp4 -output dna.png -vertical -resize input
  videodna -input video.mp4 -output dna.png -resize 1920x1080
```

## Project Structure

```
cmd/videodna/       # Main CLI entrypoint
internal/dna/       # DNA generation and color extraction
internal/video/     # Video probing via ffprobe
bin/                # Compiled binaries
tests/              # Test videos and output images
```

## Architecture

1. Uses ffprobe to get video dimensions and estimated frame count
2. Pipes raw RGB frames from ffmpeg to Go process
3. For each frame:
   - Default: extracts one color per horizontal row → output column
   - Vertical: extracts one color per vertical column → output row
4. Applies optional resize using bilinear interpolation
5. Outputs PNG image

**Output dimensions:**
- Default: width=frames, height=video_height
- Vertical: width=video_width, height=frames
