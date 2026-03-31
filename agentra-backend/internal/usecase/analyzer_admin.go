package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/adapter/llm"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
)

type AnalyzerAdminUsecase struct {
	analyzers *postgres.AnalyzerRepo
	providers *postgres.ProviderRepo
	llm       *llm.Client
}

func NewAnalyzerAdminUsecase(analyzers *postgres.AnalyzerRepo, providers *postgres.ProviderRepo, llmClient *llm.Client) *AnalyzerAdminUsecase {
	return &AnalyzerAdminUsecase{analyzers: analyzers, providers: providers, llm: llmClient}
}

func (u *AnalyzerAdminUsecase) List(ctx context.Context, orgID uuid.UUID) ([]domain.Analyzer, error) {
	items, err := u.analyzers.List(ctx, orgID)
	if err != nil {
		return nil, normalizeRepoErr(err)
	}
	return items, nil
}

type AnalyzerInput struct {
	Name         string
	AnalyzerType domain.AnalyzerType
	ProviderID   uuid.UUID
	ModelName    string
	ConfigJSON   string
	IsEnabled    bool
}

func (u *AnalyzerAdminUsecase) Create(ctx context.Context, orgID uuid.UUID, in AnalyzerInput) (domain.Analyzer, error) {
	provider, err := u.getActiveProvider(ctx, orgID, in.ProviderID)
	if err != nil {
		return domain.Analyzer{}, err
	}
	in, err = normalizeAnalyzerInput(provider, in)
	if err != nil {
		return domain.Analyzer{}, err
	}

	cfgRaw, err := normalizeConfigJSON(in.ConfigJSON)
	if err != nil {
		return domain.Analyzer{}, ErrForbidden.New("config_json must be valid JSON")
	}

	now := time.Now()
	item := domain.Analyzer{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           strings.TrimSpace(in.Name),
		AnalyzerType:   in.AnalyzerType,
		ProviderID:     in.ProviderID,
		ModelName:      strings.TrimSpace(in.ModelName),
		ConfigJSON:     cfgRaw,
		IsEnabled:      in.IsEnabled,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := u.analyzers.Create(ctx, item); err != nil {
		return domain.Analyzer{}, normalizeRepoErr(err)
	}
	return item, nil
}

func (u *AnalyzerAdminUsecase) Update(ctx context.Context, orgID, id uuid.UUID, in AnalyzerInput) (domain.Analyzer, error) {
	provider, err := u.getActiveProvider(ctx, orgID, in.ProviderID)
	if err != nil {
		return domain.Analyzer{}, err
	}
	in, err = normalizeAnalyzerInput(provider, in)
	if err != nil {
		return domain.Analyzer{}, err
	}

	cfgRaw, err := normalizeConfigJSON(in.ConfigJSON)
	if err != nil {
		return domain.Analyzer{}, ErrForbidden.New("config_json must be valid JSON")
	}

	existing, err := u.analyzers.GetByID(ctx, orgID, id)
	if err != nil {
		return domain.Analyzer{}, normalizeRepoErr(err)
	}

	existing.Name = strings.TrimSpace(in.Name)
	existing.AnalyzerType = in.AnalyzerType
	existing.ProviderID = in.ProviderID
	existing.ModelName = strings.TrimSpace(in.ModelName)
	existing.ConfigJSON = cfgRaw
	existing.IsEnabled = in.IsEnabled
	existing.UpdatedAt = time.Now()

	if err := u.analyzers.Update(ctx, existing); err != nil {
		return domain.Analyzer{}, normalizeRepoErr(err)
	}
	return existing, nil
}

func (u *AnalyzerAdminUsecase) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return normalizeRepoErr(u.analyzers.Delete(ctx, orgID, id))
}

func (u *AnalyzerAdminUsecase) Disable(ctx context.Context, orgID, id uuid.UUID) error {
	return normalizeRepoErr(u.analyzers.Disable(ctx, orgID, id))
}

func (u *AnalyzerAdminUsecase) ListEnabledRuntime(ctx context.Context, orgID uuid.UUID) ([]domain.EnabledAnalyzer, error) {
	items, err := u.analyzers.ListEnabledWithProviders(ctx, orgID)
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (u *AnalyzerAdminUsecase) getActiveProvider(ctx context.Context, orgID, providerID uuid.UUID) (domain.ModelProvider, error) {
	p, err := u.providers.GetByID(ctx, orgID, providerID)
	if err != nil {
		return domain.ModelProvider{}, normalizeRepoErr(err)
	}
	if !p.IsActive {
		return domain.ModelProvider{}, ErrForbidden.New("provider must be active")
	}
	return p, nil
}

func normalizeAnalyzerInput(provider domain.ModelProvider, in AnalyzerInput) (AnalyzerInput, error) {
	name := strings.TrimSpace(in.Name)
	modelName := strings.TrimSpace(in.ModelName)
	analyzerType := in.AnalyzerType

	if provider.ProviderType == domain.ProviderDeepgram {
		analyzerType = domain.AnalyzerTranscription
		if modelName == "" {
			modelName = "nova-3"
		}
	}

	if analyzerType == "" {
		return AnalyzerInput{}, ErrForbidden.New("analyzer_type is required")
	}

	if modelName == "" {
		return AnalyzerInput{}, ErrForbidden.New("model_name is required for this provider")
	}

	if name == "" {
		name = defaultAnalyzerName(provider.Name, analyzerType)
	}

	in.Name = name
	in.ModelName = modelName
	in.AnalyzerType = analyzerType
	return in, nil
}

func defaultAnalyzerName(providerName string, analyzerType domain.AnalyzerType) string {
	return fmt.Sprintf("%s %s Analyzer", strings.TrimSpace(providerName), analyzerTitle(analyzerType))
}

func analyzerTitle(analyzerType domain.AnalyzerType) string {
	switch analyzerType {
	case domain.AnalyzerTranscription:
		return "Transcription"
	case domain.AnalyzerVisionTags:
		return "Vision Tags"
	case domain.AnalyzerEmbedding:
		return "Embedding"
	default:
		return "Unknown"
	}
}

func normalizeConfigJSON(raw string) (json.RawMessage, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		s = "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, err
	}
	clean, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return clean, nil
}
