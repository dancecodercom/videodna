// Package audio provides audio file probing and analysis utilities.
package audio

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Info contains metadata about an audio file.
type Info struct {
	Duration   float64 // Duration in seconds
	SampleRate int     // Sample rate in Hz
	Channels   int     // Number of audio channels
	BitRate    int     // Bit rate in bps
	Codec      string  // Audio codec name
}

type probeResult struct {
	Streams []probeStream `json:"streams"`
	Format  probeFormat   `json:"format"`
}

type probeStream struct {
	CodecName  string `json:"codec_name"`
	SampleRate string `json:"sample_rate"`
	Channels   int    `json:"channels"`
	BitRate    string `json:"bit_rate"`
}

type probeFormat struct {
	Duration string `json:"duration"`
	BitRate  string `json:"bit_rate"`
}

// GetInfo retrieves audio metadata using ffprobe.
func GetInfo(inputPath string) (*Info, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "a:0",
		inputPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var result probeResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	if len(result.Streams) == 0 {
		return nil, fmt.Errorf("no audio stream found in %s", inputPath)
	}

	stream := result.Streams[0]

	info := &Info{
		Codec:    stream.CodecName,
		Channels: stream.Channels,
	}

	// Parse duration
	if result.Format.Duration != "" {
		info.Duration, _ = strconv.ParseFloat(result.Format.Duration, 64)
	}

	// Parse sample rate
	if stream.SampleRate != "" {
		info.SampleRate, _ = strconv.Atoi(stream.SampleRate)
	}

	// Parse bit rate (prefer stream, fallback to format)
	bitRateStr := stream.BitRate
	if bitRateStr == "" {
		bitRateStr = result.Format.BitRate
	}
	if bitRateStr != "" {
		info.BitRate, _ = strconv.Atoi(strings.TrimSpace(bitRateStr))
	}

	return info, nil
}
