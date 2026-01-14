// Package audio provides waveform extraction and analysis.
package audio

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
)

// WaveformData contains amplitude data for an audio file.
type WaveformData struct {
	Samples    []float64 // Normalized samples (-1.0 to 1.0)
	SampleRate int       // Sample rate in Hz
	Duration   float64   // Duration in seconds
	Channels   int       // Number of channels (mixed to mono)
}

// WaveformConfig configures waveform extraction.
type WaveformConfig struct {
	SampleRate int // Target sample rate (default: 44100)
	Mono       bool // Mix to mono (default: true)
}

// DefaultWaveformConfig returns default configuration.
func DefaultWaveformConfig() WaveformConfig {
	return WaveformConfig{
		SampleRate: 44100,
		Mono:       true,
	}
}

// ExtractWaveform extracts raw waveform data from an audio file.
func ExtractWaveform(ctx context.Context, inputPath string, config WaveformConfig) (*WaveformData, error) {
	if config.SampleRate == 0 {
		config.SampleRate = 44100
	}

	// Build ffmpeg command to output raw PCM
	args := []string{
		"-i", inputPath,
		"-f", "s16le",        // 16-bit signed little-endian
		"-acodec", "pcm_s16le",
		"-ar", fmt.Sprintf("%d", config.SampleRate),
	}

	if config.Mono {
		args = append(args, "-ac", "1") // Mix to mono
	}

	args = append(args, "-") // Output to stdout

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed to start: %w", err)
	}

	// Read samples
	reader := bufio.NewReaderSize(stdout, 1024*1024) // 1MB buffer
	var samples []float64

	buf := make([]byte, 2) // 16-bit = 2 bytes
	for {
		_, err := io.ReadFull(reader, buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		// Convert to float64 normalized to -1.0 to 1.0
		sample := int16(binary.LittleEndian.Uint16(buf))
		samples = append(samples, float64(sample)/32768.0)
	}

	if err := cmd.Wait(); err != nil {
		// Ignore exit errors if we got samples (ffmpeg sometimes exits with error after EOF)
		if len(samples) == 0 {
			return nil, fmt.Errorf("ffmpeg failed: %w", err)
		}
	}

	channels := 1
	if !config.Mono {
		// Would need to track original channel count
		channels = 2
	}

	return &WaveformData{
		Samples:    samples,
		SampleRate: config.SampleRate,
		Duration:   float64(len(samples)) / float64(config.SampleRate),
		Channels:   channels,
	}, nil
}

// VolumeSegment represents volume data for a time segment.
type VolumeSegment struct {
	TimeStart float64 // Start time in seconds
	TimeEnd   float64 // End time in seconds
	RMS       float64 // RMS volume (0.0 to 1.0)
	Peak      float64 // Peak amplitude (0.0 to 1.0)
	Min       float64 // Minimum amplitude (-1.0 to 1.0)
	Max       float64 // Maximum amplitude (-1.0 to 1.0)
}

// ExtractVolume extracts volume data segmented into time buckets.
func ExtractVolume(waveform *WaveformData, numSegments int) []VolumeSegment {
	if numSegments <= 0 || len(waveform.Samples) == 0 {
		return nil
	}

	samplesPerSegment := len(waveform.Samples) / numSegments
	if samplesPerSegment < 1 {
		samplesPerSegment = 1
	}

	segments := make([]VolumeSegment, numSegments)
	secondsPerSample := 1.0 / float64(waveform.SampleRate)

	for i := 0; i < numSegments; i++ {
		startIdx := i * samplesPerSegment
		endIdx := startIdx + samplesPerSegment
		if i == numSegments-1 {
			endIdx = len(waveform.Samples) // Last segment gets remaining samples
		}
		if endIdx > len(waveform.Samples) {
			endIdx = len(waveform.Samples)
		}

		segment := &segments[i]
		segment.TimeStart = float64(startIdx) * secondsPerSample
		segment.TimeEnd = float64(endIdx) * secondsPerSample
		segment.Min = 1.0
		segment.Max = -1.0

		var sumSquares float64
		count := 0

		for j := startIdx; j < endIdx; j++ {
			sample := waveform.Samples[j]
			absSample := math.Abs(sample)

			sumSquares += sample * sample
			count++

			if sample < segment.Min {
				segment.Min = sample
			}
			if sample > segment.Max {
				segment.Max = sample
			}
			if absSample > segment.Peak {
				segment.Peak = absSample
			}
		}

		if count > 0 {
			segment.RMS = math.Sqrt(sumSquares / float64(count))
		}
	}

	return segments
}

// NormalizeVolume normalizes volume segments to use full dynamic range.
func NormalizeVolume(segments []VolumeSegment) {
	if len(segments) == 0 {
		return
	}

	// Find max RMS
	var maxRMS float64
	for _, seg := range segments {
		if seg.RMS > maxRMS {
			maxRMS = seg.RMS
		}
	}

	if maxRMS == 0 {
		return
	}

	// Normalize
	scale := 1.0 / maxRMS
	for i := range segments {
		segments[i].RMS *= scale
		if segments[i].RMS > 1.0 {
			segments[i].RMS = 1.0
		}
	}
}
