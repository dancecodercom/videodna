// Package audiodna generates DNA visualizations from audio files.
package audiodna

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pforret/videodna/internal/audio"
)

// Config configures DNA generation.
type Config struct {
	Width        int                 // Output width in pixels (0 = auto from duration)
	Height       int                 // Output height in pixels (auto-calculated if 0)
	StemConfig   audio.StemConfig    // Stem separation config
	SkipStems    bool                // If true, use original audio only
	Normalize    bool                // Normalize volume levels
	ColorScheme  ColorScheme         // Color scheme for visualization
	StemHeight   int                 // Height per stem in pixels (default: 50)
	ShowLabels   bool                // Show stem labels at top
	LabelHeight  int                 // Height of label area at top (default: 20)
	Timeout      int                 // Timeout in seconds
	Silent       bool                // Suppress progress output
	ResizeWidth  int                 // Final resize width (0 = no resize)
	ResizeHeight int                 // Final resize height (0 = no resize)
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		Width:        0,    // Auto-calculate from duration
		Height:       0,    // Auto-calculate from stems
		StemConfig:   audio.DefaultStemConfig(),
		SkipStems:    false,
		Normalize:    true,
		ColorScheme:  SchemeDefault,
		StemHeight:   50,
		ShowLabels:   true,
		LabelHeight:  20,
		Timeout:      600, // 10 minutes default for stem separation
		Silent:       false,
		ResizeWidth:  0, // No resize by default
		ResizeHeight: 0,
	}
}

const (
	defaultFPS      = 24  // Assumed FPS for audio files
	minOutputWidth  = 720 // Minimum output width
)

// ColorScheme defines how stems are colored.
type ColorScheme string

const (
	SchemeDefault    ColorScheme = "default"    // Distinct colors per stem
	SchemeMonochrome ColorScheme = "monochrome" // Grayscale
	SchemeHeatmap    ColorScheme = "heatmap"    // Volume as heat colors
	SchemeSpectrum   ColorScheme = "spectrum"   // Rainbow spectrum
)

// StemColors maps stem types to colors.
var StemColors = map[string]color.RGBA{
	"vocals": {R: 255, G: 100, B: 100, A: 255}, // Red/Pink
	"drums":  {R: 100, G: 200, B: 255, A: 255}, // Light Blue
	"bass":   {R: 100, G: 255, B: 150, A: 255}, // Green
	"other":  {R: 200, G: 150, B: 255, A: 255}, // Purple
	"piano":  {R: 255, G: 220, B: 100, A: 255}, // Yellow
	"guitar": {R: 255, G: 180, B: 100, A: 255}, // Orange
	"mixed":  {R: 200, G: 200, B: 200, A: 255}, // Gray
}

// StemData contains processed data for a single stem.
type StemData struct {
	Label    string
	Segments []audio.VolumeSegment
	Color    color.RGBA
}

// Result contains the generated DNA image and metadata.
type Result struct {
	Image    *image.RGBA
	Stems    []StemData
	Duration float64
}

