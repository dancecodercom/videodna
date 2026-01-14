package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pforret/videodna/internal/dna"
)

var version = "1.0.0"

func main() {
	inputFile := flag.String("input", "", "Input video file (required)")
	outputFile := flag.String("output", "output.png", "Output PNG file")
	mode := flag.String("mode", "average", "Color mode: average, min, max, common")
	vertical := flag.Bool("vertical", false, "Vertical output (width=video width, height=frames)")
	resize := flag.String("resize", "", "Resize output: 'WxH' or 'input' for video dimensions")
	silent := flag.Bool("silent", false, "Suppress stdout output")
	timeout := flag.Int("timeout", 60, "Timeout in seconds")
	name := flag.String("name", "", "Display name in legend (default: input filename)")
	noLegend := flag.Bool("no-legend", false, "Hide top legend bar")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "videodna v%s - Generate DNA fingerprint images from video files\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: videodna -input <video> [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nModes:\n")
		fmt.Fprintf(os.Stderr, "  average  Average RGB per row/column (default, fastest)\n")
		fmt.Fprintf(os.Stderr, "  min      Darkest color per row/column\n")
		fmt.Fprintf(os.Stderr, "  max      Brightest color per row/column\n")
		fmt.Fprintf(os.Stderr, "  common   Most frequent color per row/column (slowest)\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  videodna -input video.mp4 -output dna.png\n")
		fmt.Fprintf(os.Stderr, "  videodna -input video.mp4 -output dna.png -mode max\n")
		fmt.Fprintf(os.Stderr, "  videodna -input video.mp4 -output dna.png -vertical -resize input\n")
		fmt.Fprintf(os.Stderr, "  videodna -input video.mp4 -output dna.png -resize 1920x1080\n")
		fmt.Fprintf(os.Stderr, "  videodna -input video.mp4 -output dna.png -name \"My Video\"\n")
	}

	flag.Parse()

	if *inputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	validModes := map[string]bool{"average": true, "min": true, "max": true, "common": true}
	if !validModes[*mode] {
		fmt.Fprintf(os.Stderr, "Error: Invalid mode '%s'. Use: average, min, max, common\n", *mode)
		os.Exit(1)
	}

	legend := dna.DefaultLegendConfig()
	legend.Enabled = !*noLegend
	legend.Name = *name

	if err := dna.GenerateWithLegend(*inputFile, *outputFile, *mode, *vertical, *resize, *silent, *timeout, legend); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !*silent {
		fmt.Printf("Video DNA generated: %s\n", *outputFile)
	}
}
