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
	"sync"

	"github.com/pforret/videodna/internal/audio"
)

// Config configures DNA generation.
type Config struct {
	Width        int                 // Output width in pixels (X = time)
	Height       int                 // Output height in pixels (auto-calculated if 0)
	StemConfig   audio.StemConfig    // Stem separation config
	SkipStems    bool                // If true, use original audio only
	Normalize    bool                // Normalize volume levels
	ColorScheme  ColorScheme         // Color scheme for visualization
	StemHeight   int                 // Height per stem in pixels (default: 50)
	ShowLabels   bool                // Show stem labels on left side
	LabelWidth   int                 // Width of label area (default: 60)
	Timeout      int                 // Timeout in seconds
	Silent       bool                // Suppress progress output
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		Width:       1920,
		Height:      0, // Auto-calculate
		StemConfig:  audio.DefaultStemConfig(),
		SkipStems:   false,
		Normalize:   true,
		ColorScheme: SchemeDefault,
		StemHeight:  50,
		ShowLabels:  true,
		LabelWidth:  60,
		Timeout:     600, // 10 minutes default for stem separation
		Silent:      false,
	}
}

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
	if !config.Silent {
		fmt.Printf("Processing: %s\n", inputPath)
	}

	// Get audio info
	info, err := audio.GetInfo(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio info: %w", err)
	}

	if !config.Silent {
		fmt.Printf("Duration: %.2fs, Sample Rate: %d Hz, Channels: %d\n",
			info.Duration, info.SampleRate, info.Channels)
	}

	var stemFiles *audio.StemFiles
	var stemLabels []string
	var stemPaths []string

	if !config.SkipStems {
		// Check if separator is available
		if err := audio.CheckSeparatorAvailable(config.StemConfig.Separator); err != nil {
			if !config.Silent {
				fmt.Printf("Warning: %v\n", err)
				fmt.Println("Falling back to original audio only")
			}
			config.SkipStems = true
		}
	}

	if !config.SkipStems {
		if !config.Silent {
			fmt.Printf("Separating stems using %s (%d stems)...\n",
				config.StemConfig.Separator, config.StemConfig.NumStems)
		}

		stemFiles, err = audio.SeparateStems(ctx, inputPath, config.StemConfig)
		if err != nil {
			return nil, fmt.Errorf("stem separation failed: %w", err)
		}

		stemPaths = stemFiles.GetStemPaths()
		stemLabels = stemFiles.GetStemLabels()

		if !config.Silent {
			fmt.Printf("Separated into %d stems\n", len(stemPaths))
		}
	}

	// If no stems, use original audio
	if len(stemPaths) == 0 {
		stemPaths = []string{inputPath}
		stemLabels = []string{"mixed"}
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

			if !config.Silent {
				fmt.Printf("Extracting waveform for %s...\n", label)
			}

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

	// Calculate output dimensions
	outputHeight := config.Height
	if outputHeight == 0 {
		outputHeight = len(stemDataList) * config.StemHeight
	}

	outputWidth := config.Width
	if config.ShowLabels {
		outputWidth += config.LabelWidth
	}

	// Create output image
	img := image.NewRGBA(image.Rect(0, 0, outputWidth, outputHeight))

	// Fill background
	bgColor := color.RGBA{R: 20, G: 20, B: 25, A: 255}
	for y := 0; y < outputHeight; y++ {
		for x := 0; x < outputWidth; x++ {
			img.SetRGBA(x, y, bgColor)
		}
	}

	// Draw each stem
	stemPixelHeight := outputHeight / len(stemDataList)
	xOffset := 0
	if config.ShowLabels {
		xOffset = config.LabelWidth
	}

	for i, stemData := range stemDataList {
		yStart := i * stemPixelHeight
		yMid := yStart + stemPixelHeight/2

		// Draw waveform
		for x, seg := range stemData.Segments {
			if x >= config.Width {
				break
			}

			// Calculate bar height based on RMS
			barHeight := int(seg.RMS * float64(stemPixelHeight) * 0.8)
			if barHeight < 1 {
				barHeight = 1
			}

			// Draw symmetric waveform
			halfHeight := barHeight / 2
			xPos := x + xOffset

			for y := yMid - halfHeight; y <= yMid+halfHeight; y++ {
				if y >= yStart && y < yStart+stemPixelHeight {
					// Calculate intensity based on distance from center
					dist := abs(y - yMid)
					intensity := 1.0 - float64(dist)/float64(halfHeight+1)*0.3

					c := scaleColor(stemData.Color, intensity)
					img.SetRGBA(xPos, y, c)
				}
			}
		}

		// Draw separator line
		if i < len(stemDataList)-1 {
			sepY := yStart + stemPixelHeight - 1
			sepColor := color.RGBA{R: 50, G: 50, B: 55, A: 255}
			for x := 0; x < outputWidth; x++ {
				img.SetRGBA(x, sepY, sepColor)
			}
		}
	}

	// Draw labels if enabled
	if config.ShowLabels {
		drawLabels(img, stemDataList, stemPixelHeight, config.LabelWidth)
	}

	// Save output
	if outputPath != "" {
		if err := saveImage(img, outputPath); err != nil {
			return nil, fmt.Errorf("failed to save image: %w", err)
		}
		if !config.Silent {
			fmt.Printf("Saved: %s\n", outputPath)
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

func drawLabels(img *image.RGBA, stems []StemData, stemHeight, labelWidth int) {
	// Simple label drawing using filled rectangles with stem color
	// For proper text, would need freetype or similar
	labelBg := color.RGBA{R: 30, G: 30, B: 35, A: 255}

	for i, stem := range stems {
		yStart := i * stemHeight
		yMid := yStart + stemHeight/2

		// Draw label background
		for y := yStart; y < yStart+stemHeight; y++ {
			for x := 0; x < labelWidth; x++ {
				img.SetRGBA(x, y, labelBg)
			}
		}

		// Draw color indicator
		indicatorSize := 8
		indicatorX := 10
		for y := yMid - indicatorSize/2; y <= yMid+indicatorSize/2; y++ {
			for x := indicatorX; x < indicatorX+indicatorSize; x++ {
				img.SetRGBA(x, y, stem.Color)
			}
		}

		// Draw simple letter representation (first letter of label)
		// This is a basic approach - for real text rendering, use a font library
		drawLetter(img, stem.Label[0], indicatorX+indicatorSize+6, yMid-4, stem.Color)
	}
}

// drawLetter draws a simple 5x7 pixel letter
func drawLetter(img *image.RGBA, letter byte, x, y int, c color.RGBA) {
	// Simple 5x7 bitmap font for basic letters
	letters := map[byte][]string{
		'v': {
			"#...#",
			"#...#",
			"#...#",
			".#.#.",
			".#.#.",
			"..#..",
			"..#..",
		},
		'd': {
			"#....",
			"#....",
			"#.##.",
			"##..#",
			"#...#",
			"#...#",
			".###.",
		},
		'b': {
			"#....",
			"#....",
			"####.",
			"#...#",
			"#...#",
			"#...#",
			"####.",
		},
		'o': {
			".....",
			".....",
			".###.",
			"#...#",
			"#...#",
			"#...#",
			".###.",
		},
		'p': {
			".....",
			".....",
			"####.",
			"#...#",
			"####.",
			"#....",
			"#....",
		},
		'g': {
			".....",
			".....",
			".####",
			"#....",
			"#.###",
			"#...#",
			".###.",
		},
		'm': {
			".....",
			".....",
			"##.#.",
			"#.#.#",
			"#...#",
			"#...#",
			"#...#",
		},
	}

	pattern, ok := letters[letter]
	if !ok {
		return
	}

	for dy, row := range pattern {
		for dx, ch := range row {
			if ch == '#' {
				img.SetRGBA(x+dx, y+dy, c)
			}
		}
	}
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
