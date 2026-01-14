// Package audio provides stem separation functionality using Demucs or Spleeter.
package audio

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// StemType represents different audio stems.
type StemType string

const (
	StemVocals  StemType = "vocals"
	StemDrums   StemType = "drums"
	StemBass    StemType = "bass"
	StemOther   StemType = "other"
	StemPiano   StemType = "piano"
	StemGuitar  StemType = "guitar"
	StemMixed   StemType = "mixed" // Original mixed audio
)

// SeparatorType represents the stem separation backend.
type SeparatorType string

const (
	SeparatorDemucs   SeparatorType = "demucs"
	SeparatorSpleeter SeparatorType = "spleeter"
)

// StemConfig configures stem separation.
type StemConfig struct {
	Separator  SeparatorType
	NumStems   int    // 2, 4, or 5 stems
	Model      string // Model name (e.g., "htdemucs", "htdemucs_6s")
	OutputDir  string // Directory to write stems
	Device     string // "cpu" or "cuda"
}

// DefaultStemConfig returns default configuration.
func DefaultStemConfig() StemConfig {
	return StemConfig{
		Separator: SeparatorDemucs,
		NumStems:  4,
		Model:     "htdemucs",
		Device:    "cpu",
	}
}

// StemFiles contains paths to separated stem files.
type StemFiles struct {
	Vocals string
	Drums  string
	Bass   string
	Other  string
	Piano  string
	Guitar string
}

// GetStemPaths returns a slice of all non-empty stem paths.
func (s *StemFiles) GetStemPaths() []string {
	var paths []string
	if s.Vocals != "" {
		paths = append(paths, s.Vocals)
	}
	if s.Drums != "" {
		paths = append(paths, s.Drums)
	}
	if s.Bass != "" {
		paths = append(paths, s.Bass)
	}
	if s.Other != "" {
		paths = append(paths, s.Other)
	}
	if s.Piano != "" {
		paths = append(paths, s.Piano)
	}
	if s.Guitar != "" {
		paths = append(paths, s.Guitar)
	}
	return paths
}

// GetStemLabels returns labels for the stems in the same order as GetStemPaths.
func (s *StemFiles) GetStemLabels() []string {
	var labels []string
	if s.Vocals != "" {
		labels = append(labels, "vocals")
	}
	if s.Drums != "" {
		labels = append(labels, "drums")
	}
	if s.Bass != "" {
		labels = append(labels, "bass")
	}
	if s.Other != "" {
		labels = append(labels, "other")
	}
	if s.Piano != "" {
		labels = append(labels, "piano")
	}
	if s.Guitar != "" {
		labels = append(labels, "guitar")
	}
	return labels
}

// SeparateStems separates an audio file into individual stems.
func SeparateStems(ctx context.Context, inputPath string, config StemConfig) (*StemFiles, error) {
	// Ensure output directory exists
	if config.OutputDir == "" {
		tmpDir, err := os.MkdirTemp("", "audiodna-stems-*")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
		config.OutputDir = tmpDir
	}

	if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output dir: %w", err)
	}

	switch config.Separator {
	case SeparatorDemucs:
		return separateWithDemucs(ctx, inputPath, config)
	case SeparatorSpleeter:
		return separateWithSpleeter(ctx, inputPath, config)
	default:
		return nil, fmt.Errorf("unknown separator: %s", config.Separator)
	}
}

func separateWithDemucs(ctx context.Context, inputPath string, config StemConfig) (*StemFiles, error) {
	// Determine model based on stem count
	model := config.Model
	if model == "" {
		switch config.NumStems {
		case 2:
			model = "htdemucs" // Will use vocals + no_vocals
		case 4:
			model = "htdemucs"
		case 6:
			model = "htdemucs_6s"
		default:
			model = "htdemucs"
		}
	}

	args := []string{
		"-n", model,
		"-o", config.OutputDir,
		"--device", config.Device,
		"--segment", "7", // Prevent OOM on long files (htdemucs max is 7.8s)
	}

	// Add two-stems flag for 2-stem separation
	if config.NumStems == 2 {
		args = append(args, "--two-stems", "vocals")
	}

	args = append(args, inputPath)

	cmd := exec.CommandContext(ctx, "demucs", args...)

	// Capture stderr to filter progress output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start demucs: %w", err)
	}

	// Process stderr in background, showing filtered progress
	go filterDemucsOutput(stderr)

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("demucs failed: %w", err)
	}

	// Find output files
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	stemDir := filepath.Join(config.OutputDir, model, baseName)

	stems := &StemFiles{}

	// Check for each possible stem file (try both .wav and .mp3)
	stemTypes := []struct {
		name string
		dest *string
	}{
		{"vocals", &stems.Vocals},
		{"drums", &stems.Drums},
		{"bass", &stems.Bass},
		{"other", &stems.Other},
		{"piano", &stems.Piano},
		{"guitar", &stems.Guitar},
		{"no_vocals", &stems.Other}, // For 2-stem mode
	}

	for _, st := range stemTypes {
		// Try wav first, then mp3
		for _, ext := range []string{".wav", ".mp3"} {
			path := filepath.Join(stemDir, st.name+ext)
			if _, err := os.Stat(path); err == nil {
				*st.dest = path
				break
			}
		}
	}

	return stems, nil
}

