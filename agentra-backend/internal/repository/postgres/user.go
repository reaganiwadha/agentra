package postgres

import (
	"github.com/reaganiwadha/agentra/internal/domain"
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type UserRepo struct {
	db *sqlx.DB
}

func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u domain.User) error {
	q, args, err := dialect.Insert("users").Rows(goqu.Record{
		"id":              u.ID,
		"organization_id": u.OrganizationID,
		"email":           u.Email,
		"password_hash":   u.PasswordHash,
		"role":            string(u.Role),
		"is_active":       u.IsActive,
		"created_at":      u.CreatedAt,
		"updated_at":      u.UpdatedAt,
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

func (r *UserRepo) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.db.GetContext(ctx, &n, "SELECT COUNT(*) FROM users WHERE role = 'admin'")
	return n, err
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	q, args, err := dialect.From("users").
		Where(goqu.C("email").Eq(email), goqu.C("is_active").IsTrue()).
		Limit(1).ToSQL()
	if err != nil {
		return domain.User{}, err
	}
	var u domain.User
	if err := r.db.GetContext(ctx, &u, q, args...); err != nil {
		return domain.User{}, getErr(err)
	}
	return u, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	q, args, err := dialect.From("users").
		Where(goqu.C("id").Eq(id), goqu.C("is_active").IsTrue()).
		Limit(1).ToSQL()
	if err != nil {
		return domain.User{}, err
	}
	var u domain.User
	if err := r.db.GetContext(ctx, &u, q, args...); err != nil {
		return domain.User{}, getErr(err)
	}
	return u, nil
}
