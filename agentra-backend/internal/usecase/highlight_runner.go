package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/adapter/llm"
	adapterStorage "github.com/reaganiwadha/agentra/internal/adapter/storage"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
	"github.com/reaganiwadha/agentra/rageditor"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v4"
)

type HighlightRunnerUsecase struct {
	projects   *postgres.ProjectRepo
	media      *postgres.MediaAssetRepo
	mediaUcase *MediaUsecase
	jobs       *postgres.HighlightJobRepo
	timelines  *postgres.TimelineRepo
	renders    *postgres.RenderRepo
	traces     *postgres.HighlightJobTraceRepo
	storage    *postgres.StorageRepo
	editor     *postgres.EditorRepo
	providers  *postgres.ProviderRepo
	llm        *llm.Client
	log        *logrus.Logger
}

func NewHighlightRunnerUsecase(
	projects *postgres.ProjectRepo,
	media *postgres.MediaAssetRepo,
	mediaUcase *MediaUsecase,
	jobs *postgres.HighlightJobRepo,
	timelines *postgres.TimelineRepo,
	renders *postgres.RenderRepo,
	traces *postgres.HighlightJobTraceRepo,
	storage *postgres.StorageRepo,
	editor *postgres.EditorRepo,
	providers *postgres.ProviderRepo,
	llmClient *llm.Client,
	log *logrus.Logger,
) *HighlightRunnerUsecase {
	return &HighlightRunnerUsecase{
		projects:   projects,
		media:      media,
		mediaUcase: mediaUcase,
		jobs:       jobs,
		timelines:  timelines,
		renders:    renders,
		traces:     traces,
		storage:    storage,
		editor:     editor,
		providers:  providers,
		llm:        llmClient,
		log:        log,
	}
}

func (u *HighlightRunnerUsecase) Run(ctx context.Context) error {
	queued, err := u.jobs.ListQueued(ctx, 8)
	if err != nil {
		return err
	}

	for _, job := range queued {
		processed, err := u.tryProcessJob(ctx, job)
		if err != nil {
			u.log.WithError(err).WithField("job_id", job.ID).Error("highlight runner job error")
			continue
		}
		if processed {
			return nil
		}
	}

	return nil
}

func (u *HighlightRunnerUsecase) tryProcessJob(ctx context.Context, job domain.HighlightJob) (bool, error) {
	project, err := u.projects.GetByID(ctx, job.ProjectID)
	if err != nil {
		return false, err
	}

	assets, err := u.listProjectAssets(ctx, project)
	if err != nil {
		return false, err
	}
	if len(assets) == 0 {
		if err := u.jobs.MarkFailed(ctx, job.ID, "project has no scoped media assets", time.Now()); err != nil {
			return false, err
		}
		return true, nil
	}

	for _, asset := range assets {
		switch asset.Status {
		case domain.MediaPending:
			return false, nil
		case domain.MediaReady:
		default:
			if err := u.jobs.MarkFailed(ctx, job.ID, fmt.Sprintf("media asset %q has unsupported status %q", asset.Filename, asset.Status), time.Now()); err != nil {
				return false, err
			}
			return true, nil
		}
	}

	startedAt := time.Now()
	marked, err := u.jobs.MarkRunning(ctx, job.ID, startedAt)
	if err != nil {
		return false, err
	}
	if !marked {
		return false, nil
	}

	if err := u.persistTimelinePlan(ctx, project, job, assets); err != nil {
		_ = u.jobs.MarkFailed(ctx, job.ID, err.Error(), time.Now())
		return true, err
	}

	runTraces, err := u.runEditor(ctx, project, job, assets)
	if traceErr := u.traces.CreateMany(ctx, runTraces); traceErr != nil {
		u.log.WithError(traceErr).WithField("job_id", job.ID).Error("highlight runner trace persistence error")
	}
	if err != nil {
		_ = u.jobs.MarkFailed(ctx, job.ID, err.Error(), time.Now())
		return true, err
	}

	if err := u.jobs.MarkCompleted(ctx, job.ID, time.Now()); err != nil {
		return true, err
	}
	return true, nil
}

