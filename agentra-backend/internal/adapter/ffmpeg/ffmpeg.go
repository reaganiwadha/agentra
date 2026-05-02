package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
)

type KeyframeInfo struct {
	Path         string
	FrameNumber  int
	TimestampSec float64
}

// ExtractAudio extracts the audio track as mono 16kHz MP3 into a temp file.
// The caller is responsible for removing the returned file.
func ExtractAudio(ctx context.Context, videoPath string) (string, error) {
	outFile, err := os.CreateTemp("", "agentra-audio-*.mp3")
	if err != nil {
		return "", err
	}
	out := outFile.Name()
	_ = outFile.Close()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-i", videoPath,
		"-vn", "-ac", "1", "-ar", "16000", "-c:a", "mp3", "-b:a", "64k",
		out,
	)
	if b, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(out)
		return "", fmt.Errorf("ffmpeg audio: %w: %s", err, b)
	}
	if err := requireOutputFile(out, "ffmpeg audio"); err != nil {
		_ = os.Remove(out)
		return "", err
	}
	return out, nil
}

// ExtractKeyframes extracts scene-change keyframes as JPEGs into a temp directory.
// Returns the directory path (caller removes it) and the list of frame file paths.
func ExtractKeyframes(ctx context.Context, videoPath string, maxFrames int) (string, []string, error) {
	if maxFrames <= 0 {
		maxFrames = 10
	}

	dir, err := os.MkdirTemp("", "agentra-frames-*")
	if err != nil {
		return "", nil, err
	}

	pattern := filepath.Join(dir, "frame%04d.jpg")
	sceneArgs := []string{
		"-y",
		"-i", videoPath,
		"-vf", "select='gt(scene,0.4)',format=yuvj420p",
		"-vsync", "vfr",
		"-pix_fmt", "yuvj420p",
		"-strict", "-1",
		"-q:v", "2",
		pattern,
	}
	primaryOut, primaryErr := runFFmpeg(ctx, sceneArgs...)
	if primaryErr != nil {
		// Fallback for codecs/pixel formats that fail scene filter JPEG encoding.
		fallbackArgs := []string{
			"-y",
			"-i", videoPath,
			"-vf", "fps=1,format=yuvj420p",
			"-vframes", fmt.Sprintf("%d", maxFrames),
			"-pix_fmt", "yuvj420p",
			"-strict", "-1",
			"-q:v", "2",
			pattern,
		}
		fallbackOut, fallbackErr := runFFmpeg(ctx, fallbackArgs...)
		if fallbackErr != nil {
			os.RemoveAll(dir)
			return "", nil, fmt.Errorf(
				"ffmpeg keyframes failed (scene and fallback): scene_err=%v scene_out=%s fallback_err=%v fallback_out=%s",
				primaryErr,
				primaryOut,
				fallbackErr,
				fallbackOut,
			)
		}
	}

	entries, err := filepath.Glob(filepath.Join(dir, "frame*.jpg"))
	if err != nil || len(entries) == 0 {
		// Fallback: extract a single mid-point frame if no scene changes detected
		mid := filepath.Join(dir, "frame0001.jpg")
		fb := exec.CommandContext(ctx, "ffmpeg", "-y",
			"-ss", "5", "-i", videoPath,
			"-vframes", "1",
			"-vf", "format=yuvj420p",
			"-pix_fmt", "yuvj420p",
			"-strict", "-1",
			"-q:v", "2",
			mid,
		)
		fb.CombinedOutput()
		entries, _ = filepath.Glob(filepath.Join(dir, "frame*.jpg"))
	}

	if len(entries) > maxFrames {
		entries = sampleEvenly(entries, maxFrames)
	}

	return dir, entries, nil
}

// ExtractKeyframesWithTimestamps uses a two-pass approach to extract frames with real timestamps.
// Pass 1 detects scene-change timestamps via ffmpeg showinfo stderr output.
// Pass 2 seek-extracts each frame individually.
// Falls back to evenly-spaced timestamps (1 per minute) if pass 1 yields no results.
func ExtractKeyframesWithTimestamps(ctx context.Context, videoPath string, maxFrames int) (string, []KeyframeInfo, error) {
	if maxFrames <= 0 {
		maxFrames = 30
	}

	dir, err := os.MkdirTemp("", "agentra-frames-*")
	if err != nil {
		return "", nil, err
	}

	timestamps := detectSceneTimestamps(ctx, videoPath, maxFrames)
	if len(timestamps) < 1 {
		timestamps = fallbackTimestamps(maxFrames)
	}

	frames := make([]KeyframeInfo, 0, len(timestamps))
	for i, ts := range timestamps {
		outPath := filepath.Join(dir, fmt.Sprintf("frame_%04d.jpg", i+1))
		cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
			"-ss", fmt.Sprintf("%.3f", ts),
			"-i", videoPath,
			"-frames:v", "1",
			"-vf", "format=yuvj420p",
			"-pix_fmt", "yuvj420p",
			"-strict", "-1",
			"-q:v", "2",
			outPath,
		)
		if _, err := cmd.CombinedOutput(); err == nil {
			if _, statErr := os.Stat(outPath); statErr == nil {
				frames = append(frames, KeyframeInfo{
					Path:         outPath,
					FrameNumber:  i + 1,
					TimestampSec: ts,
				})
			}
		}
	}

	return dir, frames, nil
}

