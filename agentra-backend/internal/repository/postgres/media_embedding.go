package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type MediaEmbeddingRepo struct {
	db *sqlx.DB
}

func NewMediaEmbeddingRepo(db *sqlx.DB) *MediaEmbeddingRepo {
	return &MediaEmbeddingRepo{db: db}
}

func (r *MediaEmbeddingRepo) Upsert(ctx context.Context, e domain.MediaEmbedding, vec []float64) error {
	vecLit := vectorLiteral(vec)

	if e.SegmentIndex == nil {
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO media_embeddings
				(id, media_id, analyzer_id, segment_index, start_sec, end_sec, source_text, embedding)
			VALUES ($1, $2, $3, NULL, NULL, NULL, $4, `+vecLit+`::vector)
			ON CONFLICT (media_id, analyzer_id) WHERE segment_index IS NULL
			DO UPDATE SET source_text = EXCLUDED.source_text,
			              embedding   = EXCLUDED.embedding,
			              created_at  = NOW()
		`, e.ID, e.MediaID, e.AnalyzerID, e.SourceText)
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO media_embeddings
			(id, media_id, analyzer_id, segment_index, start_sec, end_sec, source_text, embedding)
		VALUES ($1, $2, $3, $4, $5, $6, $7, `+vecLit+`::vector)
		ON CONFLICT (media_id, analyzer_id, segment_index) WHERE segment_index IS NOT NULL
		DO UPDATE SET start_sec   = EXCLUDED.start_sec,
		              end_sec     = EXCLUDED.end_sec,
		              source_text = EXCLUDED.source_text,
		              embedding   = EXCLUDED.embedding,
		              created_at  = NOW()
	`, e.ID, e.MediaID, e.AnalyzerID, e.SegmentIndex, e.StartSec, e.EndSec, e.SourceText)
	return err
}

func (r *MediaEmbeddingRepo) DeleteByMedia(ctx context.Context, mediaID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM media_embeddings WHERE media_id = $1`, mediaID)
	return err
}

func (r *MediaEmbeddingRepo) SearchByMediaIDs(
	ctx context.Context,
	mediaIDs []uuid.UUID,
	queryVec []float64,
	limit int,
) ([]domain.EmbeddingSearchHit, error) {
	if len(mediaIDs) == 0 || len(queryVec) == 0 {
		return nil, nil
	}

	vecLit := vectorLiteral(queryVec)
	idArr := uuidArrayLiteral(mediaIDs)

	query := fmt.Sprintf(`
		SELECT
			me.media_id,
			ma.filename,
			ma.storage_path,
			me.segment_index,
			me.start_sec,
			me.end_sec,
			me.source_text,
			1 - (me.embedding <=> %s::vector) AS score
		FROM media_embeddings me
		JOIN media_assets ma ON ma.id = me.media_id
		WHERE me.media_id = ANY(%s)
		  AND ma.status = 'ready'
		ORDER BY me.embedding <=> %s::vector
		LIMIT $1
	`, vecLit, idArr, vecLit)

	out := make([]domain.EmbeddingSearchHit, 0)
	if err := r.db.SelectContext(ctx, &out, query, limit); err != nil {
		return nil, err
	}
	return out, nil
}

func vectorLiteral(vec []float64) string {
	parts := make([]string, len(vec))
	for i, f := range vec {
		parts[i] = strconv.FormatFloat(f, 'f', -1, 64)
	}
	return "'[" + strings.Join(parts, ",") + "]'"
}

func uuidArrayLiteral(ids []uuid.UUID) string {
	if len(ids) == 0 {
		return "ARRAY[]::uuid[]"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = "'" + id.String() + "'"
	}
	return "ARRAY[" + strings.Join(parts, ",") + "]::uuid[]"
}