func (u *HighlightRunnerUsecase) persistTimelinePlan(ctx context.Context, project domain.Project, job domain.HighlightJob, assets []domain.MediaAsset) error {
	type assetPlan struct {
		ID          uuid.UUID `json:"id"`
		Filename    string    `json:"filename"`
		StoragePath string    `json:"storage_path"`
		Status      string    `json:"status"`
		DurationSec *float64  `json:"duration_sec,omitempty"`
	}

	assetPlans := make([]assetPlan, 0, len(assets))
	for _, asset := range assets {
		var duration *float64
		if asset.DurationSec.Valid {
			d := asset.DurationSec.Float64
			duration = &d
		}
		assetPlans = append(assetPlans, assetPlan{
			ID:          asset.ID,
			Filename:    asset.Filename,
			StoragePath: asset.StoragePath,
			Status:      string(asset.Status),
			DurationSec: duration,
		})
	}

	plan := map[string]any{
		"format": "agentra.highlight_plan.v1",
		"project": map[string]any{
			"id":   project.ID,
			"name": project.Name,
		},
		"job": map[string]any{
			"id":               job.ID,
			"variant_index":    job.VariantIndex,
			"variant_count":    job.VariantCount,
			"prompt":           job.Prompt,
			"min_duration_sec": job.MinDurationSec,
			"max_duration_sec": job.MaxDurationSec,
		},
		"assets": assetPlans,
	}

	payload, err := json.Marshal(plan)
	if err != nil {
		return err
	}

	return u.timelines.Create(ctx, domain.Timeline{
		ID:        uuid.New(),
		JobID:     job.ID,
		OTIOJson:  payload,
		CreatedAt: time.Now(),
	})
}

