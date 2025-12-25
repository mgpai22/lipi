package audio

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// audio chunk info
type ChunkInfo struct {
	Path      string
	Index     int
	StartTime time.Duration
	EndTime   time.Duration
}

// settings for audio compression
type CompressionOptions struct {
	Format     string // Output format (mp3, aac, etc.)
	SampleRate int    // Sample rate in Hz
	Channels   int    // Number of channels (1=mono, 2=stereo)
	Bitrate    string // Bitrate (e.g., "64k", "128k")
}

// defaults for transcription
func DefaultCompressionOptions() CompressionOptions {
	return CompressionOptions{
		Format:     "mp3",
		SampleRate: 16000,
		Channels:   1,
		Bitrate:    "64k",
	}
}

// JSON output from ffprobe
type ffprobeOutput struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// duration of an audio/video file
func GetDuration(filePath string) (time.Duration, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return 0, fmt.Errorf("file not found: %s", filePath)
	}

	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		filePath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(out.Bytes(), &probe); err != nil {
		return 0, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	var seconds float64
	if _, err := fmt.Sscanf(probe.Format.Duration, "%f", &seconds); err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return time.Duration(seconds * float64(time.Second)), nil
}

// compresses an audio file with the given options
func CompressAudio(
	ctx context.Context,
	inputPath, outputPath string,
	opts CompressionOptions,
) error {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", inputPath)
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	kwargs := ffmpeg.KwArgs{
		"vn": "",              // No video
		"ar": opts.SampleRate, // Sample rate
		"ac": opts.Channels,   // Channels
		"y":  "",              // Overwrite output
	}

	switch opts.Format {
	case "mp3":
		kwargs["acodec"] = "libmp3lame"
		if opts.Bitrate != "" {
			kwargs["b:a"] = opts.Bitrate
		}
	case "aac":
		kwargs["acodec"] = "aac"
		if opts.Bitrate != "" {
			kwargs["b:a"] = opts.Bitrate
		}
	default:
		kwargs["acodec"] = "libmp3lame"
		if opts.Bitrate != "" {
			kwargs["b:a"] = opts.Bitrate
		}
	}

	err := ffmpeg.Input(inputPath).
		Output(outputPath, kwargs).
		OverWriteOutput().
		Run()

	if err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}

	return nil
}

// splits an audio file into chunks of specified duration
func ChunkAudio(
	ctx context.Context,
	audioPath string,
	chunkDuration time.Duration,
	outputDir string,
) ([]ChunkInfo, error) {
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("audio file not found: %s", audioPath)
	}

	totalDuration, err := GetDuration(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio duration: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	var chunks []ChunkInfo
	baseName := strings.TrimSuffix(
		filepath.Base(audioPath),
		filepath.Ext(audioPath),
	)
	ext := filepath.Ext(audioPath)

	chunkSeconds := chunkDuration.Seconds()
	totalSeconds := totalDuration.Seconds()

	for i := 0; ; i++ {
		startSeconds := float64(i) * chunkSeconds
		if startSeconds >= totalSeconds {
			break
		}

		endSeconds := startSeconds + chunkSeconds
		if endSeconds > totalSeconds {
			endSeconds = totalSeconds
		}

		chunkPath := filepath.Join(
			outputDir,
			fmt.Sprintf("%s_chunk_%03d%s", baseName, i, ext),
		)

		kwargs := ffmpeg.KwArgs{
			"ss": startSeconds,
			"t":  endSeconds - startSeconds,
			"y":  "",
			"c":  "copy", // Copy codec for speed
		}

		err := ffmpeg.Input(audioPath).
			Output(chunkPath, kwargs).
			OverWriteOutput().
			Run()

		if err != nil {
			return nil, fmt.Errorf("failed to create chunk %d: %w", i, err)
		}

		chunks = append(chunks, ChunkInfo{
			Path:      chunkPath,
			Index:     i,
			StartTime: time.Duration(startSeconds * float64(time.Second)),
			EndTime:   time.Duration(endSeconds * float64(time.Second)),
		})
	}

	return chunks, nil
}

// checks if the file is a video based on extension
func IsVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	videoExts := map[string]bool{
		".mp4":  true,
		".mkv":  true,
		".avi":  true,
		".mov":  true,
		".wmv":  true,
		".flv":  true,
		".webm": true,
		".m4v":  true,
		".mpeg": true,
		".mpg":  true,
		".3gp":  true,
	}
	return videoExts[ext]
}

// checks if the file is an audio file based on extension
func IsAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	audioExts := map[string]bool{
		".mp3":  true,
		".wav":  true,
		".aac":  true,
		".flac": true,
		".ogg":  true,
		".m4a":  true,
		".wma":  true,
		".aiff": true,
	}
	return audioExts[ext]
}

// checks if the file is either audio or video
func IsMediaFile(path string) bool {
	return IsAudioFile(path) || IsVideoFile(path)
}

// removes all chunk files
func CleanupChunks(chunks []ChunkInfo) error {
	var lastErr error
	for _, chunk := range chunks {
		if err := os.Remove(chunk.Path); err != nil && !os.IsNotExist(err) {
			lastErr = err
		}
	}
	return lastErr
}
