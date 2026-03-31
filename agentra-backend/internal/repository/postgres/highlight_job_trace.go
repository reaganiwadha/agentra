package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type HighlightJobTraceRepo struct {
	db *sqlx.DB
}

func NewHighlightJobTraceRepo(db *sqlx.DB) *HighlightJobTraceRepo {
	return &HighlightJobTraceRepo{db: db}
}

func (r *HighlightJobTraceRepo) CreateMany(ctx context.Context, traces []domain.HighlightJobTrace) error {
	if len(traces) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, trace := range traces {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO highlight_job_traces (id, job_id, phase, message, payload, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			trace.ID,
			trace.JobID,
			trace.Phase,
			trace.Message,
			trace.Payload,
			trace.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *HighlightJobTraceRepo) ListByJobID(ctx context.Context, jobID uuid.UUID) ([]domain.HighlightJobTrace, error) {
	out := make([]domain.HighlightJobTrace, 0)
	err := r.db.SelectContext(ctx, &out, `
		SELECT id, job_id, phase, message, payload, created_at
		FROM highlight_job_traces
		WHERE job_id = $1
		ORDER BY created_at ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	return out, nil
}