func (u *HighlightRunnerUsecase) runEditor(ctx context.Context, project domain.Project, job domain.HighlightJob, assets []domain.MediaAsset) ([]domain.HighlightJobTrace, error) {
	traces := make([]domain.HighlightJobTrace, 0, 12)

	modelConfig, providerName, basePrompt, err := u.getEditorConfig(ctx, project)
	if err != nil {
		u.log.WithError(err).Warn("editor config incomplete or failed; continuing with empty model to trigger fallback")
		modelConfig = rageditor.ModelConfig{}
	}

	deps := rageditor.LoopDeps{
		ListProjectMoments: func(ctx context.Context, page int, pageSize int) ([]rageditor.ProjectMomentResult, int, error) {
			moments, total, listErr := u.mediaUcase.ListProjectMoments(ctx, project.OrganizationID, project.ID, page, pageSize)
			if listErr != nil {
				return nil, 0, listErr
			}
			res := make([]rageditor.ProjectMomentResult, 0, len(moments))
			for _, m := range moments {
				res = append(res, rageditor.ProjectMomentResult{
					MediaID:     m.MediaID.String(),
					Filename:    m.Filename,
					StartSec:    m.StartSec,
					EndSec:      m.EndSec,
					Score:       m.Score,
					MatchedText: m.MatchedText,
					ContextText: m.ContextText,
				})
			}
			return res, total, nil
		},
		ListProjectAssets: func(ctx context.Context) ([]rageditor.ProjectAssetSummary, error) {
			assets, listErr := u.listProjectAssets(ctx, project)
			if listErr != nil {
				return nil, listErr
			}
			out := make([]rageditor.ProjectAssetSummary, 0, len(assets))
			for _, asset := range assets {
				summary := asset.Filename
				detail, detailErr := u.mediaUcase.GetDetail(ctx, project.OrganizationID, asset.ID)
				if detailErr == nil {
					if candidate := summarizeMediaDetail(detail); strings.TrimSpace(candidate) != "" {
						summary = candidate
					}
				}
				var duration *float64
				if asset.DurationSec.Valid {
					d := asset.DurationSec.Float64
					duration = &d
				}
				out = append(out, rageditor.ProjectAssetSummary{
					MediaID:     asset.ID.String(),
					Filename:    asset.Filename,
					DurationSec: duration,
					Summary:     truncateSummary(summary, 220),
				})
			}
			return out, nil
		},
	}

	req := rageditor.RunRequest{
		ProjectID:      project.ID.String(),
		JobID:          job.ID.String(),
		VariantIndex:   job.VariantIndex,
		VariantCount:   job.VariantCount,
		Prompt:         job.Prompt,
		MinDurationSec: job.MinDurationSec,
		MaxDurationSec: job.MaxDurationSec,
		ProviderName:   providerName,
		Model:          modelConfig,
		BasePrompt:     basePrompt,
	}

	runResult, loopTraces, err := rageditor.RunTimelineLoop(ctx, req, deps)
	traces = append(traces, loopTraces...)
	if err != nil {
		return traces, err
	}

	if !runResult.RenderRequested || runResult.Render == nil || runResult.Timeline == nil {
		traces = append(traces, newTrace(job.ID, "render.skipped", "Render was not requested or timeline is invalid.", map[string]any{
			"termination_reason": runResult.TerminationReason,
		}))
		return traces, nil
	}

	cfg, err := u.storage.GetActiveByOrg(ctx, project.OrganizationID)
	if err != nil {
		return traces, err
	}
	adapter, err := adapterStorage.NewAdapter(cfg)
	if err != nil {
		return traces, err
	}

	// Download required media files
	localMediaPaths := make(map[string]string)
	assetMap := make(map[string]domain.MediaAsset)
	for _, a := range assets {
		assetMap[a.ID.String()] = a
	}

	for _, track := range runResult.Timeline.Tracks {
		for _, clip := range track.Clips {
			if _, ok := localMediaPaths[clip.MediaID]; ok {
				continue
			}
			asset, ok := assetMap[clip.MediaID]
			if !ok {
				continue
			}

			rc, _, getErr := adapter.GetFile(ctx, asset.StoragePath)
			if getErr != nil {
				u.log.WithError(getErr).WithField("media_id", clip.MediaID).Warn("failed to get media file for timeline")
				continue
			}
			tmpFile, createErr := os.CreateTemp("", "agentra-render-src-*.mp4")
			if createErr != nil {
				rc.Close()
				continue
			}
			_, copyErr := io.Copy(tmpFile, rc)
			tmpFile.Close()
			rc.Close()

			if copyErr == nil {
				localMediaPaths[clip.MediaID] = tmpFile.Name()
			}
		}
	}
	defer func() {
		for _, path := range localMediaPaths {
			os.Remove(path)
		}
	}()

	localOutputPath := filepath.Join(os.TempDir(), fmt.Sprintf("agentra-highlight-%s.mp4", job.ID.String()))
	result, executionTraces, err := rageditor.RenderTimeline(ctx, *runResult.Timeline, localOutputPath, localMediaPaths)
	traces = append(traces, executionTraces...)
	if err != nil {
		return traces, fmt.Errorf("rageditor generate highlight: %w", err)
	}

	localPath := result.OutputPath
	if strings.TrimSpace(localPath) == "" {
		localPath = localOutputPath
	}

	info, statErr := os.Stat(localPath)
	if statErr != nil {
		u.log.WithFields(logrus.Fields{
			"job_id":            job.ID,
			"project_id":        project.ID,
			"min_duration_sec":  job.MinDurationSec,
			"max_duration_sec":  job.MaxDurationSec,
			"reported_duration": result.DurationSec,
		}).Warn("highlight runner completed without a materialized output file")
		traces = append(traces, newTrace(job.ID, "render.missing", "Tool completed but no output file was materialized.", map[string]any{
			"output_path": result.OutputPath,
			"renderer":    result.Renderer,
		}))
		return traces, nil
	}

	f, err := os.Open(localPath)
	if err != nil {
		return traces, err
	}
	defer f.Close()

	outputPath := joinStoragePath(
		cfg.OutputBasePath.String,
		"projects",
		project.ID.String(),
		"runs",
		job.ID.String(),
		fmt.Sprintf("variant-%02d.mp4", job.VariantIndex),
	)
	if err := adapter.WriteFile(ctx, outputPath, f, info.Size()); err != nil {
		return traces, err
	}

	if err := u.renders.Create(ctx, domain.Render{
		ID:            uuid.New(),
		JobID:         job.ID,
		OutputPath:    outputPath,
		DurationSec:   null.FloatFrom(result.DurationSec),
		FileSizeBytes: null.IntFrom(info.Size()),
		CreatedAt:     time.Now(),
	}); err != nil {
		return traces, err
	}

	traces = append(traces, newTrace(job.ID, "render.stored", "Generated render stored in configured output storage.", map[string]any{
		"output_path":     outputPath,
		"duration_sec":    result.DurationSec,
		"file_size_bytes": info.Size(),
		"renderer":        result.Renderer,
	}))

	return traces, nil
}

