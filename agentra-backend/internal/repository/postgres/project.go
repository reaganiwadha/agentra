package postgres

import (
	"context"
	"database/sql"

	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type ProjectRepo struct {
	db *sqlx.DB
}

func NewProjectRepo(db *sqlx.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

func (r *ProjectRepo) Create(ctx context.Context, p domain.Project) error {
	q, args, err := dialect.Insert("projects").Rows(goqu.Record{
		"id":                      p.ID,
		"organization_id":         p.OrganizationID,
		"name":                    p.Name,
		"description":             p.Description,
		"editor_base_prompt":      p.EditorBasePrompt,
		"editor_variant_count":    p.EditorVariantCount,
		"editor_min_duration_sec": p.EditorMinDurationSec,
		"editor_max_duration_sec": p.EditorMaxDurationSec,
		"storage_subpath":         p.StorageSubpath,
		"status":                  string(p.Status),
		"created_by":              p.CreatedBy,
		"created_at":              p.CreatedAt,
		"updated_at":              p.UpdatedAt,
	}).ToSQL()
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, q, args...)
	return err
}

func (r *ProjectRepo) List(ctx context.Context, orgID uuid.UUID) ([]domain.Project, error) {
	q, args, err := dialect.From("projects").
		Where(
			goqu.C("organization_id").Eq(orgID),
			goqu.C("status").Eq("active"),
		).
		Order(goqu.C("created_at").Desc()).ToSQL()
	if err != nil {
		return nil, err
	}
	projects := make([]domain.Project, 0)
	if err := r.db.SelectContext(ctx, &projects, q, args...); err != nil {
		return nil, err
	}
	return projects, nil
}

func (r *ProjectRepo) GetByID(ctx context.Context, id uuid.UUID) (domain.Project, error) {
	q, args, err := dialect.From("projects").
		Where(goqu.C("id").Eq(id)).
		Limit(1).ToSQL()
	if err != nil {
		return domain.Project{}, err
	}
	var p domain.Project
	if err := r.db.GetContext(ctx, &p, q, args...); err != nil {
		return domain.Project{}, getErr(err)
	}
	return p, nil
}

func (r *ProjectRepo) UpdateMediaScope(
	ctx context.Context,
	projectID uuid.UUID,
	mode domain.MediaScopeMode,
	startAt, endAt sql.NullTime,
) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE projects
		 SET media_scope_mode = $1,
		     media_scope_start_at = $2,
		     media_scope_end_at = $3,
		     updated_at = NOW()
		 WHERE id = $4`,
		string(mode), startAt, endAt, projectID,
	)
	return err
}

func (r *ProjectRepo) UpdateDraft(
	ctx context.Context,
	projectID uuid.UUID,
	name string,
	basePrompt string,
	variantCount int,
	minDurationSec int,
	maxDurationSec int,
) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE projects
		 SET name = $1,
		     editor_base_prompt = $2,
		     editor_variant_count = $3,
		     editor_min_duration_sec = $4,
		     editor_max_duration_sec = $5,
		     updated_at = NOW()
		 WHERE id = $6`,
		name, basePrompt, variantCount, minDurationSec, maxDurationSec, projectID,
	)
	return err
}

func (r *ProjectRepo) ReplaceMediaScopeSelection(ctx context.Context, projectID uuid.UUID, mediaIDs []uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM project_media_scope_items WHERE project_id = $1`, projectID); err != nil {
		return err
	}

	seen := make(map[uuid.UUID]struct{}, len(mediaIDs))
	for _, mediaID := range mediaIDs {
		if _, ok := seen[mediaID]; ok {
			continue
		}
		seen[mediaID] = struct{}{}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO project_media_scope_items (project_id, media_id) VALUES ($1, $2)`,
			projectID, mediaID,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}
