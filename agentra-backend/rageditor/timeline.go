package rageditor

import "fmt"

type TimelineOutput struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`
}

type TimelineClip struct {
	ID               string  `json:"id"`
	MediaID          string  `json:"media_id"`
	SourceStartSec   float64 `json:"source_start_sec"`
	SourceEndSec     float64 `json:"source_end_sec"`
	TimelineStartSec float64 `json:"timeline_start_sec"`
	TimelineEndSec   float64 `json:"timeline_end_sec"`
	Label            string  `json:"label,omitempty"`
}

type TimelineTrack struct {
	ID    string         `json:"id"`
	Kind  string         `json:"kind"` // "video" or "audio"
	Clips []TimelineClip `json:"clips"`
}

type Timeline struct {
	Format            string          `json:"format"` // e.g. "agentra.timeline.v1"
	ProjectID         string          `json:"project_id"`
	JobID             string          `json:"job_id"`
	VariantIndex      int             `json:"variant_index"`
	Output            TimelineOutput  `json:"output"`
	DurationTargetSec int             `json:"duration_target_sec"`
	Tracks            []TimelineTrack `json:"tracks"`
}

type TimelineValidationError struct {
	Message string `json:"message"`
}

func ValidateTimeline(t Timeline, state *SessionState) []TimelineValidationError {
	var errors []TimelineValidationError

	if len(t.Tracks) == 0 {
		errors = append(errors, TimelineValidationError{Message: "timeline must have at least one track"})
	}

	hasVideoTrack := false
	for _, track := range t.Tracks {
		if track.Kind == "video" {
			hasVideoTrack = true
		}
		if len(track.Clips) == 0 {
			errors = append(errors, TimelineValidationError{Message: "track " + track.ID + " has no clips"})
		}
		for _, clip := range track.Clips {
			if clip.SourceEndSec <= clip.SourceStartSec {
				errors = append(errors, TimelineValidationError{Message: "clip " + clip.ID + " has invalid source bounds"})
			}
			if clip.TimelineEndSec <= clip.TimelineStartSec {
				errors = append(errors, TimelineValidationError{Message: "clip " + clip.ID + " has invalid timeline bounds"})
			}
		}
	}

	if !hasVideoTrack {
		errors = append(errors, TimelineValidationError{Message: "timeline must have at least one video track"})
	}

	totalDuration := ComputeTimelineDurationSec(t)
	if totalDuration > float64(state.MaxDurationSec) {
		errors = append(errors, TimelineValidationError{Message: fmt.Sprintf("total duration %.2fs exceeds maximum bounds of %ds", totalDuration, state.MaxDurationSec)})
	}

	return errors
}

func ComputeTimelineDurationSec(t Timeline) float64 {
	var maxDuration float64
	for _, track := range t.Tracks {
		for _, clip := range track.Clips {
			if clip.TimelineEndSec > maxDuration {
				maxDuration = clip.TimelineEndSec
			}
		}
	}
	return maxDuration
}

func NormalizeTimeline(t Timeline, state *SessionState) Timeline {
	// Re-assign basic timeline fields to match current state
	t.Format = "agentra.timeline.v1"
	t.ProjectID = state.ProjectID
	t.JobID = state.JobID
	t.VariantIndex = state.VariantIndex
	if t.Output.Width < 64 {
		t.Output.Width = state.OutputWidth
	}
	if t.Output.Height < 64 {
		t.Output.Height = state.OutputHeight
	}
	if t.Output.FPS < 1 {
		t.Output.FPS = state.OutputFPS
	}
	if t.DurationTargetSec <= 0 {
		t.DurationTargetSec = state.MinDurationSec
	}
	return t
}

func AppendNextClip(state *SessionState, assetID string, sourceStartSec float64, sourceEndSec float64, label string) (Timeline, []TimelineValidationError) {
	timeline := state.GetTimeline()
	if timeline == nil {
		timeline = &Timeline{
			Format:       "agentra.timeline.v1",
			ProjectID:    state.ProjectID,
			JobID:        state.JobID,
			VariantIndex: state.VariantIndex,
			Output: TimelineOutput{
				Width:  state.OutputWidth,
				Height: state.OutputHeight,
				FPS:    state.OutputFPS,
			},
			DurationTargetSec: state.MinDurationSec,
			Tracks: []TimelineTrack{
				{
					ID:    "video_main",
					Kind:  "video",
					Clips: []TimelineClip{},
				},
			},
		}
	}

	if len(timeline.Tracks) == 0 {
		timeline.Tracks = []TimelineTrack{{ID: "video_main", Kind: "video", Clips: []TimelineClip{}}}
	}

	start := ComputeTimelineDurationSec(*timeline)
	duration := sourceEndSec - sourceStartSec
	clipIndex := len(timeline.Tracks[0].Clips) + 1
	clip := TimelineClip{
		ID:               fmt.Sprintf("clip_%03d", clipIndex),
		MediaID:          assetID,
		SourceStartSec:   sourceStartSec,
		SourceEndSec:     sourceEndSec,
		TimelineStartSec: start,
		TimelineEndSec:   start + duration,
		Label:            label,
	}
	timeline.Tracks[0].Clips = append(timeline.Tracks[0].Clips, clip)
	normalized := NormalizeTimeline(*timeline, state)
	errs := ValidateTimeline(normalized, state)
	return normalized, errs
}
