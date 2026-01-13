# VideoDNA

Generate a visual DNA representation of videos by extracting the most common color from each horizontal row of every frame.

## How it works

- Image height = video height
- Image width = number of frames
- Each vertical line = 1 frame
- Each pixel = most common color from that horizontal row in the frame

## Installation

Requires OpenCV. Install via:

**macOS:**
```bash
brew install opencv
```

**Linux:**
```bash
sudo apt-get install libopencv-dev
```

Then install Go dependencies:
```bash
go mod download
```

## Usage

```bash
go run main.go -input video.mp4 -output dna.png
```

Or build and run:
```bash
go build -o videodna
./videodna -input video.mp4 -output dna.png
```

## Example

```bash
./videodna -input myMovie.mp4 -output myMovie_dna.png
```
