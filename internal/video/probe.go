package video

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type probeResult struct {
	Streams []struct {
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		NbFrames string `json:"nb_frames"`
	} `json:"streams"`
}

// GetInfo returns video width, height, and frame count using ffprobe.
func GetInfo(inputPath string) (width, height, frameCount int, err error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,nb_frames",
		"-of", "json",
		inputPath)

	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe probeResult
	if err := json.Unmarshal(out, &probe); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	if len(probe.Streams) == 0 {
		return 0, 0, 0, fmt.Errorf("no video streams found")
	}

	s := probe.Streams[0]
	var fc int
	fmt.Sscanf(s.NbFrames, "%d", &fc)
	return s.Width, s.Height, fc, nil
}
