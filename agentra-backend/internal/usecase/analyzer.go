package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/adapter/ffmpeg"
	"github.com/reaganiwadha/agentra/internal/adapter/llm"
	adapterStorage "github.com/reaganiwadha/agentra/internal/adapter/storage"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
	"github.com/sirupsen/logrus"
)

type AnalyzerUsecase struct {
	orgs              *postgres.OrgRepo
	storage           *postgres.StorageRepo
	analyzers         *postgres.AnalyzerRepo
	assets            *postgres.MediaAssetRepo
	analysisResults   *postgres.MediaAnalysisResultRepo
	mediaEmbeddings   *postgres.MediaEmbeddingRepo
	llm               *llm.Client
	activity          *ActivityUsecase
	log               *logrus.Logger

	mu                sync.Mutex
	lastIdleEventAt   time.Time
	lastConfigEventAt time.Time
}

func NewAnalyzerUsecase(
	orgs *postgres.OrgRepo,
	storage *postgres.StorageRepo,
	analyzers *postgres.AnalyzerRepo,
	assets *postgres.MediaAssetRepo,
	analysisResults *postgres.MediaAnalysisResultRepo,
	mediaEmbeddings *postgres.MediaEmbeddingRepo,
	llmClient *llm.Client,
	activity *ActivityUsecase,
	log *logrus.Logger,
) *AnalyzerUsecase {
	return &AnalyzerUsecase{
		orgs:            orgs,
		storage:         storage,
		analyzers:       analyzers,
		assets:          assets,
		analysisResults: analysisResults,
		mediaEmbeddings: mediaEmbeddings,
		llm:             llmClient,
		activity:        activity,
		log:             log,
	}
}

func (u *AnalyzerUsecase) Run(ctx context.Context) error {
	org, err := u.orgs.First(ctx)
	if err != nil {
		return nil // no org yet
	}

	storageCfg, err := u.storage.GetActiveByOrg(ctx, org.ID)
	if err != nil {
		return nil // storage not configured
	}

	enabled, err := u.analyzers.ListEnabledWithProviders(ctx, org.ID)
	if err != nil {
		return err
	}
	if !u.validateAnalyzerPipeline(ctx, org.ID, enabled) {
		return nil
	}
	buckets := splitAnalyzers(enabled)

	adapter, err := adapterStorage.NewAdapter(storageCfg)
	if err != nil {
		return err
	}

	assets, err := u.selectAssetsNeedingAnalysis(ctx, org.ID, buckets, 5)
	if err != nil {
		return err
	}
	if len(assets) == 0 {
		u.emitAnalyzerIdle(ctx, org.ID)
		return nil
	}

	u.emitLoopStarted(ctx, org.ID, len(assets))

	processedCount := 0
	failedCount := 0
	for _, asset := range assets {
		log := u.log.WithField("asset_id", asset.ID).WithField("filename", asset.Filename)
		u.emitAssetProgress(ctx, org.ID, asset)
		if err := u.processAsset(ctx, org.ID, asset, adapter, buckets); err != nil {
			log.WithError(err).Error("analyzer: asset failed")
			failedCount++
			u.emitAssetFailed(ctx, org.ID, asset, err)
		} else {
			log.Info("analyzer: asset pass completed")
			processedCount++
			u.emitAssetCompleted(ctx, org.ID, asset)
		}

		fresh, _ := u.analysisResults.ListByMedia(ctx, asset.ID)
		freshIDs := resultIDSet(fresh)
		if !computeAssetNeeds(asset, freshIDs, fresh, buckets).Any() {
			_ = u.assets.UpdateStatus(ctx, asset.ID, domain.MediaReady)
		}
	}

	u.emitLoopCompleted(ctx, org.ID, processedCount, failedCount, len(assets))
	return nil
}

func (u *AnalyzerUsecase) emitLoopStarted(ctx context.Context, orgID uuid.UUID, count int) {
	_ = u.activity.Emit(ctx, ActivityEmitInput{
		OrganizationID: orgID,
		Source:         domain.ActivitySourceAnalyzerLoop,
		SubjectType:    "organization",
		EventType:      domain.ActivityEventStarted,
		Level:          "info",
		Message:        fmt.Sprintf("Analyzer loop started: processing %d analyzable asset(s).", count),
		Payload:        map[string]any{"asset_count": count},
	})
}

