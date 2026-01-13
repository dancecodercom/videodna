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
	"strconv"
	"strings"
	"time"

	"github.com/pforret/videodna/internal/video"
)

// Generate creates a video DNA image from the input video.
func Generate(inputPath, outputPath, mode string, vertical bool, resize string, silent bool, timeout int) error {
	width, height, frameCount, err := video.GetInfo(inputPath)
	if err != nil {
		return err
	}

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
