package rageditor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/domain"
)

func RunTimelineLoop(
	ctx context.Context,
	req RunRequest,
	deps LoopDeps,
) (RunResult, []domain.HighlightJobTrace, error) {
	jobUUID, err := uuid.Parse(req.JobID)
	if err != nil {
		return RunResult{}, nil, fmt.Errorf("invalid job id: %w", err)
	}
	projectUUID, err := uuid.Parse(req.ProjectID)
	if err != nil {
		return RunResult{}, nil, fmt.Errorf("invalid project id: %w", err)
	}

	state := NewSessionState(req)

	traces := []domain.HighlightJobTrace{
		newTrace(jobUUID, "loop.started", "Starting timeline agent loop.", map[string]any{
			"project_id":       projectUUID.String(),
			"job_id":           jobUUID.String(),
			"min_duration_sec": req.MinDurationSec,
			"max_duration_sec": req.MaxDurationSec,
		}),
	}

	if strings.TrimSpace(req.Model.ModelName) == "" {
		traces = append(traces, newTrace(jobUUID, "llm.skipped", "Editor model is unavailable; aborting loop.", map[string]any{}))
		return RunResult{TerminationReason: "no_model"}, traces, nil
	}

	assetSummaries, assetsErr := deps.ListProjectAssets(ctx)
	if assetsErr != nil {
		traces = append(traces, newTrace(jobUUID, "assets.unavailable", "Could not load project asset summaries for editor loop.", map[string]any{
			"error": assetsErr.Error(),
		}))
	} else {
		state.AvailableAssets = assetSummaries
		traces = append(traces, newTrace(jobUUID, "assets.available", "Loaded project asset summaries for editor loop.", map[string]any{
			"asset_count": len(assetSummaries),
			"assets":      assetSummaries,
		}))
	}

	systemPrompt := "You are a professional video highlight editor. Your goal is to construct a timeline using the available tools.\n" +
		"Work in four stages: plan, retrieve, assemble, render.\n" +
		"Definitions:\n" +
		"- An asset is a whole media file.\n" +
		"- A moment is a timed window inside an asset.\n" +
		"1. Start by listing moments with list_moments. Use pagination to browse the available timed material. Use list_assets if you need broader file-level context.\n" +
		"2. Write a concrete working plan into write_notebook before serious editing. The notebook should capture the concept, selected assets or moment types, rough structure, and remaining steps.\n" +
		"3. Use list_moments to gather timestamped material. Read and update the notebook as your plan changes.\n" +
		"4. Build the timeline incrementally with add_next_clip. Only use replace_timeline if you truly need to rewrite the entire timeline.\n" +
		"5. Use current_timeline_state to inspect and repair validation errors until the timeline is valid.\n" +
		"6. Once the timeline is valid and complete, call render_timeline to finish the job.\n" +
		"7. Do not invent clips, timestamps, media IDs, or notebook facts. Use only tool outputs."

	userPrompt := fmt.Sprintf(
		"Assemble a highlight video.\nDuration must be between %d and %d seconds.\nTarget output is vertical (1080x1920 at 30fps).\nProject base prompt: %s\nRun prompt: %s\nAvailable assets:\n%s\nYour preferred flow is: list_moments -> write_notebook -> add_next_clip repeatedly -> current_timeline_state -> render_timeline. list_moments does not require a search query; browse pages until you find the right moments. You must end with a valid timeline and call render_timeline.",
		req.MinDurationSec,
		req.MaxDurationSec,
		req.BasePrompt,
		req.Prompt,
		formatAssetInventory(state.AvailableAssets),
	)

	invocation, invokeTraces, err := InvokeToolLoop(ctx, ToolInvocationRequest{
		JobID:         jobUUID,
		ProviderName:  req.ProviderName,
		Model:         req.Model,
		SystemPrompt:  systemPrompt,
		UserPrompt:    userPrompt,
		Tools:         BuildPhaseOneTools(deps, state),
		ToolChoice:    "auto",
		MaxTokens:     2048, // Higher for timeline generation
		MaxIterations: 30,
		PromptVisible: true,
	})
	traces = append(traces, invokeTraces...)

	if err != nil {
		traces = append(traces, newTrace(jobUUID, "llm.failed", "Tool loop failed.", map[string]any{
			"error": err.Error(),
		}))
		return RunResult{TerminationReason: "error"}, traces, err
	}

	terminationReason := invocation.TerminationReason
	if terminationReason == "" {
		terminationReason = "unknown"
	}
	if state.RenderRequested {
		terminationReason = "render_requested"
	} else if terminationReason == "stopped_tool_calling" && state.HasValidTimeline() {
		state.RenderRequested = true
		terminationReason = "implicit_render_requested"
	} else if terminationReason == "stopped_tool_calling" && !state.HasValidTimeline() {
		terminationReason = "stopped_without_valid_timeline"
	}

	traces = append(traces, newTrace(jobUUID, "loop.finished", "Timeline agent loop finished.", map[string]any{
		"termination_reason": terminationReason,
		"render_requested":   state.RenderRequested,
		"has_valid_timeline": state.HasValidTimeline(),
	}))

	var renderResult *Result
	if state.RenderRequested && state.HasValidTimeline() {
		// In phase 1, we still fallback to synthetic noise rendering here
		// but using the timeline's target duration
		dur := ComputeTimelineDurationSec(*state.CurrentTimeline)
		renderResult = &Result{
			OutputPath:  "", // This will be set by the caller
			DurationSec: dur,
			Renderer:    RendererSyntheticNoiseFFM,
		}
	}

	if state.CurrentTimeline != nil {
		timelineJSON, _ := json.Marshal(state.CurrentTimeline)
		traces = append(traces, newTrace(jobUUID, "timeline.final", "Final timeline from editor.", map[string]any{
			"timeline": json.RawMessage(timelineJSON),
		}))
	}
	if strings.TrimSpace(state.GetNotebook()) != "" {
		traces = append(traces, newTrace(jobUUID, "notebook.final", "Final notebook from editor.", map[string]any{
			"content": state.GetNotebook(),
		}))
	}

	return RunResult{
		Timeline:          state.CurrentTimeline,
		RenderRequested:   state.RenderRequested,
		TerminationReason: terminationReason,
		Render:            renderResult,
	}, traces, nil
}

func formatAssetInventory(items []ProjectAssetSummary) string {
	if len(items) == 0 {
		return "- no assets available"
	}
	lines := make([]string, 0, len(items))
	for i, item := range items {
		duration := ""
		if item.DurationSec != nil {
			duration = fmt.Sprintf(" (%.0fs)", *item.DurationSec)
		}
		lines = append(lines, fmt.Sprintf("%d. %s%s: %s", i+1, item.Filename, duration, truncateForPrompt(item.Summary, 220)))
	}
	return strings.Join(lines, "\n")
}

func truncateForPrompt(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return strings.TrimSpace(text[:maxLen-3]) + "..."
}
