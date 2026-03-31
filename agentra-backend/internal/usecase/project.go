package usecase

import (
	"context"
	"database/sql"
	"io"
	"time"

	"github.com/google/uuid"
	adapterStorage "github.com/reaganiwadha/agentra/internal/adapter/storage"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
	"gopkg.in/guregu/null.v4"
)

type ProjectUsecase struct {
	projects  *postgres.ProjectRepo
	media     *postgres.MediaAssetRepo
	jobs      *postgres.HighlightJobRepo
	timelines *postgres.TimelineRepo
	renders   *postgres.RenderRepo
	traces    *postgres.HighlightJobTraceRepo
	storage   *postgres.StorageRepo
}

func NewProjectUsecase(
	projects *postgres.ProjectRepo,
	media *postgres.MediaAssetRepo,
	jobs *postgres.HighlightJobRepo,
	timelines *postgres.TimelineRepo,
	renders *postgres.RenderRepo,
	traces *postgres.HighlightJobTraceRepo,
	storage *postgres.StorageRepo,
) *ProjectUsecase {
	return &ProjectUsecase{
		projects:  projects,
		media:     media,
		jobs:      jobs,
		timelines: timelines,
		renders:   renders,
		traces:    traces,
		storage:   storage,
	}
}

func (u *ProjectUsecase) List(ctx context.Context, orgID uuid.UUID) ([]domain.Project, error) {
	return u.projects.List(ctx, orgID)
}

func (u *ProjectUsecase) Get(ctx context.Context, orgID, projectID uuid.UUID) (domain.Project, error) {
	project, err := u.projects.GetByID(ctx, projectID)
	if err != nil {
		return domain.Project{}, normalizeRepoErr(err)
	}
	if project.OrganizationID != orgID {
		return domain.Project{}, ErrForbidden.New("forbidden")
	}
	return project, nil
}

type ProjectInput struct {
	Name           string
	Description    string
	StorageSubpath string
}