var ptsTimeRe = regexp.MustCompile(`pts_time:(\d+(?:\.\d+)?)`)

func detectSceneTimestamps(ctx context.Context, videoPath string, maxFrames int) []float64 {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", videoPath,
		"-vf", "select='gt(scene,0.4)',showinfo",
		"-f", "null",
		"-",
	)
	out, _ := cmd.CombinedOutput()

	seen := make(map[float64]bool)
	timestamps := make([]float64, 0, maxFrames)
	for _, m := range ptsTimeRe.FindAllSubmatch(out, -1) {
		ts, err := strconv.ParseFloat(string(m[1]), 64)
		if err != nil || seen[ts] {
			continue
		}
		seen[ts] = true
		timestamps = append(timestamps, ts)
		if len(timestamps) >= maxFrames {
			break
		}
	}
	return timestamps
}

func fallbackTimestamps(maxFrames int) []float64 {
	ts := make([]float64, maxFrames)
	for i := range ts {
		ts[i] = float64(i * 60)
	}
	return ts
}

// ExtractThumbnail extracts a single JPEG frame at the given offset (seconds).
// The caller is responsible for removing the returned file.
func ExtractThumbnail(ctx context.Context, videoPath string, offsetSec int) (string, error) {
	outFile, err := os.CreateTemp("", "agentra-thumb-*.jpg")
	if err != nil {
		return "", err
	}
	out := outFile.Name()
	_ = outFile.Close()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-ss", fmt.Sprintf("%d", offsetSec),
		"-i", videoPath,
		"-vframes", "1",
		"-vf", "format=yuvj420p",
		"-pix_fmt", "yuvj420p",
		"-strict", "-1",
		"-q:v", "2",
		out,
	)
	if b, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(out)
		return "", fmt.Errorf("ffmpeg thumbnail: %w: %s", err, b)
	}
	if err := requireOutputFile(out, "ffmpeg thumbnail"); err != nil {
		_ = os.Remove(out)
		return "", err
	}
	return out, nil
}

func requireOutputFile(path string, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s output missing: %w", label, err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s output is empty", label)
	}
	return nil
}

func runFFmpeg(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	return cmd.CombinedOutput()
}

func GenerateNoiseVideo(
	ctx context.Context,
	outputPath string,
	width int,
	height int,
	fps int,
	durationSec int,
	noiseStrength int,
) error {
	if width < 64 {
		width = 540
	}
	if height < 64 {
		height = 960
	}
	if fps < 1 {
		fps = 24
	}
	if durationSec < 1 {
		durationSec = 30
	}
	if noiseStrength < 0 {
		noiseStrength = 0
	}
	if noiseStrength > 100 {
		noiseStrength = 100
	}

	internalWidth := max(96, width/3)
	internalHeight := max(96, height/3)
	grain := max(32, min(255, noiseStrength*3))

	// Use a true synthetic noise field instead of perturbing a flat black frame.
	// The lower-resolution source keeps the stub cheap, then nearest-neighbor scaling
	// preserves the chunky TV-static look at the requested output size.
	input := fmt.Sprintf(
		"nullsrc=s=%dx%d:r=%d,geq=lum='random(1)*%d':cb=128:cr=128,scale=%d:%d:flags=neighbor,format=yuv420p",
		internalWidth,
		internalHeight,
		fps,
		grain,
		width,
		height,
	)
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", input,
		"-t", strconv.Itoa(durationSec),
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-crf", "34",
		"-pix_fmt", "yuv420p",
		outputPath,
	)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg noise video: %w: %s", err, b)
	}
	return nil
}

func sampleEvenly(items []string, n int) []string {
	if len(items) <= n {
		return items
	}
	result := make([]string, n)
	step := float64(len(items)-1) / float64(n-1)
	for i := 0; i < n; i++ {
		result[i] = items[int(float64(i)*step+0.5)]
	}
	return result
}
