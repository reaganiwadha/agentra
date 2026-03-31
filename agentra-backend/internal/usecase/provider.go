package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
	"gopkg.in/guregu/null.v4"
)

type ProviderUsecase struct {
	providers *postgres.ProviderRepo
}

func NewProviderUsecase(providers *postgres.ProviderRepo) *ProviderUsecase {
	return &ProviderUsecase{providers: providers}
}

const DeepgramBaseURL = "https://api.deepgram.com"

func resolveBaseURL(t domain.ProviderType, provided string) string {
	if t == domain.ProviderDeepgram {
		return DeepgramBaseURL
	}
	return strings.TrimSpace(provided)
}

func (u *ProviderUsecase) List(ctx context.Context, orgID uuid.UUID) ([]domain.ModelProvider, error) {
	items, err := u.providers.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].HasKey = strings.TrimSpace(items[i].APIKey.String) != ""
	}
	return items, nil
}

type ProviderInput struct {
	Name         string
	ProviderType domain.ProviderType
	BaseURL      string
	APIKey       string
	IsActive     bool
}

func (u *ProviderUsecase) Create(ctx context.Context, orgID uuid.UUID, in ProviderInput) (domain.ModelProvider, error) {
	now := time.Now()
	item := domain.ModelProvider{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           strings.TrimSpace(in.Name),
		ProviderType:   in.ProviderType,
		BaseURL:        resolveBaseURL(in.ProviderType, in.BaseURL),
		APIKey:         nullableString(strings.TrimSpace(in.APIKey)),
		IsActive:       in.IsActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := u.providers.Create(ctx, item); err != nil {
		return domain.ModelProvider{}, normalizeRepoErr(err)
	}
	item.HasKey = strings.TrimSpace(item.APIKey.String) != ""
	return item, nil
}

func (u *ProviderUsecase) Update(ctx context.Context, orgID, id uuid.UUID, in ProviderInput) (domain.ModelProvider, error) {
	existing, err := u.providers.GetByID(ctx, orgID, id)
	if err != nil {
		return domain.ModelProvider{}, normalizeRepoErr(err)
	}
	apiKey := strings.TrimSpace(in.APIKey)
	if apiKey == "" {
		apiKey = existing.APIKey.String
	}

	existing.Name = strings.TrimSpace(in.Name)
	existing.ProviderType = in.ProviderType
	existing.BaseURL = resolveBaseURL(in.ProviderType, in.BaseURL)
	existing.APIKey = nullableString(apiKey)
	existing.IsActive = in.IsActive
	existing.UpdatedAt = time.Now()

	if err := u.providers.Update(ctx, existing); err != nil {
		return domain.ModelProvider{}, normalizeRepoErr(err)
	}
	existing.HasKey = strings.TrimSpace(existing.APIKey.String) != ""
	return existing, nil
}

func (u *ProviderUsecase) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	err := u.providers.Delete(ctx, orgID, id)
	if errors.Is(err, postgres.ErrConflict) {
		return ErrConflict.New("cannot delete provider: it is used by analyzers or editor agent")
	}
	return normalizeRepoErr(err)
}

func (u *ProviderUsecase) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ModelProvider, error) {
	item, err := u.providers.GetByID(ctx, orgID, id)
	if err != nil {
		return domain.ModelProvider{}, normalizeRepoErr(err)
	}
	item.HasKey = strings.TrimSpace(item.APIKey.String) != ""
	return item, nil
}

func nullableString(v string) null.String {
	if strings.TrimSpace(v) == "" {
		return null.String{}
	}
	return null.StringFrom(v)
}