func (u *ProjectUsecase) Create(ctx context.Context, orgID, createdBy uuid.UUID, in ProjectInput) (domain.Project, error) {
	now := time.Now()
	p := domain.Project{
		ID:                   uuid.New(),
		OrganizationID:       orgID,
		Name:                 in.Name,
		Description:          null.StringFrom(in.Description),
		EditorBasePrompt:     "",
		EditorVariantCount:   1,
		EditorMinDurationSec: 30,
		EditorMaxDurationSec: 60,
		StorageSubpath:       null.NewString(in.StorageSubpath, in.StorageSubpath != ""),
		MediaScopeMode:       domain.MediaScopeSelected,
		Status:               domain.ProjectActive,
		CreatedBy:            createdBy,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := u.projects.Create(ctx, p); err != nil {
		return domain.Project{}, err
	}
	return p, nil
}

type ProjectDraftInput struct {
	Name           string
	BasePrompt     string
	VariantCount   int
	MinDurationSec int
	MaxDurationSec int
}

func (u *ProjectUsecase) UpdateDraft(
	ctx context.Context,
	orgID, projectID uuid.UUID,
	in ProjectDraftInput,
) (domain.Project, error) {
	project, err := u.projects.GetByID(ctx, projectID)
	if err != nil {
		return domain.Project{}, normalizeRepoErr(err)
	}
	if project.OrganizationID != orgID {
		return domain.Project{}, ErrForbidden.New("forbidden")
	}

	name := in.Name
	if name == "" {
		name = project.Name
	}
	if in.VariantCount < 1 || in.VariantCount > 12 {
		return domain.Project{}, ErrBadRequest.New("variant_count must be between 1 and 12")
	}
	if in.MinDurationSec < 1 {
		return domain.Project{}, ErrBadRequest.New("min_duration_sec must be at least 1")
	}
	if in.MaxDurationSec < 1 {
		return domain.Project{}, ErrBadRequest.New("max_duration_sec must be at least 1")
	}
	if in.MinDurationSec > in.MaxDurationSec {
		return domain.Project{}, ErrBadRequest.New("min_duration_sec must be less than or equal to max_duration_sec")
	}

	if err := u.projects.UpdateDraft(ctx, projectID, name, in.BasePrompt, in.VariantCount, in.MinDurationSec, in.MaxDurationSec); err != nil {
		return domain.Project{}, normalizeRepoErr(err)
	}

	project, err = u.projects.GetByID(ctx, projectID)
	if err != nil {
		return domain.Project{}, normalizeRepoErr(err)
	}
	return project, nil
}

type ProjectMediaScopeInput struct {
	Mode     domain.MediaScopeMode
	StartAt  *time.Time
	EndAt    *time.Time
	MediaIDs []uuid.UUID
}

func (u *ProjectUsecase) SetMediaScope(
	ctx context.Context,
	orgID, projectID uuid.UUID,
	in ProjectMediaScopeInput,
) (domain.Project, error) {
	project, err := u.projects.GetByID(ctx, projectID)
	if err != nil {
		return domain.Project{}, normalizeRepoErr(err)
	}
	if project.OrganizationID != orgID {
		return domain.Project{}, ErrForbidden.New("forbidden")
	}

	switch in.Mode {
	case domain.MediaScopeGlobal:
	case domain.MediaScopeDateRange:
		if in.StartAt == nil && in.EndAt == nil {
			return domain.Project{}, ErrBadRequest.New("date_range requires start_at and/or end_at")
		}
	case domain.MediaScopeSelected:
		for _, mediaID := range in.MediaIDs {
			if _, err := u.media.GetByIDForOrg(ctx, mediaID, orgID); err != nil {
				return domain.Project{}, normalizeRepoErr(err)
			}
		}
	default:
		return domain.Project{}, ErrBadRequest.New("invalid media scope mode")
	}

	startAt := sql.NullTime{}
	if in.StartAt != nil {
		startAt = sql.NullTime{Time: *in.StartAt, Valid: true}
	}
	endAt := sql.NullTime{}
	if in.EndAt != nil {
		endAt = sql.NullTime{Time: *in.EndAt, Valid: true}
	}

	if err := u.projects.UpdateMediaScope(ctx, projectID, in.Mode, startAt, endAt); err != nil {
		return domain.Project{}, normalizeRepoErr(err)
	}
	if in.Mode == domain.MediaScopeSelected {
		if err := u.projects.ReplaceMediaScopeSelection(ctx, projectID, in.MediaIDs); err != nil {
			return domain.Project{}, normalizeRepoErr(err)
		}
	} else {
		if err := u.projects.ReplaceMediaScopeSelection(ctx, projectID, nil); err != nil {
			return domain.Project{}, normalizeRepoErr(err)
		}
	}

	project, err = u.projects.GetByID(ctx, projectID)
	if err != nil {
		return domain.Project{}, normalizeRepoErr(err)
	}
	return project, nil
}

type QueueProjectRunsInput struct {
	Name           string
	BasePrompt     string
	VariantCount   int
	MinDurationSec int
	MaxDurationSec int
}

func (u *ProjectUsecase) QueueRuns(
	ctx context.Context,
	orgID, projectID, requestedBy uuid.UUID,
	in QueueProjectRunsInput,
) ([]domain.HighlightJob, error) {
	project, err := u.UpdateDraft(ctx, orgID, projectID, ProjectDraftInput{
		Name:           in.Name,
		BasePrompt:     in.BasePrompt,
		VariantCount:   in.VariantCount,
		MinDurationSec: in.MinDurationSec,
		MaxDurationSec: in.MaxDurationSec,
	})
	if err != nil {
		return nil, err
	}

	assets, err := u.media.ListByProjectSelection(ctx, projectID)
	if err != nil {
		return nil, normalizeRepoErr(err)
	}
	if len(assets) == 0 {
		return nil, ErrBadRequest.New("project has no selected media assets")
	}

	now := time.Now()
	jobs := make([]domain.HighlightJob, 0, project.EditorVariantCount)
	for i := 0; i < project.EditorVariantCount; i++ {
		jobs = append(jobs, domain.HighlightJob{
			ID:             uuid.New(),
			ProjectID:      projectID,
			RequestedBy:    requestedBy,
			Prompt:         project.EditorBasePrompt,
			VariantIndex:   i + 1,
			VariantCount:   project.EditorVariantCount,
			MinDurationSec: project.EditorMinDurationSec,
			MaxDurationSec: project.EditorMaxDurationSec,
			Status:         domain.JobQueued,
			CreatedAt:      now,
		})
	}

	if err := u.jobs.CreateMany(ctx, jobs); err != nil {
		return nil, normalizeRepoErr(err)
	}
	return jobs, nil
}

func (u *ProjectUsecase) ListRuns(ctx context.Context, orgID, projectID uuid.UUID) ([]domain.HighlightJob, error) {
	project, err := u.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, normalizeRepoErr(err)
	}
	if project.OrganizationID != orgID {
		return nil, ErrForbidden.New("forbidden")
	}
	return u.jobs.ListByProject(ctx, projectID)
}

