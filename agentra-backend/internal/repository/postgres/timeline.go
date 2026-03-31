package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type TimelineRepo struct {
	db *sqlx.DB
}

func NewTimelineRepo(db *sqlx.DB) *TimelineRepo {
	return &TimelineRepo{db: db}
}

func (r *TimelineRepo) Create(ctx context.Context, timeline domain.Timeline) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timelines (id, job_id, otio_json, created_at)
		 VALUES ($1, $2, $3, $4)`,
		timeline.ID,
		timeline.JobID,
		timeline.OTIOJson,
		timeline.CreatedAt,
	)
	return err
}

func (r *TimelineRepo) GetByJobID(ctx context.Context, jobID uuid.UUID) (domain.Timeline, error) {
	var timeline domain.Timeline
	err := r.db.GetContext(ctx, &timeline, `
		SELECT id, job_id, otio_json, created_at
		FROM timelines
		WHERE job_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, jobID)
	if err != nil {
		return domain.Timeline{}, getErr(err)
	}
	return timeline, nil
}
