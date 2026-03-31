package postgres

import (
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type AnalyzerRepo struct {
	db *sqlx.DB
}

func NewAnalyzerRepo(db *sqlx.DB) *AnalyzerRepo {
	return &AnalyzerRepo{db: db}
}

func (r *AnalyzerRepo) List(ctx context.Context, orgID uuid.UUID) ([]domain.Analyzer, error) {
	q, args, err := dialect.From(goqu.T("analyzers").As("a")).
		LeftJoin(
			goqu.T("model_providers").As("p"),
			goqu.On(
				goqu.Ex{
					"p.id":              goqu.I("a.provider_id"),
					"p.organization_id": goqu.I("a.organization_id"),
				},
			),
		).
		Select(
			goqu.I("a.id"),
			goqu.I("a.organization_id"),
			goqu.I("a.name"),
			goqu.I("a.analyzer_type"),
			goqu.I("a.provider_id"),
			goqu.I("a.model_name"),
			goqu.I("a.config_json"),
			goqu.I("a.is_enabled"),
			goqu.I("a.created_at"),
			goqu.I("a.updated_at"),
			goqu.COALESCE(goqu.I("p.name"), "").As("provider_name"),
			goqu.COALESCE(goqu.I("p.provider_type"), "").As("provider_type"),
			goqu.COALESCE(goqu.I("p.is_active"), false).As("provider_is_active"),
		).
		Where(goqu.I("a.organization_id").Eq(orgID)).
		Order(goqu.I("a.created_at").Asc(), goqu.I("a.name").Asc()).
		ToSQL()
	if err != nil {
		return nil, err
	}
	out := make([]domain.Analyzer, 0)
	if err := r.db.SelectContext(ctx, &out, q, args...); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AnalyzerRepo) Create(ctx context.Context, a domain.Analyzer) error {
	q, args, err := dialect.Insert("analyzers").Rows(goqu.Record{
		"id":              a.ID,
		"organization_id": a.OrganizationID,
		"name":            a.Name,
		"analyzer_type":   string(a.AnalyzerType),
		"provider_id":     a.ProviderID,
		"model_name":      a.ModelName,
		"config_json":     string(a.ConfigJSON),
		"is_enabled":      a.IsEnabled,
		"created_at":      a.CreatedAt,
		"updated_at":      a.UpdatedAt,
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

func (r *AnalyzerRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (domain.Analyzer, error) {
	q, args, err := dialect.From("analyzers").
		Where(
			goqu.C("organization_id").Eq(orgID),
			goqu.C("id").Eq(id),
		).
		Limit(1).ToSQL()
	if err != nil {
		return domain.Analyzer{}, err
	}
	var a domain.Analyzer
	if err := r.db.GetContext(ctx, &a, q, args...); err != nil {
		return domain.Analyzer{}, getErr(err)
	}
	return a, nil
}

func (r *AnalyzerRepo) Update(ctx context.Context, a domain.Analyzer) error {
	q, args, err := dialect.Update("analyzers").Set(goqu.Record{
		"name":          a.Name,
		"analyzer_type": string(a.AnalyzerType),
		"provider_id":   a.ProviderID,
		"model_name":    a.ModelName,
		"config_json":   string(a.ConfigJSON),
		"is_enabled":    a.IsEnabled,
		"updated_at":    a.UpdatedAt,
	}).Where(
		goqu.C("id").Eq(a.ID),
		goqu.C("organization_id").Eq(a.OrganizationID),
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

func (r *AnalyzerRepo) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	q, args, err := dialect.Delete("analyzers").Where(
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

func (r *AnalyzerRepo) Disable(ctx context.Context, orgID, id uuid.UUID) error {
	q, args, err := dialect.Update("analyzers").Set(goqu.Record{
		"is_enabled": false,
		"updated_at": goqu.L("NOW()"),
	}).Where(
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

func (r *AnalyzerRepo) ListEnabledWithProviders(ctx context.Context, orgID uuid.UUID) ([]domain.EnabledAnalyzer, error) {
	// Deterministic execution order: type -> created_at -> name.
	q, args, err := dialect.From(goqu.T("analyzers").As("a")).
		Join(
			goqu.T("model_providers").As("p"),
			goqu.On(
				goqu.Ex{
					"p.id":              goqu.I("a.provider_id"),
					"p.organization_id": goqu.I("a.organization_id"),
				},
			),
		).
		Select(
			goqu.I("a.id"),
			goqu.I("a.name"),
			goqu.I("a.analyzer_type"),
			goqu.I("a.provider_id"),
			goqu.I("a.model_name"),
			goqu.I("a.config_json"),
			goqu.I("a.created_at"),
			goqu.I("p.name").As("provider_name"),
			goqu.I("p.provider_type").As("provider_type"),
			goqu.I("p.base_url").As("base_url"),
			goqu.I("p.api_key").As("api_key"),
		).
		Where(
			goqu.I("a.organization_id").Eq(orgID),
			goqu.I("a.is_enabled").IsTrue(),
			goqu.I("p.is_active").IsTrue(),
		).
		Order(
			goqu.Case().
				Value(goqu.I("a.analyzer_type")).
				When("transcription", 1).
				When("vision_tags", 2).
				When("embedding", 3).
				Else(999).Asc(),
			goqu.I("a.created_at").Asc(),
			goqu.I("a.name").Asc(),
		).
		ToSQL()
	if err != nil {
		return nil, err
	}
	out := make([]domain.EnabledAnalyzer, 0)
	if err := r.db.SelectContext(ctx, &out, q, args...); err != nil {
		return nil, err
	}
	return out, nil
}
