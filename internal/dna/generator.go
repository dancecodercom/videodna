package dna

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pforret/videodna/internal/video"
)

// LegendConfig configures the top legend bar.
type LegendConfig struct {
	Enabled bool   // Show legend
	Height  int    // Height in pixels (default 24)
	Name    string // Display name (default: basename of input file)
}

// DefaultLegendConfig returns default legend configuration.
func DefaultLegendConfig() LegendConfig {
	return LegendConfig{
		Enabled: true,
		Height:  24,
		Name:    "",
	}
}

// Generate creates a video DNA image from the input video.
func Generate(inputPath, outputPath, mode string, vertical bool, resize string, silent bool, timeout int) error {
	return GenerateWithLegend(inputPath, outputPath, mode, vertical, resize, silent, timeout, LegendConfig{})
}

// GenerateWithLegend creates a video DNA image with optional legend.
func GenerateWithLegend(inputPath, outputPath, mode string, vertical bool, resize string, silent bool, timeout int, legend LegendConfig) error {
	info, err := video.GetFullInfo(inputPath)
	if err != nil {
		return err
	}

	width, height, frameCount := info.Width, info.Height, info.FrameCount

	if frameCount == 0 || height == 0 {
		return fmt.Errorf("invalid video properties")
	}

	if !silent {
		fmt.Printf("Processing video: %d frames, %dx%d pixels\n", frameCount, width, height)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputPath,
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-v", "error",
		"pipe:1")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	maxFrames := frameCount + frameCount/10 + 10
	var dnaImage *image.RGBA
	if vertical {
		dnaImage = image.NewRGBA(image.Rect(0, 0, width, maxFrames))
	} else {
		dnaImage = image.NewRGBA(image.Rect(0, 0, maxFrames, height))
	}

	frameSize := width * height * 3
	reader := bufio.NewReaderSize(stdout, frameSize)
	frameBuf := make([]byte, frameSize)
	startTime := time.Now()

	frameIdx := 0
	for {
		_, err := io.ReadFull(reader, frameBuf)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return fmt.Errorf("failed to read frame: %w", err)
		}

		if vertical {
			for x := 0; x < width; x++ {
				var c color.Color
				switch mode {
				case "average":
					c = AverageColorCol(frameBuf, x, width, height)
				case "min":
					c = MinColorCol(frameBuf, x, width, height)
				case "max":
					c = MaxColorCol(frameBuf, x, width, height)
				default:
					c = MostCommonColorCol(frameBuf, x, width, height)
				}
				dnaImage.Set(x, frameIdx, c)
			}
		} else {
			for y := 0; y < height; y++ {
				rowStart := y * width * 3
				row := frameBuf[rowStart : rowStart+width*3]

				var c color.Color
				switch mode {
				case "average":
					c = AverageColor(row, width)
				case "min":
					c = MinColor(row, width)
				case "max":
					c = MaxColor(row, width)
				default:
					c = MostCommonColor(row, width)
				}
				dnaImage.Set(frameIdx, y, c)
			}
		}

		frameIdx++

		if !silent && frameIdx%100 == 0 {
			fps := float64(frameIdx) / time.Since(startTime).Seconds()
			pct := float64(frameIdx) * 100 / float64(frameCount)
			fmt.Printf("Processed %d/%d frames (%.1f fps, %.0f%% done)\n", frameIdx, frameCount, fps, pct)
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("timeout after %d seconds", timeout)
		}
	}

	elapsed := time.Since(startTime).Seconds()
	if !silent && elapsed > 0 {
		fps := float64(frameIdx) / elapsed
		totalPixels := float64(frameIdx) * float64(width) * float64(height)
		pps := totalPixels / elapsed / 1e6
		fmt.Printf("Done: %d frames in %.2fs (%.1f fps, %.1f Mpx/s)\n", frameIdx, elapsed, fps, pps)
	}

	var finalImage image.Image
	if vertical {
		finalImage = dnaImage.SubImage(image.Rect(0, 0, width, frameIdx))
	} else {
		finalImage = dnaImage.SubImage(image.Rect(0, 0, frameIdx, height))
	}

	// Handle resize
	if resize != "" {
		var targetW, targetH int
		if resize == "input" {
			targetW, targetH = width, height
		} else {
			parts := strings.Split(strings.ToLower(resize), "x")
			if len(parts) != 2 {
				return fmt.Errorf("invalid resize format, use WxH or 'input'")
			}
			targetW, err = strconv.Atoi(parts[0])
			if err != nil {
				return fmt.Errorf("invalid resize width: %w", err)
			}
			targetH, err = strconv.Atoi(parts[1])
			if err != nil {
				return fmt.Errorf("invalid resize height: %w", err)
			}
		}
		finalImage = resizeImage(finalImage, targetW, targetH)
	}

	// Add light gray border lines at top and bottom to make letterboxing visible
	finalImage = addBorderLines(finalImage)

	// Add legend if enabled
	if legend.Enabled {
		legendHeight := legend.Height
		if legendHeight == 0 {
			legendHeight = 24
		}
		name := legend.Name
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		}
		finalImage = addLegend(finalImage, legendHeight, name, info)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, finalImage); err != nil {
		return fmt.Errorf("failed to encode PNG: %w", err)
	}

	return nil
}

