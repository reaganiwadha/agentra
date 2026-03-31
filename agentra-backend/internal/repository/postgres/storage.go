package postgres

import (
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type StorageRepo struct {
	db *sqlx.DB
}

func NewStorageRepo(db *sqlx.DB) *StorageRepo {
	return &StorageRepo{db: db}
}

func (r *StorageRepo) GetActive(ctx context.Context) (domain.StorageConfig, error) {
	q, args, err := dialect.From("storage_configs").
		Where(goqu.C("is_active").IsTrue()).
		Limit(1).ToSQL()
	if err != nil {
		return domain.StorageConfig{}, err
	}
	var cfg domain.StorageConfig
	if err := r.db.GetContext(ctx, &cfg, q, args...); err != nil {
		return domain.StorageConfig{}, getErr(err)
	}
	return cfg, nil
}

func (r *StorageRepo) GetActiveByOrg(ctx context.Context, orgID uuid.UUID) (domain.StorageConfig, error) {
	q, args, err := dialect.From("storage_configs").
		Where(
			goqu.C("organization_id").Eq(orgID),
			goqu.C("is_active").IsTrue(),
		).
		Limit(1).ToSQL()
	if err != nil {
		return domain.StorageConfig{}, err
	}
	var cfg domain.StorageConfig
	if err := r.db.GetContext(ctx, &cfg, q, args...); err != nil {
		return domain.StorageConfig{}, getErr(err)
	}
	return cfg, nil
}

func (r *StorageRepo) Upsert(ctx context.Context, cfg domain.StorageConfig) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "UPDATE storage_configs SET is_active = FALSE WHERE organization_id = $1", cfg.OrganizationID); err != nil {
		return err
	}

	q, args, err := dialect.Insert("storage_configs").Rows(goqu.Record{
		"id":               cfg.ID,
		"organization_id":  cfg.OrganizationID,
		"storage_type":     string(cfg.StorageType),
		"endpoint":         cfg.Endpoint,
		"access_key":       cfg.AccessKey,
		"secret_key":       cfg.SecretKey,
		"bucket":           cfg.Bucket,
		"base_path":        cfg.BasePath,
		"output_base_path": cfg.OutputBasePath,
		"is_active":        cfg.IsActive,
		"created_at":       cfg.CreatedAt,
		"updated_at":       cfg.UpdatedAt,
	}).ToSQL()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, q, args...); err != nil {
		return err
	}

	return tx.Commit()
}
