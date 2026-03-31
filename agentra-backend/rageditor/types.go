package rageditor

import (
	"context"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/tmc/langchaingo/llms"
)

const (
	ToolGenerateNoiseVideo    = "generate_noise_video"
	RendererSyntheticNoiseFFM = "ffmpeg.synthetic_noise"
)

type ChatClient interface {
	Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

type ModelConfig struct {
	BaseURL      string
	APIKey       string
	ModelName    string
	ProviderType domain.ProviderType
}

type PlannedToolCall struct {
	Thinking      string
	Name          string
	DurationSec   int
	Width         int
	Height        int
	FPS           int
	NoiseStrength float64
}

type PlanResult struct {
	ToolCall PlannedToolCall
}

type ToolHandler func(ctx context.Context, argumentsJSON string) (any, error)

type RunnableTool struct {
	Definition llms.Tool
	Handle     ToolHandler
}

type ToolInvocationRequest struct {
	JobID         uuid.UUID
	ProviderName  string
	Model         ModelConfig
	SystemPrompt  string
	UserPrompt    string
	Tools         []RunnableTool
	ToolChoice    any
	MaxTokens     int
	MaxIterations int
	PromptVisible bool
}

type ToolInvocationResult struct {
	ToolCalls         []llms.ToolCall
	ToolOutputs       [][]byte
	AssistantContent  string
	ReasoningContent  string
	TerminationReason string
	IterationsUsed    int
}

type RunRequest struct {
	ProjectID      string
	JobID          string
	VariantIndex   int
	VariantCount   int
	Prompt         string
	MinDurationSec int
	MaxDurationSec int
	ProviderName   string
	Model          ModelConfig
	BasePrompt     string
}

type RunResult struct {
	Timeline          *Timeline
	Render            *Result
	RenderRequested   bool
	TerminationReason string
}

type LoopDeps struct {
	ListProjectAssets  func(ctx context.Context) ([]ProjectAssetSummary, error)
	ListProjectMoments func(ctx context.Context, page int, pageSize int) ([]ProjectMomentResult, int, error)
}

type Request struct {
	ProjectID      string
	JobID          uuid.UUID
	VariantIndex   int
	VariantCount   int
	Prompt         string
	MinDurationSec int
	MaxDurationSec int
	ToolCall       PlannedToolCall
	OutputPath     string
}

type Result struct {
	OutputPath  string
	DurationSec float64
	Renderer    string
}

type ProjectAssetSummary struct {
	MediaID     string   `json:"media_id"`
	Filename    string   `json:"filename"`
	DurationSec *float64 `json:"duration_sec,omitempty"`
	Summary     string   `json:"summary"`
}
