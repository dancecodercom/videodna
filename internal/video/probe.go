package video

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type probeResult struct {
	Streams []struct {
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		NbFrames     string `json:"nb_frames"`
		CodecName    string `json:"codec_name"`
		RFrameRate   string `json:"r_frame_rate"`
		AvgFrameRate string `json:"avg_frame_rate"`
		Duration     string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// Info contains video metadata.
type Info struct {
	Width      int
	Height     int
	FrameCount int
	Duration   float64
	FPS        float64
	Codec      string
}

// GetInfo returns video width, height, and frame count using ffprobe.
func GetInfo(inputPath string) (width, height, frameCount int, err error) {
	info, err := GetFullInfo(inputPath)
	if err != nil {
		return 0, 0, 0, err
	}
	return info.Width, info.Height, info.FrameCount, nil
}

// GetFullInfo returns complete video metadata using ffprobe.
func GetFullInfo(inputPath string) (*Info, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,nb_frames,codec_name,r_frame_rate,avg_frame_rate,duration",
		"-show_entries", "format=duration",
		"-of", "json",
		inputPath)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe probeResult
	if err := json.Unmarshal(out, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	if len(probe.Streams) == 0 {
		return nil, fmt.Errorf("no video streams found")
	}

	s := probe.Streams[0]
	info := &Info{
		Width:  s.Width,
		Height: s.Height,
		Codec:  s.CodecName,
	}

	// Parse frame count
	fmt.Sscanf(s.NbFrames, "%d", &info.FrameCount)

	// Parse duration (prefer stream, fallback to format)
	if s.Duration != "" {
		info.Duration, _ = strconv.ParseFloat(s.Duration, 64)
	} else if probe.Format.Duration != "" {
		info.Duration, _ = strconv.ParseFloat(probe.Format.Duration, 64)
	}

	// Parse FPS from r_frame_rate or avg_frame_rate (format: "num/den")
	fpsStr := s.RFrameRate
	if fpsStr == "" || fpsStr == "0/0" {
		fpsStr = s.AvgFrameRate
	}
	if parts := strings.Split(fpsStr, "/"); len(parts) == 2 {
		num, _ := strconv.ParseFloat(parts[0], 64)
		den, _ := strconv.ParseFloat(parts[1], 64)
		if den > 0 {
			info.FPS = num / den
		}
	}

	return info, nil
}
