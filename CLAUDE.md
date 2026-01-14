# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
# Video DNA
go build -o bin/videodna ./cmd/videodna
go run ./cmd/videodna -input video.mp4 -output dna.png

# Audio DNA
go build -o bin/audiodna ./cmd/audiodna
go run ./cmd/audiodna -input song.mp3 -output audiodna.png
```

## Dependencies

**Required (both tools):**
- ffmpeg/ffprobe
  - macOS: `brew install ffmpeg`
  - Linux: `sudo apt-get install ffmpeg`

**Audio DNA stem separation (optional):**
- Demucs (default): `pip install demucs`
- Spleeter: `pip install spleeter`

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

## Audio DNA Usage

```bash
audiodna -input <audio> [options]

Options:
  -input string      Input audio file (required)
  -output string     Output PNG file (default "audiodna.png")
  -width int         Output width in pixels (default 1920)
  -stem-height int   Height per stem in pixels (default 50)
  -stems int         Number of stems: 2, 4, or 6 (default 4)
  -separator string  Stem separator: demucs or spleeter (default "demucs")
  -device string     Device: cpu or cuda (default "cpu")
  -no-stems          Skip stem separation
  -no-labels         Hide stem labels
  -no-normalize      Don't normalize volume levels
  -timeout int       Timeout in seconds (default 600)
  -silent            Suppress stdout output

Stem Types:
  2 stems: vocals + accompaniment
  4 stems: vocals + drums + bass + other
  6 stems: vocals + drums + bass + other + piano + guitar (demucs only)

Examples:
  audiodna -input song.mp3 -output dna.png
  audiodna -input song.mp3 -no-stems                    # Waveform only
  audiodna -input song.mp3 -stems 6 -device cuda        # GPU acceleration
  audiodna -input song.mp3 -separator spleeter          # Use Spleeter

Docker:
  docker build -f Dockerfile.audiodna -t audiodna .
  docker run -v $(pwd):/data audiodna -input /data/song.mp3 -output /data/dna.png
```

## Project Structure

```
cmd/videodna/           # Video DNA CLI entrypoint
cmd/audiodna/           # Audio DNA CLI entrypoint
internal/dna/           # Video DNA generation and color extraction
internal/video/         # Video probing via ffprobe
internal/audio/         # Audio probing, stem separation, waveform extraction
internal/audiodna/      # Audio DNA generation
functions/audiodna/     # Cloud function for audio DNA
bin/                    # Compiled binaries
tests/                  # Test files and output images
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

## Audio DNA Architecture

1. Uses ffprobe to get audio duration, sample rate, channels
2. If stem separation enabled:
   - Runs Demucs/Spleeter to separate audio into stems (vocals, drums, bass, etc.)
   - Each stem is processed separately
3. Extracts waveform data via ffmpeg (raw PCM)
4. Computes RMS volume per time segment
5. Generates visualization:
   - X-axis = time
   - Y-axis = volume per stem (stacked vertically)
   - Each stem has distinct color

**Output dimensions:**
- Width = configurable (default 1920)
- Height = number of stems × stem height (default 50px each)