// resizeImage scales an image to the target dimensions using bilinear interpolation.
func resizeImage(src image.Image, targetW, targetH int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			// Map destination pixel to source coordinates
			srcX := float64(x) * float64(srcW) / float64(targetW)
			srcY := float64(y) * float64(srcH) / float64(targetH)

			// Bilinear interpolation
			x0 := int(srcX)
			y0 := int(srcY)
			x1 := x0 + 1
			y1 := y0 + 1

			if x1 >= srcW {
				x1 = srcW - 1
			}
			if y1 >= srcH {
				y1 = srcH - 1
			}

			xFrac := srcX - float64(x0)
			yFrac := srcY - float64(y0)

			r00, g00, b00, _ := src.At(bounds.Min.X+x0, bounds.Min.Y+y0).RGBA()
			r10, g10, b10, _ := src.At(bounds.Min.X+x1, bounds.Min.Y+y0).RGBA()
			r01, g01, b01, _ := src.At(bounds.Min.X+x0, bounds.Min.Y+y1).RGBA()
			r11, g11, b11, _ := src.At(bounds.Min.X+x1, bounds.Min.Y+y1).RGBA()

			r := bilinear(r00, r10, r01, r11, xFrac, yFrac)
			g := bilinear(g00, g10, g01, g11, xFrac, yFrac)
			b := bilinear(b00, b10, b01, b11, xFrac, yFrac)

			dst.Set(x, y, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255})
		}
	}

	return dst
}

func bilinear(v00, v10, v01, v11 uint32, xFrac, yFrac float64) uint32 {
	v0 := float64(v00)*(1-xFrac) + float64(v10)*xFrac
	v1 := float64(v01)*(1-xFrac) + float64(v11)*xFrac
	return uint32(v0*(1-yFrac) + v1*yFrac)
}

// addBorderLines adds light gray lines at top and bottom to make letterboxing visible
func addBorderLines(src image.Image) image.Image {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	// Copy original image
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, src.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}

	// Draw light gray border lines
	borderColor := color.RGBA{R: 80, G: 80, B: 80, A: 255}
	for x := 0; x < w; x++ {
		dst.Set(x, 0, borderColor)   // Top line
		dst.Set(x, h-1, borderColor) // Bottom line
	}

	return dst
}

// addLegend adds a legend bar at the top of the image
func addLegend(src image.Image, legendHeight int, name string, info *video.Info) *image.RGBA {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, w, h+legendHeight))

	// Fill legend background
	labelBg := color.RGBA{R: 25, G: 25, B: 30, A: 255}
	for y := 0; y < legendHeight; y++ {
		for x := 0; x < w; x++ {
			dst.SetRGBA(x, y, labelBg)
		}
	}

	// Copy original image below legend
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := src.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			dst.SetRGBA(x, y+legendHeight, color.RGBA{
				R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8),
			})
		}
	}

	// Build legend text
	textColor := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	yText := (legendHeight - 7) / 2 // Center 7px tall font

	// Format: name | duration | fps | frames | codec | resolution
	var parts []string
	parts = append(parts, name)

	if info.Duration > 0 {
		mins := int(info.Duration) / 60
		secs := int(info.Duration) % 60
		if mins > 0 {
			parts = append(parts, fmt.Sprintf("%dm%02ds", mins, secs))
		} else {
			parts = append(parts, fmt.Sprintf("%.1fs", info.Duration))
		}
	}

	if info.FPS > 0 {
		parts = append(parts, fmt.Sprintf("%.1ffps", info.FPS))
	}

	if info.FrameCount > 0 {
		parts = append(parts, fmt.Sprintf("%df", info.FrameCount))
	}

	if info.Codec != "" {
		parts = append(parts, info.Codec)
	}

	if info.Width > 0 && info.Height > 0 {
		parts = append(parts, fmt.Sprintf("%dx%d", info.Width, info.Height))
	}

	legendText := strings.Join(parts, " | ")
	drawText(dst, legendText, 8, yText, textColor)

	return dst
}