func (u *AnalyzerUsecase) emitAssetProgress(ctx context.Context, orgID uuid.UUID, asset domain.MediaAsset) {
	_ = u.activity.Emit(ctx, ActivityEmitInput{
		OrganizationID: orgID,
		Source:         domain.ActivitySourceAnalyzerLoop,
		SubjectType:    "media_asset",
		SubjectID:      asset.ID,
		EventType:      domain.ActivityEventProgress,
		Level:          "info",
		Message:        fmt.Sprintf("Analyzing asset %q.", asset.Filename),
		Payload:        map[string]any{"asset_id": asset.ID.String(), "filename": asset.Filename},
	})
}

func (u *AnalyzerUsecase) emitAssetFailed(ctx context.Context, orgID uuid.UUID, asset domain.MediaAsset, err error) {
	_ = u.activity.Emit(ctx, ActivityEmitInput{
		OrganizationID: orgID,
		Source:         domain.ActivitySourceAnalyzerLoop,
		SubjectType:    "media_asset",
		SubjectID:      asset.ID,
		EventType:      domain.ActivityEventFailed,
		Level:          "error",
		Message:        fmt.Sprintf("Analyzer failed for %q.", asset.Filename),
		Payload:        map[string]any{"asset_id": asset.ID.String(), "filename": asset.Filename, "error": err.Error()},
	})
}

func (u *AnalyzerUsecase) emitAssetCompleted(ctx context.Context, orgID uuid.UUID, asset domain.MediaAsset) {
	_ = u.activity.Emit(ctx, ActivityEmitInput{
		OrganizationID: orgID,
		Source:         domain.ActivitySourceAnalyzerLoop,
		SubjectType:    "media_asset",
		SubjectID:      asset.ID,
		EventType:      domain.ActivityEventCompleted,
		Level:          "info",
		Message:        fmt.Sprintf("Analyzer pass completed for %q.", asset.Filename),
		Payload:        map[string]any{"asset_id": asset.ID.String(), "filename": asset.Filename},
	})
}

func (u *AnalyzerUsecase) emitLoopCompleted(ctx context.Context, orgID uuid.UUID, processed, failed, total int) {
	_ = u.activity.Emit(ctx, ActivityEmitInput{
		OrganizationID: orgID,
		Source:         domain.ActivitySourceAnalyzerLoop,
		SubjectType:    "organization",
		EventType:      domain.ActivityEventCompleted,
		Level:          "info",
		Message:        "Analyzer loop completed.",
		Payload:        map[string]any{"processed": processed, "failed": failed, "total": total},
	})
}

func (u *AnalyzerUsecase) validateAnalyzerPipeline(ctx context.Context, orgID uuid.UUID, enabled []domain.EnabledAnalyzer) bool {
	if len(enabled) == 0 {
		u.log.Errorf("Analyzer loop disabled: no enabled analyzers with active providers.")
		u.emitConfigIssue(ctx, orgID)
		return false
	}
	return true
}

func (u *AnalyzerUsecase) emitStepProgress(ctx context.Context, orgID uuid.UUID, asset domain.MediaAsset, message string) {
	_ = u.activity.Emit(ctx, ActivityEmitInput{
		OrganizationID: orgID,
		Source:         domain.ActivitySourceAnalyzerLoop,
		SubjectType:    "media_asset",
		SubjectID:      asset.ID,
		EventType:      domain.ActivityEventProgress,
		Level:          "info",
		Message:        message,
		Payload:        map[string]any{"asset_id": asset.ID.String(), "filename": asset.Filename},
	})
}

func (u *AnalyzerUsecase) emitAnalyzerIdle(ctx context.Context, orgID uuid.UUID) {
	inactive := false
	_ = u.activity.Emit(ctx, ActivityEmitInput{
		OrganizationID: orgID,
		Source:         domain.ActivitySourceAnalyzerLoop,
		SubjectType:    "organization",
		EventType:      domain.ActivityEventInfo,
		Level:          "info",
		Message:        "Analyzer idle: no analyzable media assets.",
		Payload:        map[string]any{"pending": 0},
		IsActive:       &inactive,
	})
}

