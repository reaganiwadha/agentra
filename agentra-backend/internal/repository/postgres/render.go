package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type RenderRepo struct {
	db *sqlx.DB
}

func NewRenderRepo(db *sqlx.DB) *RenderRepo {
	return &RenderRepo{db: db}
}

func (r *RenderRepo) Create(ctx context.Context, render domain.Render) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO renders (id, job_id, output_path, duration_sec, file_size_bytes, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		render.ID,
		render.JobID,
		render.OutputPath,
		render.DurationSec,
		render.FileSizeBytes,
		render.CreatedAt,
	)
	return err
}

func (r *RenderRepo) GetByJobID(ctx context.Context, jobID uuid.UUID) (domain.Render, error) {
	var render domain.Render
	err := r.db.GetContext(ctx, &render, `
		SELECT id, job_id, output_path, duration_sec, file_size_bytes, created_at
		FROM renders
		WHERE job_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, jobID)
	if err != nil {
		return domain.Render{}, getErr(err)
	}
	return render, nil
}

func (r *RenderRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Render, error) {
	var render domain.Render
	err := r.db.GetContext(ctx, &render, `
		SELECT id, job_id, output_path, duration_sec, file_size_bytes, created_at
		FROM renders
		WHERE id = $1
		LIMIT 1
	`, id)
	if err != nil {
		return domain.Render{}, getErr(err)
	}
	return render, nil
}
