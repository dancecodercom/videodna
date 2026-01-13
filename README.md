# VideoDNA

Generate DNA-style color fingerprint images from video files.

![](assets/1minute.average.png)

## How it works

For each frame, extract one representative color per row (or column), creating a visual signature of the entire video.

**Default (horizontal):**
- Image width = number of frames
- Image height = video height
- Each vertical line = 1 frame

**Vertical mode (`-vertical`):**
- Image width = video width
- Image height = number of frames
- Each horizontal line = 1 frame

## Installation

Requires ffmpeg:

```bash
# macOS
brew install ffmpeg

# Linux
sudo apt-get install ffmpeg
```

Build:
```bash
go build -o bin/videodna ./cmd/videodna
```

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
```

## Modes

| Mode | Description | Speed |
|------|-------------|-------|
| `average` | Average RGB per row/column | Fastest |
| `min` | Darkest color per row/column | Fast |
| `max` | Brightest color per row/column | Fast |
| `common` | Most frequent color per row/column | Slowest |

## Examples

```bash
# Basic usage
./bin/videodna -input video.mp4 -output dna.png

# Use max (brightest) mode
./bin/videodna -input video.mp4 -output dna.png -mode max

# Vertical output, resized to match input video dimensions
./bin/videodna -input video.mp4 -output dna.png -vertical -resize input

# Resize to specific dimensions
./bin/videodna -input video.mp4 -output dna.png -resize 1920x1080
```

## Project Structure

```
cmd/videodna/       Main CLI entrypoint
internal/dna/       DNA generation and color extraction
internal/video/     Video probing via ffprobe
bin/                Compiled binaries
```