func (u *AnalyzerUsecase) emitConfigIssue(ctx context.Context, orgID uuid.UUID) {
	u.mu.Lock()
	shouldEmit := time.Since(u.lastConfigEventAt) >= 2*time.Minute
	if shouldEmit {
		u.lastConfigEventAt = time.Now()
	}
	u.mu.Unlock()
	if !shouldEmit {
		return
	}

	inactive := false
	_ = u.activity.Emit(ctx, ActivityEmitInput{
		OrganizationID: orgID,
		Source:         domain.ActivitySourceAnalyzerLoop,
		SubjectType:    "organization",
		EventType:      domain.ActivityEventInfo,
		Level:          "warn",
		Message:        "Analyzer loop disabled: no enabled analyzers available.",
		Payload:        map[string]any{"requires_enabled_analyzer": true},
		IsActive:       &inactive,
	})
}

type analyzerBuckets struct {
	transcription []domain.EnabledAnalyzer
	vision        []domain.EnabledAnalyzer
	embedding     []domain.EnabledAnalyzer
}

type assetNeeds struct {
	transcription bool
	vision        bool
	embedding     bool
}

func (n assetNeeds) Any() bool {
	return n.transcription || n.vision || n.embedding
}

func splitAnalyzers(enabled []domain.EnabledAnalyzer) analyzerBuckets {
	out := analyzerBuckets{
		transcription: make([]domain.EnabledAnalyzer, 0),
		vision:        make([]domain.EnabledAnalyzer, 0),
		embedding:     make([]domain.EnabledAnalyzer, 0),
	}
	for _, a := range enabled {
		switch a.AnalyzerType {
		case domain.AnalyzerTranscription:
			out.transcription = append(out.transcription, a)
		case domain.AnalyzerVisionTags:
			out.vision = append(out.vision, a)
		case domain.AnalyzerEmbedding:
			out.embedding = append(out.embedding, a)
		}
	}
	return out
}

func resultIDSet(results []domain.MediaAnalysisResult) map[uuid.UUID]struct{} {
	m := make(map[uuid.UUID]struct{}, len(results))
	for _, r := range results {
		m[r.AnalyzerID] = struct{}{}
	}
	return m
}

func anyMissingIn(analyzers []domain.EnabledAnalyzer, existingIDs map[uuid.UUID]struct{}) bool {
	for _, a := range analyzers {
		if _, ok := existingIDs[a.ID]; !ok {
			return true
		}
	}
	return false
}

func anyPresentIn(analyzers []domain.EnabledAnalyzer, existingIDs map[uuid.UUID]struct{}) bool {
	for _, a := range analyzers {
		if _, ok := existingIDs[a.ID]; ok {
			return true
		}
	}
	return false
}

func hasResultOfType(results []domain.MediaAnalysisResult, analyzerType string) bool {
	for _, r := range results {
		if r.AnalyzerType == analyzerType {
			return true
		}
	}
	return false
}

func (u *AnalyzerUsecase) selectAssetsNeedingAnalysis(
	ctx context.Context,
	orgID uuid.UUID,
	buckets analyzerBuckets,
	limit int,
) ([]domain.MediaAsset, error) {
	allAssets, err := u.assets.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}

	selected := make([]domain.MediaAsset, 0, limit)
	for _, asset := range allAssets {
		existing, err := u.analysisResults.ListByMedia(ctx, asset.ID)
		if err != nil {
			return nil, err
		}
		existingIDs := resultIDSet(existing)

		needs := computeAssetNeeds(asset, existingIDs, existing, buckets)
		if !needs.Any() {
			continue
		}

		selected = append(selected, asset)
		if len(selected) >= limit {
			break
		}
	}
	return selected, nil
}

func computeAssetNeeds(asset domain.MediaAsset, existingIDs map[uuid.UUID]struct{}, existingResults []domain.MediaAnalysisResult, buckets analyzerBuckets) assetNeeds {
	canTranscribe := isTranscriptionCompatible(asset)
	canVision := isVisionCompatible(asset)

	hasSignal := hasResultOfType(existingResults, string(domain.AnalyzerTranscription)) ||
		hasResultOfType(existingResults, string(domain.AnalyzerVisionTags))
	canProduceSignal := (canTranscribe && anyMissingIn(buckets.transcription, existingIDs)) ||
		(canVision && anyMissingIn(buckets.vision, existingIDs))
	canEmbedNowOrLater := hasSignal || canProduceSignal

	return assetNeeds{
		transcription: canTranscribe && anyMissingIn(buckets.transcription, existingIDs),
		vision:        canVision && anyMissingIn(buckets.vision, existingIDs),
		embedding:     canEmbedNowOrLater && anyMissingIn(buckets.embedding, existingIDs),
	}
}

