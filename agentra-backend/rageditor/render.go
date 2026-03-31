package rageditor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/adapter/ffmpeg"
	"github.com/reaganiwadha/agentra/internal/domain"
)

func RenderTimeline(ctx context.Context, timeline Timeline, outputPath string, localMediaPaths map[string]string) (Result, []domain.HighlightJobTrace, error) {
	jobID, _ := uuid.Parse(timeline.JobID)

	width := timeline.Output.Width
	if width < 64 { width = 1080 }
	height := timeline.Output.Height
	if height < 64 { height = 1920 }
	fps := timeline.Output.FPS
	if fps < 1 { fps = 30 }
	durationSec := int(ComputeTimelineDurationSec(timeline))
	if durationSec < 1 { durationSec = 1 }

	if len(timeline.Tracks) == 0 || len(timeline.Tracks[0].Clips) == 0 {
		return RenderSyntheticDebug(ctx, timeline, outputPath, localMediaPaths)
	}

	traces := []domain.HighlightJobTrace{
		newTrace(jobID, "render.exec.started", "Executing real ffmpeg renderer.", map[string]any{
			"renderer": "ffmpeg.concat",
			"arguments": map[string]any{
				"duration_sec": durationSec,
				"width":        width,
				"height":       height,
				"fps":          fps,
				"output_path":  outputPath,
				"clips":        len(timeline.Tracks[0].Clips),
			},
		}),
	}

	clipsDir, err := os.MkdirTemp("", "agentra-concat-*")
	if err != nil {
		traces = append(traces, newTrace(jobID, "render.exec.failed", "Failed to create temp dir for concat.", map[string]any{"error": err.Error()}))
		return Result{}, traces, err
	}
	defer os.RemoveAll(clipsDir)

	var concatList strings.Builder
	for i, clip := range timeline.Tracks[0].Clips {
		sourcePath, ok := localMediaPaths[clip.MediaID]
		if !ok || sourcePath == "" {
			err = fmt.Errorf("missing local media path for clip %s (media_id: %s)", clip.ID, clip.MediaID)
			traces = append(traces, newTrace(jobID, "render.exec.failed", "Missing local media file.", map[string]any{"error": err.Error()}))
			return Result{}, traces, err
		}

		clipOutPath := filepath.Join(clipsDir, fmt.Sprintf("clip_%03d.mp4", i))
		
		clipDur := clip.SourceEndSec - clip.SourceStartSec
		if clipDur <= 0 {
			clipDur = 1.0
		}

		// scale and pad to match target aspect ratio and fps
		// use setsar to ensure aspect ratio is correct
		filterGraph := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=%d", width, height, width, height, fps)

		cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
			"-ss", fmt.Sprintf("%.3f", clip.SourceStartSec),
			"-t", fmt.Sprintf("%.3f", clipDur),
			"-i", sourcePath,
			"-map", "0:v",
			"-map", "0:a?",
			"-vf", filterGraph,
			"-pix_fmt", "yuv420p",
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-crf", "23",
			"-c:a", "aac",
			"-b:a", "128k",
			"-ar", "44100",
			"-ac", "2",
			clipOutPath,
		)

		if out, err := cmd.CombinedOutput(); err != nil {
			traces = append(traces, newTrace(jobID, "render.exec.failed", fmt.Sprintf("Failed to process clip %d.", i), map[string]any{"error": err.Error(), "ffmpeg_out": string(out)}))
			return Result{}, traces, fmt.Errorf("ffmpeg process clip: %w\n%s", err, out)
		}

		concatList.WriteString(fmt.Sprintf("file '%s'\n", filepath.ToSlash(clipOutPath)))
	}

	concatFilePath := filepath.Join(clipsDir, "concat.txt")
	if err := os.WriteFile(concatFilePath, []byte(concatList.String()), 0644); err != nil {
		traces = append(traces, newTrace(jobID, "render.exec.failed", "Failed to write concat list.", map[string]any{"error": err.Error()}))
		return Result{}, traces, err
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFilePath,
		"-c", "copy",
		outputPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		traces = append(traces, newTrace(jobID, "render.exec.failed", "Failed to concat clips.", map[string]any{"error": err.Error(), "ffmpeg_out": string(out)}))
		return Result{}, traces, fmt.Errorf("ffmpeg concat: %w\n%s", err, out)
	}

	result := Result{
		OutputPath:  outputPath,
		DurationSec: float64(durationSec),
		Renderer:    "ffmpeg.concat",
	}
	traces = append(traces, newTrace(jobID, "render.exec.completed", "Real ffmpeg renderer completed.", map[string]any{
		"renderer":       result.Renderer,
		"output_path":    result.OutputPath,
		"duration_sec":   result.DurationSec,
		"width":          width,
		"height":         height,
		"fps":            fps,
		"ffmpeg_out":     string(out),
	}))
	return result, traces, nil
}

func RenderSyntheticDebug(ctx context.Context, timeline Timeline, outputPath string, localMediaPaths map[string]string) (Result, []domain.HighlightJobTrace, error) {
	jobID, _ := uuid.Parse(timeline.JobID)
	durationSec := int(ComputeTimelineDurationSec(timeline))
	if durationSec < 1 {
		durationSec = 1
	}
	
	width := timeline.Output.Width
	if width < 64 { width = 540 }
	height := timeline.Output.Height
	if height < 64 { height = 960 }
	fps := timeline.Output.FPS
	if fps < 1 { fps = 24 }
	noiseStrength := 0.18

	traces := []domain.HighlightJobTrace{
		newTrace(jobID, "tool.exec.started", "Executing synthetic renderer tool via ffmpeg based on timeline.", map[string]any{
			"renderer": RendererSyntheticNoiseFFM,
			"arguments": map[string]any{
				"duration_sec":   durationSec,
				"width":          width,
				"height":         height,
				"fps":            fps,
				"noise_strength": noiseStrength,
				"output_path":    outputPath,
			},
		}),
	}

	if err := ffmpeg.GenerateNoiseVideo(
		ctx,
		outputPath,
		width,
		height,
		fps,
		durationSec,
		int(noiseStrength * 100),
	); err != nil {
		traces = append(traces, newTrace(jobID, "tool.exec.failed", "Synthetic renderer tool failed.", map[string]any{
			"renderer": RendererSyntheticNoiseFFM,
			"error":    err.Error(),
		}))
		return Result{}, traces, fmt.Errorf("ffmpeg synthetic render: %w", err)
	}

	result := Result{
		OutputPath:  outputPath,
		DurationSec: float64(durationSec),
		Renderer:    RendererSyntheticNoiseFFM,
	}
	traces = append(traces, newTrace(jobID, "tool.exec.completed", "Synthetic renderer tool completed.", map[string]any{
		"renderer":       result.Renderer,
		"output_path":    result.OutputPath,
		"duration_sec":   result.DurationSec,
		"width":          width,
		"height":         height,
		"fps":            fps,
		"noise_strength": noiseStrength,
	}))
	return result, traces, nil
}
