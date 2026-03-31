package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/adapter/llm"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
)

type EditorUsecase struct {
	editor    *postgres.EditorRepo
	providers *postgres.ProviderRepo
	llm       *llm.Client
}

func NewEditorUsecase(editor *postgres.EditorRepo, providers *postgres.ProviderRepo, llmClient *llm.Client) *EditorUsecase {
	return &EditorUsecase{editor: editor, providers: providers, llm: llmClient}
}

func (u *EditorUsecase) Get(ctx context.Context, orgID uuid.UUID) (domain.EditorConfig, error) {
	cfg, err := u.editor.Get(ctx, orgID)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return domain.EditorConfig{}, ErrNotFound.New("no editor config found")
		}
		return domain.EditorConfig{}, err
	}
	return cfg, nil
}

type EditorInput struct {
	ProviderID          uuid.UUID
	ModelName           string
	BasePrompt          string
	MaxDurationSec      int
	IsAutonomousEnabled bool
}

func (u *EditorUsecase) Set(ctx context.Context, orgID uuid.UUID, in EditorInput) (domain.EditorConfig, error) {
	existing, err := u.editor.Get(ctx, orgID)
	now := time.Now()

	id := uuid.New()
	createdAt := now
	resolvedProviderID := in.ProviderID
	resolvedModelName := strings.TrimSpace(in.ModelName)
	if err == nil {
		id = existing.ID
		createdAt = existing.CreatedAt
		if resolvedProviderID == uuid.Nil && existing.ProviderID.Valid {
			resolvedProviderID = existing.ProviderID.UUID
		}
		if resolvedModelName == "" {
			resolvedModelName = strings.TrimSpace(existing.ModelName)
		}
	} else if !errors.Is(err, postgres.ErrNotFound) {
		return domain.EditorConfig{}, err
	}

	if resolvedProviderID == uuid.Nil {
		return domain.EditorConfig{}, ErrBadRequest.New("provider_id is required")
	}
	if resolvedModelName == "" {
		return domain.EditorConfig{}, ErrBadRequest.New("model_name is required")
	}
	if in.MaxDurationSec < 1 {
		return domain.EditorConfig{}, ErrBadRequest.New("max_duration_sec must be at least 1")
	}

	provider, err := u.providers.GetByID(ctx, orgID, resolvedProviderID)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return domain.EditorConfig{}, ErrBadRequest.New("provider not found")
		}
		return domain.EditorConfig{}, err
	}
	if !provider.IsActive {
		return domain.EditorConfig{}, ErrForbidden.New("provider must be active")
	}
	if provider.ProviderType == domain.ProviderDeepgram {
		return domain.EditorConfig{}, ErrBadRequest.New("deepgram provider type is not supported for editor agent")
	}

	cfg := domain.EditorConfig{
		ID:                  id,
		OrganizationID:      orgID,
		ProviderID:          uuid.NullUUID{UUID: resolvedProviderID, Valid: true},
		ModelName:           resolvedModelName,
		BasePrompt:          in.BasePrompt,
		MaxDurationSec:      in.MaxDurationSec,
		IsAutonomousEnabled: in.IsAutonomousEnabled,
		CreatedAt:           createdAt,
		UpdatedAt:           now,
	}
	if err := u.editor.Upsert(ctx, cfg); err != nil {
		return domain.EditorConfig{}, err
	}
	return cfg, nil
}

type EditorTestEvent struct {
	Event        string         `json:"event"`
	Message      string         `json:"message"`
	TimestampUTC time.Time      `json:"timestamp_utc"`
	Payload      map[string]any `json:"payload,omitempty"`
}

func (u *EditorUsecase) StartAgentTest(ctx context.Context, orgID uuid.UUID) (<-chan EditorTestEvent, error) {
	cfg, err := u.editor.Get(ctx, orgID)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return nil, ErrNotFound.New("no editor config found")
		}
		return nil, err
	}
	if !cfg.ProviderID.Valid {
		return nil, ErrBadRequest.New("editor agent provider is not configured")
	}
	modelName := strings.TrimSpace(cfg.ModelName)
	if modelName == "" {
		return nil, ErrBadRequest.New("editor agent model_name is not configured")
	}

	provider, err := u.providers.GetByID(ctx, orgID, cfg.ProviderID.UUID)
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return nil, ErrBadRequest.New("editor agent provider does not exist")
		}
		return nil, err
	}
	if !provider.IsActive {
		return nil, ErrForbidden.New("editor agent provider must be active")
	}
	if provider.ProviderType == domain.ProviderDeepgram {
		return nil, ErrBadRequest.New("deepgram provider type is not supported for editor agent")
	}

	events := make(chan EditorTestEvent, 32)
	go u.runAgentTest(ctx, provider, cfg, events)
	return events, nil
}