func (u *AnalyzerUsecase) processAsset(
	ctx context.Context,
	orgID uuid.UUID,
	asset domain.MediaAsset,
	adapter adapterStorage.Adapter,
	buckets analyzerBuckets,
) error {
	existingResults, err := u.analysisResults.ListByMedia(ctx, asset.ID)
	if err != nil {
		return err
	}
	existingIDs := resultIDSet(existingResults)

	needs := computeAssetNeeds(asset, existingIDs, existingResults, buckets)
	if !needs.Any() {
		return nil
	}

	var tmpPath string
	if needs.transcription || needs.vision {
		u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Downloading %q...", asset.Filename))
		rc, _, err := adapter.GetFile(ctx, asset.StoragePath)
		if err != nil {
			return fmt.Errorf("get file: %w", err)
		}

		tmp, err := os.CreateTemp("", "agentra-asset-*")
		if err != nil {
			rc.Close()
			return err
		}
		tmpPath = tmp.Name()
		defer os.Remove(tmpPath)

		h := sha256.New()
		if _, err := io.Copy(io.MultiWriter(tmp, h), rc); err != nil {
			rc.Close()
			tmp.Close()
			return fmt.Errorf("download: %w", err)
		}
		rc.Close()
		tmp.Close()

		checksum := fmt.Sprintf("%x", h.Sum(nil))
		_ = u.assets.UpdateSHA256(ctx, asset.ID, checksum)
	}

	failures := make([]string, 0)
	var freshTranscriptResult *llm.TranscriptResult
	var freshVisionSegmented *llm.VisionSegmentedResult

	if needs.transcription {
		u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Extracting audio from %q...", asset.Filename))
		audioPath, err := ffmpeg.ExtractAudio(ctx, tmpPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("extract audio: %v", err))
		} else {
			defer os.Remove(audioPath)
			for _, a := range buckets.transcription {
				if _, ok := existingIDs[a.ID]; ok {
					continue
				}
				u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Transcribing %q with %q...", asset.Filename, a.Name))
				result, err := u.llm.Transcribe(ctx, modelFromAnalyzer(a), audioPath)
				if err != nil {
					u.log.WithError(err).WithField("analyzer", a.Name).Warn("transcription analyzer failed")
					failures = append(failures, fmt.Sprintf("transcription %q: %v", a.Name, err))
					continue
				}
				b, _ := json.Marshal(result)
				if err := u.analysisResults.Upsert(ctx, domain.MediaAnalysisResult{
					ID:           uuid.New(),
					MediaID:      asset.ID,
					AnalyzerID:   a.ID,
					AnalyzerName: a.Name,
					AnalyzerType: string(domain.AnalyzerTranscription),
					Output:       b,
					AnalyzedAt:   time.Now().UTC(),
				}); err != nil {
					failures = append(failures, fmt.Sprintf("save transcription %q: %v", a.Name, err))
				}
				freshTranscriptResult = &result
				u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Transcription ready for %q.", asset.Filename))
			}
		}
	}

	if needs.vision {
		u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Extracting keyframes from %q...", asset.Filename))
		framesDir, frames, err := ffmpeg.ExtractKeyframesWithTimestamps(ctx, tmpPath, 30)
		if err != nil {
			u.log.WithError(err).WithField("asset_id", asset.ID).Warn("extract keyframes failed; continuing without vision frames")
			failures = append(failures, fmt.Sprintf("extract keyframes: %v", err))
		} else {
			defer os.RemoveAll(framesDir)
			for _, a := range buckets.vision {
				if _, ok := existingIDs[a.ID]; ok {
					continue
				}

				u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Analyzing %d keyframe(s) from %q with %q...", len(frames), asset.Filename, a.Name))
				segments := make([]llm.VisionSegment, 0, len(frames))
				for _, frame := range frames {
					storagePath := fmt.Sprintf("_agentra/thumbnails/%s/frame_%04d.jpg", asset.ID, frame.FrameNumber)
					thumbnailPath := ""

					frameFile, err := os.Open(frame.Path)
					if err != nil {
						u.log.WithError(err).Warnf("vision: open frame %d", frame.FrameNumber)
					} else {
						frameInfo, statErr := frameFile.Stat()
						if statErr == nil {
							if uploadErr := adapter.WriteFile(ctx, storagePath, frameFile, frameInfo.Size()); uploadErr != nil {
								u.log.WithError(uploadErr).Warnf("vision: upload frame %d", frame.FrameNumber)
							} else {
								thumbnailPath = storagePath
							}
						}
						frameFile.Close()
					}

					u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Analyzing frame %d/%d at %.1fs for %q...", frame.FrameNumber, len(frames), frame.TimestampSec, asset.Filename))
					seg, err := u.llm.AnalyzeSingleFrame(ctx, modelFromAnalyzer(a), frame.Path, frame.FrameNumber, frame.TimestampSec)
					if err != nil {
						u.log.WithError(err).Warnf("vision frame %d analyze failed", frame.FrameNumber)
						u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Frame %d/%d failed: %v", frame.FrameNumber, len(frames), err))
						continue
					}
					seg.ThumbnailStoragePath = thumbnailPath
					segments = append(segments, seg)
				}

				if len(segments) == 0 {
					failures = append(failures, fmt.Sprintf("vision %q: all frames failed", a.Name))
					continue
				}

				summary, err := u.llm.SummarizeVisionSegments(ctx, modelFromAnalyzer(a), segments)
				if err != nil {
					u.log.WithError(err).WithField("analyzer", a.Name).Warn("vision summarize failed")
				}

				result := llm.VisionSegmentedResult{
					Segments: segments,
					Summary:  summary,
				}
				u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Vision analysis ready for %q (%d frame(s) analyzed).", asset.Filename, len(segments)))
				b, _ := json.Marshal(result)
				if err := u.analysisResults.Upsert(ctx, domain.MediaAnalysisResult{
					ID:           uuid.New(),
					MediaID:      asset.ID,
					AnalyzerID:   a.ID,
					AnalyzerName: a.Name,
					AnalyzerType: string(domain.AnalyzerVisionTags),
					Output:       b,
					AnalyzedAt:   time.Now().UTC(),
				}); err != nil {
					failures = append(failures, fmt.Sprintf("save vision %q: %v", a.Name, err))
				}
				freshVisionSegmented = &result
			}
		}
	}

	if needs.embedding {
		transcriptForEmbed := freshTranscriptResult
		if transcriptForEmbed == nil {
			for _, r := range existingResults {
				if r.AnalyzerType == string(domain.AnalyzerTranscription) {
					transcriptForEmbed = decodeTranscript(r.Output)
					break
				}
			}
		}
		visionForEmbed := freshVisionSegmented
		if visionForEmbed == nil {
			for _, r := range existingResults {
				if r.AnalyzerType == string(domain.AnalyzerVisionTags) {
					visionForEmbed = decodeVisionSegmented(r.Output)
					break
				}
			}
		}

		embInput := buildEmbeddingInput(transcriptForEmbed, visionForEmbed)
		if strings.TrimSpace(embInput) == "" {
			failures = append(failures, "embedding skipped: no transcript or vision signal available yet")
		} else {
			for _, a := range buckets.embedding {
				if _, ok := existingIDs[a.ID]; ok {
					continue
				}
				cfg := modelFromAnalyzer(a)

				// Whole-asset embedding
				u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Computing whole-asset embedding for %q...", asset.Filename))
				wholeVec, err := u.llm.Embed(ctx, cfg, embInput)
				if err != nil {
					u.log.WithError(err).WithField("analyzer", a.Name).Warn("embedding analyzer failed")
					failures = append(failures, fmt.Sprintf("embedding %q: %v", a.Name, err))
					continue
				}

				b, _ := json.Marshal(wholeVec)
				if err := u.analysisResults.Upsert(ctx, domain.MediaAnalysisResult{
					ID:           uuid.New(),
					MediaID:      asset.ID,
					AnalyzerID:   a.ID,
					AnalyzerName: a.Name,
					AnalyzerType: string(domain.AnalyzerEmbedding),
					Output:       b,
					AnalyzedAt:   time.Now().UTC(),
				}); err != nil {
					failures = append(failures, fmt.Sprintf("save embedding result %q: %v", a.Name, err))
				}

				if err := u.mediaEmbeddings.Upsert(ctx, domain.MediaEmbedding{
					ID:         uuid.New(),
					MediaID:    asset.ID,
					AnalyzerID: a.ID,
					SourceText: embInput,
				}, wholeVec); err != nil {
					failures = append(failures, fmt.Sprintf("save whole-asset embedding %q: %v", a.Name, err))
				}

				// Per-segment embeddings from transcript
				if transcriptForEmbed != nil && len(transcriptForEmbed.Segments) > 0 {
					segs := transcriptForEmbed.Segments
					texts := make([]string, len(segs))
					for i, s := range segs {
						texts[i] = strings.TrimSpace(s.Text)
					}
					u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Computing %d segment embeddings for %q...", len(texts), asset.Filename))
					segVecs, err := u.llm.EmbedBatch(ctx, cfg, texts)
					if err != nil {
						u.log.WithError(err).WithField("analyzer", a.Name).Warn("segment embedding batch failed")
						failures = append(failures, fmt.Sprintf("segment embeddings %q: %v", a.Name, err))
					} else {
						for i, vec := range segVecs {
							if vec == nil || i >= len(segs) {
								continue
							}
							idx := i
							start := segs[i].Start
							end := segs[i].End
							if err := u.mediaEmbeddings.Upsert(ctx, domain.MediaEmbedding{
								ID:           uuid.New(),
								MediaID:      asset.ID,
								AnalyzerID:   a.ID,
								SegmentIndex: &idx,
								StartSec:     &start,
								EndSec:       &end,
								SourceText:   texts[i],
							}, vec); err != nil {
								u.log.WithError(err).WithField("segment", i).Warn("save segment embedding failed")
							}
						}
					}
				}

				u.emitStepProgress(ctx, orgID, asset, fmt.Sprintf("Embeddings ready for %q.", asset.Filename))
			}
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf(strings.Join(failures, "; "))
	}
	return nil
}

