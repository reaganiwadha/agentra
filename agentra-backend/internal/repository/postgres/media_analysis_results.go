package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type MediaAnalysisResultRepo struct {
	db *sqlx.DB
}

func NewMediaAnalysisResultRepo(db *sqlx.DB) *MediaAnalysisResultRepo {
	return &MediaAnalysisResultRepo{db: db}
}

func (r *MediaAnalysisResultRepo) Upsert(ctx context.Context, result domain.MediaAnalysisResult) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO media_analysis_results (id, media_id, analyzer_id, analyzer_name, analyzer_type, output, analyzed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (media_id, analyzer_id)
		DO UPDATE SET output = EXCLUDED.output, analyzed_at = EXCLUDED.analyzed_at
	`, result.ID, result.MediaID, result.AnalyzerID, result.AnalyzerName, result.AnalyzerType, result.Output, result.AnalyzedAt)
	return err
}

func (r *MediaAnalysisResultRepo) DeleteByMedia(ctx context.Context, mediaID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM media_analysis_results WHERE media_id = $1`,
		mediaID)
	return err
}

func (r *MediaAnalysisResultRepo) ListByMedia(ctx context.Context, mediaID uuid.UUID) ([]domain.MediaAnalysisResult, error) {
	out := make([]domain.MediaAnalysisResult, 0)
	err := r.db.SelectContext(ctx, &out, `
		SELECT id, media_id, analyzer_id, analyzer_name, analyzer_type, output, analyzed_at
		FROM media_analysis_results
		WHERE media_id = $1
		ORDER BY analyzed_at ASC
	`, mediaID)
	if err != nil {
		return nil, err
	}
	return out, nil
}