// Generate creates a DNA visualization from an audio file.
func Generate(ctx context.Context, inputPath, outputPath string, config Config) (*Result, error) {
	// Get audio info
	info, err := audio.GetInfo(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio info: %w", err)
	}

	// Calculate width from duration if not specified
	// Width = max(720, duration * 24fps)
	if config.Width == 0 {
		frames := int(info.Duration * defaultFPS)
		config.Width = frames
		if config.Width < minOutputWidth {
			config.Width = minOutputWidth
		}
	}

	if !config.Silent {
		fmt.Printf("Input: %s (%.1fs, %dHz, %dch, %dpx)\n",
			inputPath, info.Duration, info.SampleRate, info.Channels, config.Width)
	}

	var stemFiles *audio.StemFiles
	var stemLabels []string
	var stemPaths []string

	if !config.SkipStems {
		// Check if separator is available
		if err := audio.CheckSeparatorAvailable(config.StemConfig.Separator); err != nil {
			if !config.Silent {
				fmt.Printf("Warning: %v, using original audio\n", err)
			}
			config.SkipStems = true
		}
	}

	if !config.SkipStems {
		if !config.Silent {
			fmt.Printf("Separating into %d stems (%s)...\n",
				config.StemConfig.NumStems, config.StemConfig.Separator)
		}

		stemFiles, err = audio.SeparateStems(ctx, inputPath, config.StemConfig)
		if err != nil {
			return nil, fmt.Errorf("stem separation failed: %w", err)
		}

		stemPaths = stemFiles.GetStemPaths()
		stemLabels = stemFiles.GetStemLabels()
	}

	// If no stems, use original audio
	if len(stemPaths) == 0 {
		stemPaths = []string{inputPath}
		stemLabels = []string{"mixed"}
	}

	if !config.Silent {
		fmt.Printf("Extracting waveforms: %s\n", strings.Join(stemLabels, ", "))
	}

	// Process each stem in parallel
	waveformConfig := audio.DefaultWaveformConfig()
	stemDataList := make([]StemData, len(stemPaths))
	var wg sync.WaitGroup
	var processErr error
	var errMu sync.Mutex

	for i, stemPath := range stemPaths {
		wg.Add(1)
		go func(idx int, path, label string) {
			defer wg.Done()

			waveform, err := audio.ExtractWaveform(ctx, path, waveformConfig)
			if err != nil {
				errMu.Lock()
				if processErr == nil {
					processErr = fmt.Errorf("failed to extract waveform for %s: %w", label, err)
				}
				errMu.Unlock()
				return
			}

			segments := audio.ExtractVolume(waveform, config.Width)
			if config.Normalize {
				audio.NormalizeVolume(segments)
			}

			stemColor := StemColors[label]
			if stemColor.A == 0 {
				stemColor = StemColors["mixed"]
			}

			stemDataList[idx] = StemData{
				Label:    label,
				Segments: segments,
				Color:    stemColor,
			}
		}(i, stemPath, stemLabels[i])
	}

	wg.Wait()

	if processErr != nil {
		return nil, processErr
	}

	// Calculate waveform dimensions (without labels)
	waveformHeight := config.Height
	if waveformHeight == 0 {
		waveformHeight = len(stemDataList) * config.StemHeight
	}
	waveformWidth := config.Width

	// Create waveform image (without labels)
	waveformImg := image.NewRGBA(image.Rect(0, 0, waveformWidth, waveformHeight))

	// Fill background
	bgColor := color.RGBA{R: 20, G: 20, B: 25, A: 255}
	for y := 0; y < waveformHeight; y++ {
		for x := 0; x < waveformWidth; x++ {
			waveformImg.SetRGBA(x, y, bgColor)
		}
	}

	// Draw each stem
	stemPixelHeight := waveformHeight / len(stemDataList)

	for i, stemData := range stemDataList {
		yStart := i * stemPixelHeight
		yMid := yStart + stemPixelHeight/2

		// Draw waveform
		for x, seg := range stemData.Segments {
			if x >= waveformWidth {
				break
			}

			// Calculate bar height based on RMS
			barHeight := int(seg.RMS * float64(stemPixelHeight) * 0.8)
			if barHeight < 1 {
				barHeight = 1
			}

			// Draw symmetric waveform
			halfHeight := barHeight / 2

			for y := yMid - halfHeight; y <= yMid+halfHeight; y++ {
				if y >= yStart && y < yStart+stemPixelHeight {
					// Calculate intensity based on distance from center
					dist := abs(y - yMid)
					intensity := 1.0 - float64(dist)/float64(halfHeight+1)*0.3

					c := scaleColor(stemData.Color, intensity)
					waveformImg.SetRGBA(x, y, c)
				}
			}
		}

		// Draw separator line
		if i < len(stemDataList)-1 {
			sepY := yStart + stemPixelHeight - 1
			sepColor := color.RGBA{R: 50, G: 50, B: 55, A: 255}
			for x := 0; x < waveformWidth; x++ {
				waveformImg.SetRGBA(x, sepY, sepColor)
			}
		}
	}

	// Resize waveform if requested (before adding labels)
	finalWaveform := waveformImg
	if config.ResizeWidth > 0 && config.ResizeHeight > 0 {
		finalWaveform = resizeImage(waveformImg, config.ResizeWidth, config.ResizeHeight)
	}

	// Create final image with labels on top
	finalWidth := finalWaveform.Bounds().Dx()
	finalWaveformHeight := finalWaveform.Bounds().Dy()
	finalHeight := finalWaveformHeight
	labelOffset := 0

	if config.ShowLabels {
		finalHeight += config.LabelHeight
		labelOffset = config.LabelHeight
	}

	img := image.NewRGBA(image.Rect(0, 0, finalWidth, finalHeight))

	// Fill label area background
	if config.ShowLabels {
		labelBg := color.RGBA{R: 25, G: 25, B: 30, A: 255}
		for y := 0; y < config.LabelHeight; y++ {
			for x := 0; x < finalWidth; x++ {
				img.SetRGBA(x, y, labelBg)
			}
		}
	}

	// Copy waveform to final image
	for y := 0; y < finalWaveformHeight; y++ {
		for x := 0; x < finalWidth; x++ {
			img.SetRGBA(x, y+labelOffset, finalWaveform.RGBAAt(x, y))
		}
	}

	// Draw labels at top if enabled
	if config.ShowLabels {
		drawLabelsTop(img, stemDataList, config.LabelHeight, finalWidth)
	}

	// Save output
	if outputPath != "" {
		if err := saveImage(img, outputPath); err != nil {
			return nil, fmt.Errorf("failed to save image: %w", err)
		}
	}

	return &Result{
		Image:    img,
		Stems:    stemDataList,
		Duration: info.Duration,
	}, nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func scaleColor(c color.RGBA, scale float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(c.R) * scale),
		G: uint8(float64(c.G) * scale),
		B: uint8(float64(c.B) * scale),
		A: c.A,
	}
}

