package postgres

import (
	"context"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

var capturedOrCreated = goqu.Func("COALESCE", goqu.I("captured_at"), goqu.I("created_at"))
var capturedOrCreatedQ = goqu.Func("COALESCE", goqu.I("m.captured_at"), goqu.I("m.created_at"))

type MediaAssetRepo struct {
	db *sqlx.DB
}

func NewMediaAssetRepo(db *sqlx.DB) *MediaAssetRepo {
	return &MediaAssetRepo{db: db}
}

func (r *MediaAssetRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.MediaAsset, error) {
	q, args, err := dialect.From("media_assets").
		Where(goqu.C("organization_id").Eq(orgID)).
		Order(capturedOrCreated.Desc(), goqu.I("created_at").Desc()).
		ToSQL()
	if err != nil {
		return nil, err
	}

	assets := make([]domain.MediaAsset, 0)
	if err := r.db.SelectContext(ctx, &assets, q, args...); err != nil {
		return nil, err
	}
	return assets, nil
}

func (r *MediaAssetRepo) ListByOrgInDateRange(
	ctx context.Context,
	orgID uuid.UUID,
	startAt, endAt *time.Time,
) ([]domain.MediaAsset, error) {
	ds := dialect.From("media_assets").Where(goqu.C("organization_id").Eq(orgID))
	if startAt != nil {
		ds = ds.Where(capturedOrCreated.Gte(*startAt))
	}
	if endAt != nil {
		ds = ds.Where(capturedOrCreated.Lte(*endAt))
	}

	q, args, err := ds.Order(capturedOrCreated.Desc(), goqu.I("created_at").Desc()).ToSQL()
	if err != nil {
		return nil, err
	}

	assets := make([]domain.MediaAsset, 0)
	if err := r.db.SelectContext(ctx, &assets, q, args...); err != nil {
		return nil, err
	}
	return assets, nil
}

func (r *MediaAssetRepo) ListByProjectSelection(ctx context.Context, projectID uuid.UUID) ([]domain.MediaAsset, error) {
	q, args, err := dialect.
		From(goqu.T("project_media_scope_items").As("s")).
		Join(
			goqu.T("media_assets").As("m"),
			goqu.On(goqu.I("m.id").Eq(goqu.I("s.media_id"))),
		).
		Where(goqu.I("s.project_id").Eq(projectID)).
		Select("m.*").
		Order(capturedOrCreatedQ.Desc(), goqu.I("m.created_at").Desc()).
		ToSQL()
	if err != nil {
		return nil, err
	}
	assets := make([]domain.MediaAsset, 0)
	if err := r.db.SelectContext(ctx, &assets, q, args...); err != nil {
		return nil, err
	}
	return assets, nil
}

func (r *MediaAssetRepo) GetByIDForOrg(ctx context.Context, id, orgID uuid.UUID) (domain.MediaAsset, error) {
	q, args, err := dialect.From("media_assets").
		Where(
			goqu.I("id").Eq(id),
			goqu.I("organization_id").Eq(orgID),
		).
		Limit(1).
		ToSQL()
	if err != nil {
		return domain.MediaAsset{}, err
	}

	var asset domain.MediaAsset
	if err := r.db.GetContext(ctx, &asset, q, args...); err != nil {
		return domain.MediaAsset{}, getErr(err)
	}
	return asset, nil
}

func (r *MediaAssetRepo) Create(ctx context.Context, a domain.MediaAsset) error {
	q, args, err := dialect.Insert("media_assets").Rows(goqu.Record{
		"id":              a.ID,
		"organization_id": a.OrganizationID,
		"project_id":      a.ProjectID,
		"filename":        a.Filename,
		"storage_path":    a.StoragePath,
		"thumbnail_path":  a.ThumbnailPath,
		"mime_type":       a.MIMEType,
		"sha256":          a.SHA256,
		"duration_sec":    a.DurationSec,
		"file_size_bytes": a.FileSizeBytes,
		"captured_at":     a.CapturedAt,
		"status":          string(a.Status),
		"created_at":      a.CreatedAt,
		"updated_at":      a.UpdatedAt,
	}).ToSQL()
	if err != nil {
		return err
	}

	if _, err := r.db.ExecContext(ctx, q, args...); err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

func (r *MediaAssetRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.MediaStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE media_assets SET status = $1, updated_at = NOW() WHERE id = $2`,
		string(status), id)
	return err
}

func (r *MediaAssetRepo) UpdateSHA256(ctx context.Context, id uuid.UUID, sha256 string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE media_assets SET sha256 = $1, updated_at = NOW() WHERE id = $2`,
		sha256, id)
	return err
}

func (r *MediaAssetRepo) Delete(ctx context.Context, id, orgID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM media_assets WHERE id = $1 AND organization_id = $2`,
		id, orgID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MediaAssetRepo) DeleteScopeItems(ctx context.Context, mediaID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM project_media_scope_items WHERE media_id = $1`,
		mediaID)
	return err
}

// Upsert inserts a new media asset. On conflict with an existing storage_path for the
// same project it does nothing — the asset is already tracked.
func (r *MediaAssetRepo) Upsert(ctx context.Context, a domain.MediaAsset) error {
	q, args, err := dialect.Insert("media_assets").Rows(goqu.Record{
		"id":              a.ID,
		"organization_id": a.OrganizationID,
		"project_id":      a.ProjectID,
		"filename":        a.Filename,
		"storage_path":    a.StoragePath,
		"thumbnail_path":  a.ThumbnailPath,
		"mime_type":       a.MIMEType,
		"sha256":          a.SHA256,
		"duration_sec":    a.DurationSec,
		"file_size_bytes": a.FileSizeBytes,
		"captured_at":     a.CapturedAt,
		"status":          string(a.Status),
		"created_at":      a.CreatedAt,
		"updated_at":      a.UpdatedAt,
	}).OnConflict(goqu.DoNothing()).ToSQL()
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	return err
}
