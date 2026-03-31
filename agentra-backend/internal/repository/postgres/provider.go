package postgres

import (
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type ProviderRepo struct {
	db *sqlx.DB
}

func NewProviderRepo(db *sqlx.DB) *ProviderRepo {
	return &ProviderRepo{db: db}
}

func (r *ProviderRepo) List(ctx context.Context, orgID uuid.UUID) ([]domain.ModelProvider, error) {
	q, args, err := dialect.From("model_providers").
		Where(goqu.C("organization_id").Eq(orgID)).
		Order(goqu.C("created_at").Asc()).ToSQL()
	if err != nil {
		return nil, err
	}
	out := make([]domain.ModelProvider, 0)
	if err := r.db.SelectContext(ctx, &out, q, args...); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ProviderRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.ModelProvider, error) {
	q, args, err := dialect.From("model_providers").
		Where(
			goqu.C("organization_id").Eq(orgID),
			goqu.C("id").Eq(id),
		).
		Limit(1).ToSQL()
	if err != nil {
		return domain.ModelProvider{}, err
	}
	var p domain.ModelProvider
	if err := r.db.GetContext(ctx, &p, q, args...); err != nil {
		return domain.ModelProvider{}, getErr(err)
	}
	return p, nil
}

func (r *ProviderRepo) Create(ctx context.Context, p domain.ModelProvider) error {
	q, args, err := dialect.Insert("model_providers").Rows(goqu.Record{
		"id":              p.ID,
		"organization_id": p.OrganizationID,
		"name":            p.Name,
		"provider_type":   string(p.ProviderType),
		"base_url":        p.BaseURL,
		"api_key":         p.APIKey,
		"is_active":       p.IsActive,
		"created_at":      p.CreatedAt,
		"updated_at":      p.UpdatedAt,
	}).ToSQL()
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (r *ProviderRepo) Update(ctx context.Context, p domain.ModelProvider) error {
	q, args, err := dialect.Update("model_providers").Set(goqu.Record{
		"name":          p.Name,
		"provider_type": string(p.ProviderType),
		"base_url":      p.BaseURL,
		"api_key":       p.APIKey,
		"is_active":     p.IsActive,
		"updated_at":    p.UpdatedAt,
	}).Where(
		goqu.C("id").Eq(p.ID),
		goqu.C("organization_id").Eq(p.OrganizationID),
	).ToSQL()
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, q, args...)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ProviderRepo) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	usedQ, usedArgs, err := dialect.From("analyzers").
		Select(goqu.L("COUNT(1)")).
		Where(
			goqu.C("organization_id").Eq(orgID),
			goqu.C("provider_id").Eq(id),
		).ToSQL()
	if err != nil {
		return err
	}
	var usedCount int64
	if err := r.db.GetContext(ctx, &usedCount, usedQ, usedArgs...); err != nil {
		return err
	}
	if usedCount > 0 {
		return ErrConflict
	}

	editorUsedQ, editorUsedArgs, err := dialect.From("editor_configs").
		Select(goqu.L("COUNT(1)")).
		Where(
			goqu.C("organization_id").Eq(orgID),
			goqu.C("provider_id").Eq(id),
		).ToSQL()
	if err != nil {
		return err
	}
	var editorUsedCount int64
	if err := r.db.GetContext(ctx, &editorUsedCount, editorUsedQ, editorUsedArgs...); err != nil {
		return err
	}
	if editorUsedCount > 0 {
		return ErrConflict
	}

	q, args, err := dialect.Delete("model_providers").Where(
		goqu.C("id").Eq(id),
		goqu.C("organization_id").Eq(orgID),
	).ToSQL()
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
