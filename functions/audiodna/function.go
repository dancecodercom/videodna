// Package audiodna provides a Cloud Function for generating audio DNA visualizations.
//
// This can be deployed to:
// - Google Cloud Functions (Go runtime)
// - AWS Lambda (via custom Go runtime)
// - Any serverless platform supporting Go
//
// Note: Stem separation requires Demucs which is heavy (~1GB+ with PyTorch).
// For serverless, consider:
// 1. Using -no-stems mode for lightweight waveform only
// 2. Running stem separation as a separate container service
// 3. Pre-separating stems and passing them to this function
package audiodna

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pforret/videodna/internal/audio"
	"github.com/pforret/github.com/pforret/videodna/internal/audiodna"
)

// Request is the Cloud Function request format.
type Request struct {
	// AudioURL is a URL to fetch the audio file from
	AudioURL string `json:"audio_url,omitempty"`

	// AudioBase64 is base64-encoded audio data (for small files)
	AudioBase64 string `json:"audio_base64,omitempty"`

	// Filename is the original filename (used for temp file extension)
	Filename string `json:"filename,omitempty"`

	// Options
	Width      int  `json:"width,omitempty"`       // Output width (default: 1920)
	StemHeight int  `json:"stem_height,omitempty"` // Height per stem (default: 50)
	NumStems   int  `json:"num_stems,omitempty"`   // 2, 4, or 6 (default: 4)
	NoStems    bool `json:"no_stems,omitempty"`    // Skip stem separation
	NoLabels   bool `json:"no_labels,omitempty"`   // Hide labels
}

// Response is the Cloud Function response format.
type Response struct {
	// ImageBase64 is the PNG image encoded as base64
	ImageBase64 string `json:"image_base64,omitempty"`

	// ImageURL is a URL where the image was uploaded (if configured)
	ImageURL string `json:"image_url,omitempty"`

	// Metadata
	Duration   float64  `json:"duration"`
	Stems      []string `json:"stems"`
	Width      int      `json:"width"`
	Height     int      `json:"height"`

	// Error info
	Error string `json:"error,omitempty"`
}

// HandleHTTP is the HTTP Cloud Function entry point.
func HandleHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	// Parse request
	var req Request
	if r.Method == http.MethodPost {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendError(w, "Invalid JSON request", http.StatusBadRequest)
			return
		}
	} else if r.Method == http.MethodGet {
		req.AudioURL = r.URL.Query().Get("url")
		req.NoStems = r.URL.Query().Get("no_stems") == "true"
	} else {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Process
	resp, err := Process(ctx, req)
	if err != nil {
		sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Process generates the audio DNA and returns the result.
func Process(ctx context.Context, req Request) (*Response, error) {
	// Get audio data
	audioPath, cleanup, err := getAudioFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio: %w", err)
	}
	defer cleanup()

	// Configure
	config := audiodna.DefaultConfig()
	if req.Width > 0 {
		config.Width = req.Width
	}
	if req.StemHeight > 0 {
		config.StemHeight = req.StemHeight
	}
	if req.NumStems > 0 {
		config.StemConfig.NumStems = req.NumStems
	}
	config.SkipStems = req.NoStems
	config.ShowLabels = !req.NoLabels
	config.Silent = true

	// For cloud functions, check if demucs is available
	if !config.SkipStems {
		if err := audio.CheckSeparatorAvailable(audio.SeparatorDemucs); err != nil {
			// Fallback to no stems if demucs not available
			config.SkipStems = true
		}
	}

	// Generate
	result, err := audiodna.Generate(ctx, audioPath, "", config)
	if err != nil {
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	// Encode image to base64
	var imgBuf strings.Builder
	b64Writer := base64.NewEncoder(base64.StdEncoding, &imgBuf)
	if err := png.Encode(b64Writer, result.Image); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}
	b64Writer.Close()

	// Build response
	resp := &Response{
		ImageBase64: imgBuf.String(),
		Duration:    result.Duration,
		Width:       result.Image.Bounds().Dx(),
		Height:      result.Image.Bounds().Dy(),
	}

	for _, stem := range result.Stems {
		resp.Stems = append(resp.Stems, stem.Label)
	}

	return resp, nil
}

func getAudioFile(ctx context.Context, req Request) (string, func(), error) {
	// Determine file extension
	ext := ".mp3"
	if req.Filename != "" {
		ext = filepath.Ext(req.Filename)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "audiodna-*"+ext)
	if err != nil {
		return "", nil, err
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		os.Remove(tmpPath)
	}

	// Get audio data
	if req.AudioBase64 != "" {
		// Decode base64
		data, err := base64.StdEncoding.DecodeString(req.AudioBase64)
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("invalid base64: %w", err)
		}
		if _, err := tmpFile.Write(data); err != nil {
			cleanup()
			return "", nil, err
		}
	} else if req.AudioURL != "" {
		// Fetch from URL
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.AudioURL, nil)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			cleanup()
			return "", nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			cleanup()
			return "", nil, fmt.Errorf("failed to fetch audio: %s", resp.Status)
		}

		if _, err := io.Copy(tmpFile, resp.Body); err != nil {
			cleanup()
			return "", nil, err
		}
	} else {
		cleanup()
		return "", nil, fmt.Errorf("no audio provided: use audio_url or audio_base64")
	}

	tmpFile.Close()
	return tmpPath, cleanup, nil
}

func sendError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(Response{Error: msg})
}