func (u *HighlightRunnerUsecase) getEditorConfig(ctx context.Context, project domain.Project) (rageditor.ModelConfig, string, string, error) {
	cfg, err := u.editor.Get(ctx, project.OrganizationID)
	if err != nil {
		return rageditor.ModelConfig{}, "", "", err
	}
	if !cfg.ProviderID.Valid {
		return rageditor.ModelConfig{}, "", cfg.BasePrompt, fmt.Errorf("provider unset")
	}

	provider, err := u.providers.GetByID(ctx, project.OrganizationID, cfg.ProviderID.UUID)
	if err != nil {
		return rageditor.ModelConfig{}, "", cfg.BasePrompt, err
	}

	return rageditor.ModelConfig{
		BaseURL:      provider.BaseURL,
		APIKey:       provider.APIKey.String,
		ModelName:    cfg.ModelName,
		ProviderType: provider.ProviderType,
	}, provider.Name, cfg.BasePrompt, nil
}

func newTrace(jobID uuid.UUID, phase string, message string, payload map[string]any) domain.HighlightJobTrace {
	body, _ := json.Marshal(payload)
	return domain.HighlightJobTrace{
		ID:        uuid.New(),
		JobID:     jobID,
		Phase:     phase,
		Message:   message,
		Payload:   body,
		CreatedAt: time.Now(),
	}
}

func summarizeMediaDetail(detail MediaDetailResponse) string {
	parts := make([]string, 0, len(detail.AnalyzerResults))
	for _, item := range detail.AnalyzerResults {
		if item.Status != "ready" || len(item.Output) == 0 {
			continue
		}
		switch item.AnalyzerType {
		case string(domain.AnalyzerVisionTags):
			if text := firstJSONString(item.Output, "summary", "description"); text != "" {
				parts = append(parts, text)
			}
		case string(domain.AnalyzerTranscription):
			if text := firstJSONString(item.Output, "text"); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) >= 2 {
			break
		}
	}
	if len(parts) == 0 {
		return detail.Filename
	}
	return strings.Join(parts, " ")
}

func firstJSONString(raw json.RawMessage, keys ...string) string {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncateSummary(text string, maxLen int) string {
	text = strings.TrimSpace(text)
	if text == "" || maxLen <= 0 || len(text) <= maxLen {
		return text
	}
	if maxLen <= 3 {
		return text[:maxLen]
	}
	return strings.TrimSpace(text[:maxLen-3]) + "..."
}

func (u *HighlightRunnerUsecase) listProjectAssets(ctx context.Context, project domain.Project) ([]domain.MediaAsset, error) {
	switch project.MediaScopeMode {
	case domain.MediaScopeDateRange:
		return u.media.ListByOrgInDateRange(ctx, project.OrganizationID, nullTimePtr(project.MediaScopeStartAt), nullTimePtr(project.MediaScopeEndAt))
	case domain.MediaScopeSelected:
		return u.media.ListByProjectSelection(ctx, project.ID)
	default:
		return u.media.ListByOrg(ctx, project.OrganizationID)
	}
}