func modelFromAnalyzer(a domain.EnabledAnalyzer) llm.Config {
	return llm.Config{
		BaseURL:      a.BaseURL,
		APIKey:       a.APIKey.String,
		ModelName:    a.ModelName,
		ProviderType: a.ProviderType,
	}
}

func buildEmbeddingInput(transcript *llm.TranscriptResult, vision *llm.VisionSegmentedResult) string {
	parts := make([]string, 0, 2)
	if transcript != nil && strings.TrimSpace(transcript.Text) != "" {
		parts = append(parts, strings.TrimSpace(transcript.Text))
	}
	if vision != nil && strings.TrimSpace(vision.Summary) != "" {
		parts = append(parts, "Vision: "+strings.TrimSpace(vision.Summary))
	}
	return strings.Join(parts, "\n")
}

func rawHasContent(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false
	}
	return true
}

func decodeTranscript(raw json.RawMessage) *llm.TranscriptResult {
	if !rawHasContent(raw) {
		return nil
	}
	var out llm.TranscriptResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	if strings.TrimSpace(out.Text) == "" && len(out.Segments) == 0 {
		return nil
	}
	return &out
}

func decodeVisionSegmented(raw json.RawMessage) *llm.VisionSegmentedResult {
	if !rawHasContent(raw) {
		return nil
	}
	var segmented llm.VisionSegmentedResult
	if err := json.Unmarshal(raw, &segmented); err == nil {
		if len(segmented.Segments) > 0 || strings.TrimSpace(segmented.Summary) != "" {
			return &segmented
		}
	}
	// Fall back to old VisionResult format
	var old llm.VisionResult
	if err := json.Unmarshal(raw, &old); err == nil {
		if strings.TrimSpace(old.Description) != "" || len(old.Tags) > 0 {
			return &llm.VisionSegmentedResult{Summary: old.Description}
		}
	}
	return nil
}

func isTranscriptionCompatible(asset domain.MediaAsset) bool {
	mime := strings.ToLower(strings.TrimSpace(asset.MIMEType.String))
	if strings.HasPrefix(mime, "audio/") || strings.HasPrefix(mime, "video/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(asset.Filename)) {
	case ".mov", ".mp4", ".wav", ".mp3":
		return true
	default:
		return false
	}
}

func isVisionCompatible(asset domain.MediaAsset) bool {
	mime := strings.ToLower(strings.TrimSpace(asset.MIMEType.String))
	if strings.HasPrefix(mime, "image/") || strings.HasPrefix(mime, "video/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(asset.Filename)) {
	case ".mov", ".mp4", ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}