func (u *EditorUsecase) runAgentTest(
	ctx context.Context,
	provider domain.ModelProvider,
	cfg domain.EditorConfig,
	events chan<- EditorTestEvent,
) {
	defer close(events)

	llmCfg := llm.Config{
		BaseURL:      provider.BaseURL,
		APIKey:       provider.APIKey.String,
		ModelName:    cfg.ModelName,
		ProviderType: provider.ProviderType,
	}

	emitEditorEvent(ctx, events, EditorTestEvent{
		Event:        "run.started",
		Message:      "Running editor agent test.",
		TimestampUTC: time.Now().UTC(),
		Payload: map[string]any{
			"provider_name": provider.Name,
			"provider_type": provider.ProviderType,
			"model_name":    cfg.ModelName,
		},
	})

	type promptSample struct {
		id        string
		prompt    string
		expectAny []string
	}
	samples := []promptSample{
		{
			id:        "summary_check",
			prompt:    "Summarize this clip in one sentence: A daytime football match with cheering crowd and two goals.",
			expectAny: []string{"football", "goal", "crowd"},
		},
		{
			id:        "edit_intent_check",
			prompt:    "Write one sentence describing an edit intent for a highlight reel from an interview and b-roll footage.",
			expectAny: []string{"highlight", "interview", "b-roll"},
		},
	}

	successes := 0
	failures := 0
	warnings := 0
	systemPrompt := strings.TrimSpace(cfg.BasePrompt)
	if systemPrompt == "" {
		systemPrompt = "You are a precise video editor assistant. Reply in concise plain text."
	}

	for i, sample := range samples {
		if ctx.Err() != nil {
			emitEditorEvent(ctx, events, EditorTestEvent{
				Event:        "run.cancelled",
				Message:      "Editor test stream disconnected.",
				TimestampUTC: time.Now().UTC(),
			})
			return
		}

		emitEditorEvent(ctx, events, EditorTestEvent{
			Event:        "sample.started",
			Message:      fmt.Sprintf("Running sample %d/%d.", i+1, len(samples)),
			TimestampUTC: time.Now().UTC(),
			Payload: map[string]any{
				"sample_id": sample.id,
				"prompt":    sample.prompt,
			},
		})

		reply, err := u.llm.Chat(ctx, llmCfg, systemPrompt, sample.prompt)
		if err != nil {
			failures++
			emitEditorEvent(ctx, events, EditorTestEvent{
				Event:        "sample.failed",
				Message:      fmt.Sprintf("Editor model request failed: %v", err),
				TimestampUTC: time.Now().UTC(),
				Payload: map[string]any{
					"sample_id": sample.id,
				},
			})
			continue
		}

		output := strings.TrimSpace(reply.Content)
		if output == "" {
			failures++
			emitEditorEvent(ctx, events, EditorTestEvent{
				Event:        "sample.failed",
				Message:      "Editor model returned an empty response.",
				TimestampUTC: time.Now().UTC(),
				Payload: map[string]any{
					"sample_id": sample.id,
				},
			})
			continue
		}

		expectationOK := textContainsAny(output, sample.expectAny)
		if !expectationOK {
			warnings++
		}
		successes++
		emitEditorEvent(ctx, events, EditorTestEvent{
			Event:        "sample.completed",
			Message:      "Editor sample completed.",
			TimestampUTC: time.Now().UTC(),
			Payload: map[string]any{
				"sample_id":      sample.id,
				"expectation_ok": expectationOK,
				"expected_any":   sample.expectAny,
				"output_preview": compactPreview(output, 220),
			},
		})
	}

	finalEvent := "run.completed"
	if successes == 0 {
		finalEvent = "run.failed"
	}
	emitEditorEvent(ctx, events, EditorTestEvent{
		Event:        finalEvent,
		Message:      "Editor test run finished.",
		TimestampUTC: time.Now().UTC(),
		Payload: map[string]any{
			"successes": successes,
			"failures":  failures,
			"warnings":  warnings,
		},
	})
}

func emitEditorEvent(ctx context.Context, events chan<- EditorTestEvent, event EditorTestEvent) {
	select {
	case <-ctx.Done():
		return
	case events <- event:
	}
}