// drawText draws text using a simple bitmap font
func drawText(img *image.RGBA, text string, x, y int, c color.RGBA) {
	for _, ch := range strings.ToLower(text) {
		pattern, ok := bitmapFont[byte(ch)]
		if !ok {
			x += 4 // space for unknown chars
			continue
		}

		for dy, row := range pattern {
			for dx, pixel := range row {
				if pixel == '#' {
					img.SetRGBA(x+dx, y+dy, c)
				}
			}
		}
		x += len(pattern[0]) + 1 // char width + spacing
	}
}

// bitmapFont is a simple 5x7 bitmap font
var bitmapFont = map[byte][]string{
	'a': {"..#..", ".#.#.", "#...#", "#####", "#...#", "#...#", "#...#"},
	'b': {"####.", "#...#", "#...#", "####.", "#...#", "#...#", "####."},
	'c': {".###.", "#...#", "#....", "#....", "#....", "#...#", ".###."},
	'd': {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'e': {"#####", "#....", "#....", "####.", "#....", "#....", "#####"},
	'f': {"#####", "#....", "#....", "####.", "#....", "#....", "#...."},
	'g': {".###.", "#....", "#....", "#.###", "#...#", "#...#", ".###."},
	'h': {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'i': {".###.", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'j': {"..###", "...#.", "...#.", "...#.", "#..#.", "#..#.", ".##.."},
	'k': {"#...#", "#..#.", "#.#..", "##...", "#.#..", "#..#.", "#...#"},
	'l': {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'm': {"#...#", "##.##", "#.#.#", "#...#", "#...#", "#...#", "#...#"},
	'n': {"#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#", "#...#"},
	'o': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'p': {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'q': {".###.", "#...#", "#...#", "#...#", "#.#.#", "#..#.", ".##.#"},
	'r': {"####.", "#...#", "#...#", "####.", "#.#..", "#..#.", "#...#"},
	's': {".####", "#....", "#....", ".###.", "....#", "....#", "####."},
	't': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'u': {"#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'v': {"#...#", "#...#", "#...#", "#...#", ".#.#.", ".#.#.", "..#.."},
	'w': {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "##.##", "#...#"},
	'x': {"#...#", ".#.#.", "..#..", "..#..", "..#..", ".#.#.", "#...#"},
	'y': {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#..", "..#.."},
	'z': {"#####", "....#", "...#.", "..#..", ".#...", "#....", "#####"},
	'0': {".###.", "#...#", "#..##", "#.#.#", "##..#", "#...#", ".###."},
	'1': {"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'2': {".###.", "#...#", "....#", "..##.", ".#...", "#....", "#####"},
	'3': {".###.", "#...#", "....#", "..##.", "....#", "#...#", ".###."},
	'4': {"#...#", "#...#", "#...#", "#####", "....#", "....#", "....#"},
	'5': {"#####", "#....", "####.", "....#", "....#", "#...#", ".###."},
	'6': {".###.", "#....", "####.", "#...#", "#...#", "#...#", ".###."},
	'7': {"#####", "....#", "...#.", "..#..", ".#...", "#....", "#...."},
	'8': {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", ".###."},
	'9': {".###.", "#...#", "#...#", ".####", "....#", "....#", ".###."},
	'.': {".....", ".....", ".....", ".....", ".....", "..#..", "..#.."},
	'|': {"..#..", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'-': {".....", ".....", ".....", "#####", ".....", ".....", "....."},
	'_': {".....", ".....", ".....", ".....", ".....", ".....", "#####"},
	' ': {".....", ".....", ".....", ".....", ".....", ".....", "....."},
	'(': {"...#.", "..#..", ".#...", ".#...", ".#...", "..#..", "...#."},
	')': {".#...", "..#..", "...#.", "...#.", "...#.", "..#..", ".#..."},
}
