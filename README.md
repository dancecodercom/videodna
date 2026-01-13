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
## Output

```
Processing video: 1439 frames, 1280x720 pixels
Processed 100/1439 frames (472.0 fps, 7% done)
Processed 200/1439 frames (548.0 fps, 14% done)
Processed 300/1439 frames (569.8 fps, 21% done)
Processed 400/1439 frames (590.1 fps, 28% done)
Processed 500/1439 frames (611.8 fps, 35% done)
Processed 600/1439 frames (623.5 fps, 42% done)
Processed 700/1439 frames (633.8 fps, 49% done)
Processed 800/1439 frames (639.5 fps, 56% done)
Processed 900/1439 frames (644.0 fps, 63% done)
Processed 1000/1439 frames (649.6 fps, 69% done)
Processed 1100/1439 frames (648.7 fps, 76% done)
Processed 1200/1439 frames (648.4 fps, 83% done)
Processed 1300/1439 frames (646.8 fps, 90% done)
Processed 1400/1439 frames (650.5 fps, 97% done)
Done: 1439 frames in 2.21s (650.7 fps, 599.7 Mpx/s)
```
## Project Structure

```
cmd/videodna/       Main CLI entrypoint
internal/dna/       DNA generation and color extraction
internal/video/     Video probing via ffprobe
bin/                Compiled binaries
```
