package postgres

import (
	"github.com/reaganiwadha/agentra/internal/domain"
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

type OrgRepo struct {
	db *sqlx.DB
}

func NewOrgRepo(db *sqlx.DB) *OrgRepo {
	return &OrgRepo{db: db}
}

func (r *OrgRepo) Create(ctx context.Context, org domain.Organization) error {
	q, args, err := dialect.Insert("organizations").Rows(goqu.Record{
		"id":         org.ID,
		"name":       org.Name,
		"slug":       org.Slug,
		"is_active":  org.IsActive,
		"created_at": org.CreatedAt,
	}).ToSQL()
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *OrgRepo) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, "SELECT COUNT(*) FROM organizations")
	return n, err
}

func (r *OrgRepo) First(ctx context.Context) (domain.Organization, error) {
	q, args, err := dialect.From("organizations").Limit(1).ToSQL()
	if err != nil {
		return domain.Organization{}, err
	}
	var org domain.Organization
	if err := r.db.GetContext(ctx, &org, q, args...); err != nil {
		return domain.Organization{}, getErr(err)
	}
	return org, nil
}