func separateWithSpleeter(ctx context.Context, inputPath string, config StemConfig) (*StemFiles, error) {
	// Determine stems argument
	stemsArg := "spleeter:4stems"
	switch config.NumStems {
	case 2:
		stemsArg = "spleeter:2stems"
	case 4:
		stemsArg = "spleeter:4stems"
	case 5:
		stemsArg = "spleeter:5stems"
	}

	args := []string{
		"separate",
		"-p", stemsArg,
		"-o", config.OutputDir,
		inputPath,
	}

	cmd := exec.CommandContext(ctx, "spleeter", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("spleeter failed: %w", err)
	}

	// Find output files
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	stemDir := filepath.Join(config.OutputDir, baseName)

	stems := &StemFiles{}

	// Check for each possible stem file
	stemTypes := []struct {
		name string
		dest *string
	}{
		{"vocals.wav", &stems.Vocals},
		{"drums.wav", &stems.Drums},
		{"bass.wav", &stems.Bass},
		{"other.wav", &stems.Other},
		{"piano.wav", &stems.Piano},
		{"accompaniment.wav", &stems.Other}, // For 2-stem mode
	}

	for _, st := range stemTypes {
		path := filepath.Join(stemDir, st.name)
		if _, err := os.Stat(path); err == nil {
			*st.dest = path
		}
	}

	return stems, nil
}

// CheckSeparatorAvailable checks if the specified separator is installed.
func CheckSeparatorAvailable(sep SeparatorType) error {
	var cmd string
	switch sep {
	case SeparatorDemucs:
		cmd = "demucs"
	case SeparatorSpleeter:
		cmd = "spleeter"
	default:
		return fmt.Errorf("unknown separator: %s", sep)
	}

	_, err := exec.LookPath(cmd)
	if err != nil {
		return fmt.Errorf("%s not found in PATH. Install it with: pip install %s", cmd, cmd)
	}
	return nil
}

// filterDemucsOutput reads demucs stderr and shows clean progress
func filterDemucsOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	// Match progress lines like "100%|██████| 5.85/5.85 [00:03<00:00, 1.91seconds/s]"
	progressRe := regexp.MustCompile(`(\d+)%\|[^|]*\|\s*([\d.]+)/([\d.]+)\s*\[([^\]]+)\]`)
	lastPct := -1
	startTime := time.Now()
	var lastLine string

	for scanner.Scan() {
		line := scanner.Text()

		// Skip OpenBLAS warnings and empty lines
		if strings.Contains(line, "OpenBLAS Warning") || strings.TrimSpace(line) == "" {
			continue
		}

		// Check for progress updates
		if matches := progressRe.FindStringSubmatch(line); matches != nil {
			pct := 0
			fmt.Sscanf(matches[1], "%d", &pct)

			// Only show progress at 10% intervals
			if pct/10 > lastPct/10 || pct == 100 {
				elapsed := time.Since(startTime).Seconds()
				throughput := 0.0
				if elapsed > 0 {
					var current float64
					fmt.Sscanf(matches[2], "%f", &current)
					throughput = current / elapsed
				}
				fmt.Printf("  Stem separation: %3d%% (%.1f sec/s)\n", pct, throughput)
				lastPct = pct
			}
			lastLine = line
		} else if strings.Contains(line, "Downloading") {
			// Show download progress
			fmt.Printf("  Downloading model...\n")
		}
	}
	_ = lastLine // suppress unused warning
}
