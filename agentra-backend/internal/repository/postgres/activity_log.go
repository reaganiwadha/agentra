package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type ActivityLogRepo struct {
	db *sqlx.DB
}

func NewActivityLogRepo(db *sqlx.DB) *ActivityLogRepo {
	return &ActivityLogRepo{db: db}
}

func (r *ActivityLogRepo) Create(ctx context.Context, log domain.ActivityLog) error {
	payload := log.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	payloadJSON := string(payload)
	if payloadJSON == "" {
		payloadJSON = "{}"
	}

	q, args, err := dialect.Insert("activity_logs").Prepared(true).Rows(goqu.Record{
		"id":              log.ID,
		"organization_id": log.OrganizationID,
		"source":          string(log.Source),
		"subject_type":    log.SubjectType,
		"subject_id":      log.SubjectID,
		"event_type":      string(log.EventType),
		"level":           log.Level,
		"message":         log.Message,
		"payload":         payloadJSON,
		"is_active":       log.IsActive,
		"created_at":      log.CreatedAt,
	}).ToSQL()
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *ActivityLogRepo) LatestByOrg(ctx context.Context, orgID uuid.UUID) (domain.ActivityLog, error) {
	q, args, err := dialect.From("activity_logs").
		Where(goqu.C("organization_id").Eq(orgID)).
		Order(goqu.C("created_at").Desc()).
		Limit(1).
		ToSQL()
	if err != nil {
		return domain.ActivityLog{}, err
	}

	var log domain.ActivityLog
	if err := r.db.GetContext(ctx, &log, q, args...); err != nil {
		return domain.ActivityLog{}, getErr(err)
	}
	return log, nil
}

func (r *ActivityLogRepo) ListRecentByOrg(ctx context.Context, orgID uuid.UUID, limit int) ([]domain.ActivityLog, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 300 {
		limit = 300
	}

	q, args, err := dialect.From("activity_logs").
		Where(goqu.C("organization_id").Eq(orgID)).
		Order(goqu.C("created_at").Desc()).
		Limit(uint(limit)).
		ToSQL()
	if err != nil {
		return nil, err
	}

	out := make([]domain.ActivityLog, 0)
	if err := r.db.SelectContext(ctx, &out, q, args...); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ActivityLogRepo) HasActiveSince(ctx context.Context, orgID uuid.UUID, since time.Time) (bool, error) {
	q, args, err := dialect.From("activity_logs").
		Select(goqu.L("COUNT(1)")).
		Where(
			goqu.C("organization_id").Eq(orgID),
			goqu.C("is_active").IsTrue(),
			goqu.C("created_at").Gte(since),
		).
		ToSQL()
	if err != nil {
		return false, err
	}

	var count int64
	if err := r.db.GetContext(ctx, &count, q, args...); err != nil {
		return false, err
	}
	return count > 0, nil
}
