package rageditor

type ProjectMomentResult struct {
	MediaID     string   `json:"media_id"`
	Filename    string   `json:"filename"`
	StartSec    float64  `json:"start_sec"`
	EndSec      float64  `json:"end_sec"`
	Score       float64  `json:"score"`
	MatchedText string   `json:"matched_text"`
	ContextText string   `json:"context_text"`
	Keywords    []string `json:"keywords,omitempty"`
}

type SessionState struct {
	ProjectID         string
	JobID             string
	VariantIndex      int
	VariantCount      int
	MinDurationSec    int
	MaxDurationSec    int
	OutputWidth       int
	OutputHeight      int
	OutputFPS         int
	AvailableAssets   []ProjectAssetSummary
	Notebook          string
	CurrentTimeline   *Timeline
	LastMomentResults []ProjectMomentResult
	RenderRequested   bool
}

func NewSessionState(req RunRequest) *SessionState {
	return &SessionState{
		ProjectID:      req.ProjectID,
		JobID:          req.JobID,
		VariantIndex:   req.VariantIndex,
		VariantCount:   req.VariantCount,
		MinDurationSec: req.MinDurationSec,
		MaxDurationSec: req.MaxDurationSec,
		OutputWidth:    1080,
		OutputHeight:   1920,
		OutputFPS:      30,
	}
}

func (s *SessionState) SetTimeline(t Timeline) {
	s.CurrentTimeline = &t
}

func (s *SessionState) GetTimeline() *Timeline {
	return s.CurrentTimeline
}

func (s *SessionState) SetNotebook(content string) {
	s.Notebook = content
}

func (s *SessionState) GetNotebook() string {
	return s.Notebook
}

func (s *SessionState) HasValidTimeline() bool {
	if s.CurrentTimeline == nil {
		return false
	}
	errs := ValidateTimeline(*s.CurrentTimeline, s)
	return len(errs) == 0
}

func (s *SessionState) HasAsset(assetID string) bool {
	for _, asset := range s.AvailableAssets {
		if asset.MediaID == assetID {
			return true
		}
	}
	return false
}
