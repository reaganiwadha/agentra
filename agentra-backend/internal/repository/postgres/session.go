package postgres

import (
	"github.com/reaganiwadha/agentra/internal/domain"
	"context"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type SessionRepo struct {
	db *sqlx.DB
}

func NewSessionRepo(db *sqlx.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, s domain.Session) error {
	q, args, err := dialect.Insert("sessions").Rows(goqu.Record{
		"id":         s.ID,
		"user_id":    s.UserID,
		"expires_at": s.ExpiresAt,
		"created_at": s.CreatedAt,
	}).ToSQL()
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *SessionRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	q, args, err := dialect.From("sessions").
		Where(goqu.C("id").Eq(id)).
		Limit(1).ToSQL()
	if err != nil {
		return domain.Session{}, err
	}
	var s domain.Session
	if err := r.db.GetContext(ctx, &s, q, args...); err != nil {
		return domain.Session{}, getErr(err)
	}
	return s, nil
}

func (r *SessionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	q, args, err := dialect.Delete("sessions").
		Where(goqu.C("id").Eq(id)).ToSQL()
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	return err
}
