package rageditor

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/tmc/langchaingo/llms"
)

func BuildPhaseOneTools(deps LoopDeps, state *SessionState) []RunnableTool {
	return []RunnableTool{
		{
			Definition: llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        "write_notebook",
					Description: "Write or replace the in-memory working notebook. Use this to record your plan, chosen assets, timeline strategy, open questions, and validation checklist before or during editing.",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"content": map[string]any{
								"type":        "string",
								"description": "Plain text notebook content containing your current plan and working notes.",
							},
						},
						"required": []string{"content"},
					},
				},
			},
			Handle: func(ctx context.Context, argumentsJSON string) (any, error) {
				var args struct {
					Content string `json:"content"`
				}
				if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
					return nil, err
				}
				state.SetNotebook(args.Content)
				return map[string]any{
					"status":  "saved",
					"content": state.GetNotebook(),
					"length":  len(state.GetNotebook()),
				}, nil
			},
		},
		{
			Definition: llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        "read_notebook",
					Description: "Read the current in-memory notebook. Use this to recall your plan, selected moments, and remaining steps before editing or rendering.",
					Parameters: map[string]any{
						"type":       "object",
						"properties": map[string]any{},
					},
				},
			},
			Handle: func(ctx context.Context, argumentsJSON string) (any, error) {
				return map[string]any{
					"content": state.GetNotebook(),
					"length":  len(state.GetNotebook()),
				}, nil
			},
		},
		{
			Definition: llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        "list_assets",
					Description: "List all available project assets. An asset is a whole media file, with a short summary of its overall content. Use this for file-level selection and context.",
					Parameters: map[string]any{
						"type":       "object",
						"properties": map[string]any{},
					},
				},
			},
			Handle: func(ctx context.Context, argumentsJSON string) (any, error) {
				return map[string]any{
					"results": state.AvailableAssets,
					"count":   len(state.AvailableAssets),
				}, nil
			},
		},
		{
			Definition: llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        "list_moments",
					Description: "List timestamped moments inside project assets with pagination. A moment is a timed window inside an asset, such as a transcript segment or timed visual event. Use this first to browse candidate clips before building the timeline.",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"page": map[string]any{
								"type":        "integer",
								"description": "1-based page number.",
							},
							"page_size": map[string]any{
								"type":        "integer",
								"description": "Number of results per page (max 20).",
							},
						},
						"required": []string{},
					},
				},
			},
			Handle: func(ctx context.Context, argumentsJSON string) (any, error) {
				var args struct {
					Page     int `json:"page"`
					PageSize int `json:"page_size"`
				}
				if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
					return nil, err
				}
				if args.Page <= 0 {
					args.Page = 1
				}
				if args.PageSize <= 0 || args.PageSize > 20 {
					args.PageSize = 10
				}
				results, total, err := deps.ListProjectMoments(ctx, args.Page, args.PageSize)
				if err != nil {
					return map[string]any{"error": err.Error()}, nil
				}
				for i := range results {
					results[i].Keywords = extractTopKeywords(results[i].MatchedText+" "+results[i].ContextText, 5)
				}
				state.LastMomentResults = results
				return map[string]any{
					"results":   results,
					"page":      args.Page,
					"page_size": args.PageSize,
					"total":     total,
					"has_more":  args.Page*args.PageSize < total,
				}, nil
			},
		},
		{
			Definition: llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        "replace_timeline",
					Description: "Replace the entire draft timeline with a new one. Call this after you have a concrete plan in the notebook and enough search results to assemble a complete highlight edit.",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"timeline": map[string]any{
								"type":        "object",
								"description": "The new timeline JSON object. Must match the agentra.timeline.v1 format, containing output settings, a duration target, and tracks with clips.",
							},
						},
						"required": []string{"timeline"},
					},
				},
			},
			Handle: func(ctx context.Context, argumentsJSON string) (any, error) {
				var args struct {
					Timeline Timeline `json:"timeline"`
				}
				if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
					return map[string]any{"error": "Failed to parse timeline JSON: " + err.Error()}, nil
				}
				t := NormalizeTimeline(args.Timeline, state)
				errs := ValidateTimeline(t, state)
				if len(errs) > 0 {
					return map[string]any{"errors": errs}, nil
				}
				state.SetTimeline(t)
				return map[string]any{"status": "success", "timeline": t}, nil
			},
		},
		{
			Definition: llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        "current_timeline_state",
					Description: "Retrieve the current timeline state, total duration, clip count, and validation errors. Use this after add_next_clip and before rendering.",
					Parameters: map[string]any{
						"type":       "object",
						"properties": map[string]any{},
					},
				},
			},
			Handle: func(ctx context.Context, argumentsJSON string) (any, error) {
				t := state.GetTimeline()
				if t == nil {
					return map[string]any{"status": "no_timeline"}, nil
				}
				errs := ValidateTimeline(*t, state)
				return map[string]any{
					"status":             "exists",
					"timeline":           t,
					"errors":             errs,
					"total_duration_sec": ComputeTimelineDurationSec(*t),
					"clip_count":         countTimelineClips(*t),
				}, nil
			},
		},
		{
			Definition: llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        "add_next_clip",
					Description: "Append a clip to the end of the current timeline using a timed moment inside a specific asset. This is the preferred way to build the timeline incrementally.",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"asset_id": map[string]any{
								"type":        "string",
								"description": "The asset id for the source media file.",
							},
							"start_time": map[string]any{
								"type":        "number",
								"description": "Source start time in seconds inside the asset.",
							},
							"end_time": map[string]any{
								"type":        "number",
								"description": "Source end time in seconds inside the asset.",
							},
							"label": map[string]any{
								"type":        "string",
								"description": "Optional short label for why this clip is included.",
							},
						},
						"required": []string{"asset_id", "start_time", "end_time"},
					},
				},
			},
			Handle: func(ctx context.Context, argumentsJSON string) (any, error) {
				var args struct {
					AssetID   string  `json:"asset_id"`
					StartTime float64 `json:"start_time"`
					EndTime   float64 `json:"end_time"`
					Label     string  `json:"label"`
				}
				if err := json.Unmarshal([]byte(argumentsJSON), &args); err != nil {
					return nil, err
				}
				if !state.HasAsset(args.AssetID) {
					return map[string]any{"error": "unknown asset_id"}, nil
				}
				timeline, errs := AppendNextClip(state, args.AssetID, args.StartTime, args.EndTime, strings.TrimSpace(args.Label))
				if len(errs) > 0 {
					return map[string]any{
						"status":             "rejected",
						"errors":             errs,
						"proposed_timeline":  timeline,
						"total_duration_sec": ComputeTimelineDurationSec(timeline),
					}, nil
				}
				state.SetTimeline(timeline)
				return map[string]any{
					"status":             "success",
					"timeline":           timeline,
					"total_duration_sec": ComputeTimelineDurationSec(timeline),
					"clip_count":         countTimelineClips(timeline),
				}, nil
			},
		},
		{
			Definition: llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        "render_timeline",
					Description: "Request final render from the current validated timeline. Call this ONLY when you are finished editing and the timeline is valid.",
					Parameters: map[string]any{
						"type":       "object",
						"properties": map[string]any{},
					},
				},
			},
			Handle: func(ctx context.Context, argumentsJSON string) (any, error) {
				if !state.HasValidTimeline() {
					return map[string]any{"error": "Timeline is invalid or missing. Fix errors before rendering."}, nil
				}
				t := state.GetTimeline()
				totalDur := ComputeTimelineDurationSec(*t)
				if totalDur < float64(state.MinDurationSec) {
					return map[string]any{"error": fmt.Sprintf("Timeline is too short (%.2fs). Add more clips to reach the minimum duration of %ds.", totalDur, state.MinDurationSec)}, nil
				}
				state.RenderRequested = true
				return map[string]any{"status": "render_requested"}, nil
			},
		},
	}
}

func countTimelineClips(t Timeline) int {
	total := 0
	for _, track := range t.Tracks {
		total += len(track.Clips)
	}
	return total
}

func extractTopKeywords(text string, limit int) []string {
	if limit <= 0 {
		limit = 5
	}
	stop := map[string]struct{}{
		"the": {}, "and": {}, "with": {}, "from": {}, "that": {}, "this": {}, "into": {}, "inside": {},
		"there": {}, "their": {}, "about": {}, "have": {}, "your": {}, "will": {}, "using": {}, "through": {},
		"video": {}, "asset": {}, "moment": {}, "presentation": {}, "project": {}, "then": {},
	}
	parts := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	counts := make(map[string]int)
	for _, part := range parts {
		if len(part) < 4 {
			continue
		}
		if _, ok := stop[part]; ok {
			continue
		}
		counts[part]++
	}
	type pair struct {
		word  string
		count int
	}
	items := make([]pair, 0, len(counts))
	for word, count := range counts {
		items = append(items, pair{word: word, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].word < items[j].word
		}
		return items[i].count > items[j].count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.word)
	}
	return out
}