// resizeImage resizes an image using bilinear interpolation
func resizeImage(src *image.RGBA, newWidth, newHeight int) *image.RGBA {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	xRatio := float64(srcW) / float64(newWidth)
	yRatio := float64(srcH) / float64(newHeight)

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			// Calculate source coordinates
			srcX := float64(x) * xRatio
			srcY := float64(y) * yRatio

			// Get integer and fractional parts
			x0 := int(srcX)
			y0 := int(srcY)
			xFrac := srcX - float64(x0)
			yFrac := srcY - float64(y0)

			// Clamp to bounds
			x1 := x0 + 1
			y1 := y0 + 1
			if x1 >= srcW {
				x1 = srcW - 1
			}
			if y1 >= srcH {
				y1 = srcH - 1
			}

			// Get four neighboring pixels
			c00 := src.RGBAAt(x0, y0)
			c10 := src.RGBAAt(x1, y0)
			c01 := src.RGBAAt(x0, y1)
			c11 := src.RGBAAt(x1, y1)

			// Bilinear interpolation
			r := bilinear(float64(c00.R), float64(c10.R), float64(c01.R), float64(c11.R), xFrac, yFrac)
			g := bilinear(float64(c00.G), float64(c10.G), float64(c01.G), float64(c11.G), xFrac, yFrac)
			b := bilinear(float64(c00.B), float64(c10.B), float64(c01.B), float64(c11.B), xFrac, yFrac)
			a := bilinear(float64(c00.A), float64(c10.A), float64(c01.A), float64(c11.A), xFrac, yFrac)

			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(r),
				G: uint8(g),
				B: uint8(b),
				A: uint8(a),
			})
		}
	}

	return dst
}

func bilinear(c00, c10, c01, c11, xFrac, yFrac float64) float64 {
	return c00*(1-xFrac)*(1-yFrac) + c10*xFrac*(1-yFrac) + c01*(1-xFrac)*yFrac + c11*xFrac*yFrac
}

// stemDisplayNames maps internal stem names to display names
var stemDisplayNames = map[string]string{
	"vocals": "vocals",
	"drums":  "drums",
	"bass":   "bass",
	"other":  "other",
	"piano":  "piano",
	"guitar": "guitar",
	"mixed":  "mixed",
}

// drawLabelsTop draws stem labels horizontally at the top of the image
func drawLabelsTop(img *image.RGBA, stems []StemData, labelHeight, totalWidth int) {
	// Calculate spacing for labels
	numStems := len(stems)
	if numStems == 0 {
		return
	}

	// Draw label bar background
	labelBg := color.RGBA{R: 25, G: 25, B: 30, A: 255}
	for y := 0; y < labelHeight; y++ {
		for x := 0; x < totalWidth; x++ {
			img.SetRGBA(x, y, labelBg)
		}
	}

	// Calculate label positions - evenly spaced
	labelSpacing := totalWidth / numStems
	yMid := labelHeight / 2

	for i, stem := range stems {
		xStart := i*labelSpacing + 10

		// Draw color indicator square
		indicatorSize := 8
		for y := yMid - indicatorSize/2; y <= yMid+indicatorSize/2; y++ {
			for x := xStart; x < xStart+indicatorSize; x++ {
				img.SetRGBA(x, y, stem.Color)
			}
		}

		// Draw label text
		displayName := stemDisplayNames[stem.Label]
		if displayName == "" {
			displayName = stem.Label
		}
		drawText(img, displayName, xStart+indicatorSize+4, yMid-3, stem.Color)
	}
}

// drawText draws text using a simple bitmap font
func drawText(img *image.RGBA, text string, x, y int, c color.RGBA) {
	for _, ch := range text {
		pattern, ok := bitmapFont[byte(ch)]
		if !ok {
			x += 6 // space for unknown chars
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
	'g': {".###.", "#....", "#....", "#.###", "#...#", "#...#", ".###."},
	'h': {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'i': {".###.", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'l': {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'm': {"#...#", "##.##", "#.#.#", "#...#", "#...#", "#...#", "#...#"},
	'n': {"#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#", "#...#"},
	'o': {".###.", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'p': {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'r': {"####.", "#...#", "#...#", "####.", "#.#..", "#..#.", "#...#"},
	's': {".####", "#....", "#....", ".###.", "....#", "....#", "####."},
	't': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'u': {"#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'v': {"#...#", "#...#", "#...#", "#...#", ".#.#.", ".#.#.", "..#.."},
	'x': {"#...#", ".#.#.", "..#..", "..#..", "..#..", ".#.#.", "#...#"},
}

func saveImage(img *image.RGBA, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

// GenerateSimple generates a DNA visualization without stem separation.
func GenerateSimple(ctx context.Context, inputPath, outputPath string, width int) (*Result, error) {
	config := DefaultConfig()
	config.Width = width
	config.SkipStems = true
	return Generate(ctx, inputPath, outputPath, config)
}