func (u *ProjectUsecase) CancelRun(ctx context.Context, orgID, runID uuid.UUID) (domain.HighlightJob, error) {
	job, err := u.jobs.GetByID(ctx, runID)
	if err != nil {
		return domain.HighlightJob{}, normalizeRepoErr(err)
	}

	project, err := u.projects.GetByID(ctx, job.ProjectID)
	if err != nil {
		return domain.HighlightJob{}, normalizeRepoErr(err)
	}
	if project.OrganizationID != orgID {
		return domain.HighlightJob{}, ErrForbidden.New("forbidden")
	}

	if job.Status != domain.JobQueued && job.Status != domain.JobRunning {
		return domain.HighlightJob{}, ErrBadRequest.New("only queued or running jobs can be cancelled")
	}

	if err := u.jobs.UpdateStatus(ctx, runID, domain.JobCancelled); err != nil {
		return domain.HighlightJob{}, normalizeRepoErr(err)
	}
	job.Status = domain.JobCancelled
	return job, nil
}

type RunDetail struct {
	Job      domain.HighlightJob        `json:"job"`
	Timeline *domain.Timeline           `json:"timeline,omitempty"`
	Render   *domain.Render             `json:"render,omitempty"`
	Traces   []domain.HighlightJobTrace `json:"traces"`
}

func (u *ProjectUsecase) GetRun(ctx context.Context, orgID, runID uuid.UUID) (RunDetail, error) {
	job, err := u.jobs.GetByID(ctx, runID)
	if err != nil {
		return RunDetail{}, normalizeRepoErr(err)
	}

	project, err := u.projects.GetByID(ctx, job.ProjectID)
	if err != nil {
		return RunDetail{}, normalizeRepoErr(err)
	}
	if project.OrganizationID != orgID {
		return RunDetail{}, ErrForbidden.New("forbidden")
	}

	var timeline *domain.Timeline
	if item, err := u.timelines.GetByJobID(ctx, runID); err == nil {
		timeline = &item
	} else if err != postgres.ErrNotFound {
		return RunDetail{}, normalizeRepoErr(err)
	}

	var render *domain.Render
	if item, err := u.renders.GetByJobID(ctx, runID); err == nil {
		render = &item
	} else if err != postgres.ErrNotFound {
		return RunDetail{}, normalizeRepoErr(err)
	}

	traces, err := u.traces.ListByJobID(ctx, runID)
	if err != nil {
		return RunDetail{}, normalizeRepoErr(err)
	}

	return RunDetail{
		Job:      job,
		Timeline: timeline,
		Render:   render,
		Traces:   traces,
	}, nil
}

func (u *ProjectUsecase) OpenRenderStream(
	ctx context.Context,
	orgID, renderID uuid.UUID,
) (io.ReadCloser, int64, string, string, error) {
	render, err := u.renders.GetByID(ctx, renderID)
	if err != nil {
		return nil, 0, "", "", normalizeRepoErr(err)
	}

	job, err := u.jobs.GetByID(ctx, render.JobID)
	if err != nil {
		return nil, 0, "", "", normalizeRepoErr(err)
	}

	project, err := u.projects.GetByID(ctx, job.ProjectID)
	if err != nil {
		return nil, 0, "", "", normalizeRepoErr(err)
	}
	if project.OrganizationID != orgID {
		return nil, 0, "", "", ErrForbidden.New("forbidden")
	}

	cfg, err := u.storage.GetActiveByOrg(ctx, orgID)
	if err != nil {
		return nil, 0, "", "", normalizeRepoErr(err)
	}

	adapter, err := adapterStorage.NewAdapter(cfg)
	if err != nil {
		return nil, 0, "", "", err
	}

	rc, size, err := adapter.GetFile(ctx, render.OutputPath)
	if err != nil {
		return nil, 0, "", "", err
	}

	return rc, size, "video/mp4", "render.mp4", nil
}
