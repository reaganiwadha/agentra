package postgres

import (
	"context"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type EditorRepo struct {
	db *sqlx.DB
}

func NewEditorRepo(db *sqlx.DB) *EditorRepo {
	return &EditorRepo{db: db}
}

func (r *EditorRepo) Get(ctx context.Context, orgID uuid.UUID) (domain.EditorConfig, error) {
	q, args, err := dialect.From("editor_configs").
		Where(goqu.C("organization_id").Eq(orgID)).
		Limit(1).ToSQL()
	if err != nil {
		return domain.EditorConfig{}, err
	}
	var cfg domain.EditorConfig
	if err := r.db.GetContext(ctx, &cfg, q, args...); err != nil {
		return domain.EditorConfig{}, getErr(err)
	}
	return cfg, nil
}

func (r *EditorRepo) Upsert(ctx context.Context, cfg domain.EditorConfig) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if cfg.UpdatedAt.IsZero() {
		cfg.UpdatedAt = time.Now()
	}

	updateQ, updateArgs, err := dialect.Update("editor_configs").Set(goqu.Record{
		"provider_id":           cfg.ProviderID,
		"model_name":            cfg.ModelName,
		"base_prompt":           cfg.BasePrompt,
		"max_duration_sec":      cfg.MaxDurationSec,
		"is_autonomous_enabled": cfg.IsAutonomousEnabled,
		"updated_at":            cfg.UpdatedAt,
	}).Where(
		goqu.C("organization_id").Eq(cfg.OrganizationID),
	).ToSQL()
	if err != nil {
		return err
	}
	updateRes, err := tx.ExecContext(ctx, updateQ, updateArgs...)
	if err != nil {
		return err
	}
	affected, err := updateRes.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return tx.Commit()
	}

	insertQ, insertArgs, err := dialect.Insert("editor_configs").Rows(goqu.Record{
		"id":                    cfg.ID,
		"organization_id":       cfg.OrganizationID,
		"provider_id":           cfg.ProviderID,
		"model_name":            cfg.ModelName,
		"base_prompt":           cfg.BasePrompt,
		"max_duration_sec":      cfg.MaxDurationSec,
		"is_autonomous_enabled": cfg.IsAutonomousEnabled,
		"created_at":            cfg.CreatedAt,
		"updated_at":            cfg.UpdatedAt,
	}).ToSQL()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, insertQ, insertArgs...); err != nil {
		return err
	}
	return tx.Commit()
}
