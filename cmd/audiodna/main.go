package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pforret/videodna/internal/audio"
	"github.com/pforret/videodna/internal/audiodna"
)

func main() {
	// Define flags
	input := flag.String("input", "", "Input audio file (required)")
	output := flag.String("output", "audiodna.png", "Output PNG file")
	width := flag.Int("width", 1920, "Output width in pixels (X = time)")
	stemHeight := flag.Int("stem-height", 50, "Height per stem in pixels")
	stems := flag.Int("stems", 4, "Number of stems: 2, 4, or 6")
	separator := flag.String("separator", "demucs", "Stem separator: demucs or spleeter")
	model := flag.String("model", "", "Model name (e.g., htdemucs, htdemucs_6s)")
	device := flag.String("device", "cpu", "Device: cpu or cuda")
	noStems := flag.Bool("no-stems", false, "Skip stem separation, use original audio only")
	noLabels := flag.Bool("no-labels", false, "Hide stem labels")
	noNormalize := flag.Bool("no-normalize", false, "Don't normalize volume levels")
	timeout := flag.Int("timeout", 600, "Timeout in seconds (default 10 minutes)")
	silent := flag.Bool("silent", false, "Suppress stdout output")

	// Custom usage
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Audio DNA Generator - Create visual DNA from audio with stem separation\n\n")
		fmt.Fprintf(os.Stderr, "Usage: audiodna -input <audio> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Stem Separation:
  Uses Demucs (default) or Spleeter to separate audio into:
    2 stems: vocals + accompaniment
    4 stems: vocals + drums + bass + other (default)
    6 stems: vocals + drums + bass + other + piano + guitar (demucs only)

Output:
  X-axis = time, Y-axis = volume per stem
  Each stem is shown as a waveform with distinct color:
    vocals (red), drums (blue), bass (green), other (purple)
    piano (yellow), guitar (orange)

Examples:
  # Simple usage with default 4-stem separation
  audiodna -input song.mp3 -output dna.png

  # No stem separation (just show overall waveform)
  audiodna -input song.mp3 -output dna.png -no-stems

  # 6-stem separation with GPU acceleration
  audiodna -input song.mp3 -stems 6 -device cuda

  # Use Spleeter instead of Demucs
  audiodna -input song.mp3 -separator spleeter

  # Custom dimensions
  audiodna -input song.mp3 -width 3840 -stem-height 80

Dependencies:
  - ffmpeg/ffprobe (required)
  - demucs: pip install demucs
  - spleeter: pip install spleeter

Docker:
  docker run -v $(pwd):/data audiodna -input /data/song.mp3 -output /data/dna.png
`)
	}

	flag.Parse()

	// Validate input
	if *input == "" {
		fmt.Fprintln(os.Stderr, "Error: -input is required")
		flag.Usage()
		os.Exit(1)
	}

	// Check if input file exists
	if _, err := os.Stat(*input); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: input file does not exist: %s\n", *input)
		os.Exit(1)
	}

	// Validate stems count
	if *stems != 2 && *stems != 4 && *stems != 6 {
		fmt.Fprintln(os.Stderr, "Error: -stems must be 2, 4, or 6")
		os.Exit(1)
	}

	// Validate separator
	sep := audio.SeparatorType(strings.ToLower(*separator))
	if sep != audio.SeparatorDemucs && sep != audio.SeparatorSpleeter {
		fmt.Fprintln(os.Stderr, "Error: -separator must be 'demucs' or 'spleeter'")
		os.Exit(1)
	}

	// 6 stems only supported by demucs
	if *stems == 6 && sep == audio.SeparatorSpleeter {
		fmt.Fprintln(os.Stderr, "Error: 6-stem separation requires demucs (spleeter only supports 2, 4, or 5 stems)")
		os.Exit(1)
	}

	// Build config
	config := audiodna.DefaultConfig()
	config.Width = *width
	config.StemHeight = *stemHeight
	config.StemConfig.NumStems = *stems
	config.StemConfig.Separator = sep
	config.StemConfig.Device = *device
	if *model != "" {
		config.StemConfig.Model = *model
	}
	config.SkipStems = *noStems
	config.ShowLabels = !*noLabels
	config.Normalize = !*noNormalize
	config.Timeout = *timeout
	config.Silent = *silent

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	// Generate DNA
	startTime := time.Now()

	result, err := audiodna.Generate(ctx, *input, *output, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !*silent {
		elapsed := time.Since(startTime)
		fmt.Printf("\nCompleted in %.2fs\n", elapsed.Seconds())
		fmt.Printf("Duration: %.2fs, Stems: %d\n", result.Duration, len(result.Stems))
		fmt.Printf("Output: %s (%dx%d)\n", *output, result.Image.Bounds().Dx(), result.Image.Bounds().Dy())
	}
}
