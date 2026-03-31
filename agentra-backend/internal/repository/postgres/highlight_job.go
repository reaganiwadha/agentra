package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
	"gopkg.in/guregu/null.v4"
)

type HighlightJobRepo struct {
	db *sqlx.DB
}

func NewHighlightJobRepo(db *sqlx.DB) *HighlightJobRepo {
	return &HighlightJobRepo{db: db}
}

func (r *HighlightJobRepo) CreateMany(ctx context.Context, jobs []domain.HighlightJob) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, job := range jobs {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO highlight_jobs
				(id, project_id, requested_by, prompt, variant_index, variant_count, min_duration_sec, max_duration_sec, status, error_message, started_at, finished_at, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			job.ID,
			job.ProjectID,
			job.RequestedBy,
			job.Prompt,
			job.VariantIndex,
			job.VariantCount,
			job.MinDurationSec,
			job.MaxDurationSec,
			string(job.Status),
			job.ErrorMessage,
			job.StartedAt,
			job.FinishedAt,
			job.CreatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *HighlightJobRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]domain.HighlightJob, error) {
	out := make([]domain.HighlightJob, 0)
	err := r.db.SelectContext(ctx, &out, `
		SELECT id, project_id, requested_by, prompt, variant_index, variant_count, min_duration_sec, max_duration_sec, status, error_message, started_at, finished_at, created_at
		FROM highlight_jobs
		WHERE project_id = $1
		ORDER BY created_at DESC, variant_index ASC
	`, projectID)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *HighlightJobRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.HighlightJob, error) {
	var job domain.HighlightJob
	err := r.db.GetContext(ctx, &job, `
		SELECT id, project_id, requested_by, prompt, variant_index, variant_count, min_duration_sec, max_duration_sec, status, error_message, started_at, finished_at, created_at
		FROM highlight_jobs
		WHERE id = $1
		LIMIT 1
	`, id)
	if err != nil {
		return domain.HighlightJob{}, getErr(err)
	}
	return job, nil
}

func (r *HighlightJobRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.JobStatus) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE highlight_jobs
		 SET status = $1
		 WHERE id = $2`,
		string(status), id,
	)
	return err
}

func (r *HighlightJobRepo) ListQueued(ctx context.Context, limit int) ([]domain.HighlightJob, error) {
	if limit <= 0 {
		limit = 10
	}
	out := make([]domain.HighlightJob, 0, limit)
	err := r.db.SelectContext(ctx, &out, `
		SELECT id, project_id, requested_by, prompt, variant_index, variant_count, min_duration_sec, max_duration_sec, status, error_message, started_at, finished_at, created_at
		FROM highlight_jobs
		WHERE status = 'queued'
		ORDER BY created_at ASC, variant_index ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *HighlightJobRepo) MarkRunning(ctx context.Context, id uuid.UUID, startedAt time.Time) (bool, error) {
	res, err := r.db.ExecContext(
		ctx,
		`UPDATE highlight_jobs
		 SET status = $1,
		     started_at = $2,
		     finished_at = NULL,
		     error_message = NULL
		 WHERE id = $3
		   AND status = 'queued'`,
		string(domain.JobRunning), startedAt, id,
	)
	if err != nil {
		return false, err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *HighlightJobRepo) MarkCompleted(ctx context.Context, id uuid.UUID, finishedAt time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE highlight_jobs
		 SET status = $1,
		     finished_at = $2
		 WHERE id = $3`,
		string(domain.JobCompleted), finishedAt, id,
	)
	return err
}

func (r *HighlightJobRepo) MarkFailed(ctx context.Context, id uuid.UUID, message string, finishedAt time.Time) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE highlight_jobs
		 SET status = $1,
		     error_message = $2,
		     finished_at = $3
		 WHERE id = $4`,
		string(domain.JobFailed), null.StringFrom(message), finishedAt, id,
	)
	return err
}
