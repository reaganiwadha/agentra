package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	drawpkg "image/draw"
	"image/jpeg"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/adapter/ffmpeg"
	"github.com/reaganiwadha/agentra/internal/adapter/llm"
	adapterStorage "github.com/reaganiwadha/agentra/internal/adapter/storage"
	"github.com/reaganiwadha/agentra/internal/domain"
	"github.com/reaganiwadha/agentra/internal/repository/postgres"
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v4"
)

type MediaUploadInput struct {
	Filename  string
	LocalPath string
}

type MediaUsecase struct {
	orgs            *postgres.OrgRepo
	projects        *postgres.ProjectRepo
	storage         *postgres.StorageRepo
	analyzers       *postgres.AnalyzerRepo
	media           *postgres.MediaAssetRepo
	analysisResults *postgres.MediaAnalysisResultRepo
	mediaEmbeddings *postgres.MediaEmbeddingRepo
	llm             *llm.Client
	log             *logrus.Logger
}

type AnalyzerResultItem struct {
	AnalyzerID   uuid.UUID       `json:"analyzer_id"`
	AnalyzerName string          `json:"analyzer_name"`
	AnalyzerType string          `json:"analyzer_type"`
	Status       string          `json:"status"`
	Output       json.RawMessage `json:"output,omitempty"`
	AnalyzedAt   *time.Time      `json:"analyzed_at,omitempty"`
}

type MediaDetailResponse struct {
	domain.MediaAsset
	AnalyzerResults []AnalyzerResultItem `json:"analyzer_results"`
}

func NewMediaUsecase(
	orgs *postgres.OrgRepo,
	projects *postgres.ProjectRepo,
	storage *postgres.StorageRepo,
	analyzers *postgres.AnalyzerRepo,
	media *postgres.MediaAssetRepo,
	analysisResults *postgres.MediaAnalysisResultRepo,
	mediaEmbeddings *postgres.MediaEmbeddingRepo,
	llmClient *llm.Client,
	log *logrus.Logger,
) *MediaUsecase {
	return &MediaUsecase{
		orgs:            orgs,
		projects:        projects,
		storage:         storage,
		analyzers:       analyzers,
		media:           media,
		analysisResults: analysisResults,
		mediaEmbeddings: mediaEmbeddings,
		llm:             llmClient,
		log:             log,
	}
}

func (u *MediaUsecase) GetDetail(ctx context.Context, orgID, mediaID uuid.UUID) (MediaDetailResponse, error) {
	asset, err := u.media.GetByIDForOrg(ctx, mediaID, orgID)
	if err != nil {
		return MediaDetailResponse{}, normalizeRepoErr(err)
	}

	resp := MediaDetailResponse{
		MediaAsset:      asset,
		AnalyzerResults: make([]AnalyzerResultItem, 0),
	}

	analyzerOrgID := orgID
	if org, orgErr := u.orgs.First(ctx); orgErr == nil {
		analyzerOrgID = org.ID
	}

	enabled, err := u.analyzers.ListEnabledWithProviders(ctx, analyzerOrgID)
	if err != nil {
		return resp, nil
	}

	results, err := u.analysisResults.ListByMedia(ctx, mediaID)
	if err != nil {
		return MediaDetailResponse{}, err
	}
	byAnalyzerID := make(map[uuid.UUID]domain.MediaAnalysisResult, len(results))
	for _, result := range results {
		byAnalyzerID[result.AnalyzerID] = result
	}

	for _, analyzer := range enabled {
		switch analyzer.AnalyzerType {
		case domain.AnalyzerTranscription:
			if !isTranscriptionCompatible(asset) {
				continue
			}
		case domain.AnalyzerVisionTags:
			if !isVisionCompatible(asset) {
				continue
			}
		}

		item := AnalyzerResultItem{
			AnalyzerID:   analyzer.ID,
			AnalyzerName: analyzer.Name,
			AnalyzerType: string(analyzer.AnalyzerType),
			Status:       "pending",
		}
		if result, ok := byAnalyzerID[analyzer.ID]; ok {
			item.Status = "ready"
			item.Output = result.Output
			analyzedAt := result.AnalyzedAt
			item.AnalyzedAt = &analyzedAt
		}
		resp.AnalyzerResults = append(resp.AnalyzerResults, item)
	}

	return resp, nil
}

func (u *MediaUsecase) ListByProject(ctx context.Context, orgID, projectID uuid.UUID) ([]domain.MediaAsset, error) {
	project, err := u.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, normalizeRepoErr(err)
	}
	if project.OrganizationID != orgID {
		return nil, ErrForbidden.New("forbidden")
	}

	switch project.MediaScopeMode {
	case domain.MediaScopeDateRange:
		assets, err := u.media.ListByOrgInDateRange(
			ctx,
			orgID,
			nullTimePtr(project.MediaScopeStartAt),
			nullTimePtr(project.MediaScopeEndAt),
		)
		if err != nil {
			return nil, normalizeRepoErr(err)
		}
		return assets, nil
	case domain.MediaScopeSelected:
		assets, err := u.media.ListByProjectSelection(ctx, projectID)
		if err != nil {
			return nil, normalizeRepoErr(err)
		}
		return assets, nil
	default:
		assets, err := u.media.ListByOrg(ctx, orgID)
		if err != nil {
			return nil, normalizeRepoErr(err)
		}
		return assets, nil
	}
}

func (u *MediaUsecase) Delete(ctx context.Context, orgID, mediaID uuid.UUID) error {
	asset, err := u.media.GetByIDForOrg(ctx, mediaID, orgID)
	if err != nil {
		return normalizeRepoErr(err)
	}

	if err := u.media.DeleteScopeItems(ctx, mediaID); err != nil {
		return err
	}

	if err := u.media.Delete(ctx, mediaID, orgID); err != nil {
		return normalizeRepoErr(err)
	}

	cfg, err := u.storage.GetActiveByOrg(ctx, orgID)
	if err != nil {
		return nil
	}
	adapter, err := adapterStorage.NewAdapter(cfg)
	if err != nil {
		return nil
	}
	if err := adapter.DeleteFile(ctx, asset.StoragePath); err != nil {
		u.log.WithError(err).Warn("failed to delete media file from storage")
	}
	if asset.ThumbnailPath.Valid && strings.TrimSpace(asset.ThumbnailPath.String) != "" {
		if err := adapter.DeleteFile(ctx, asset.ThumbnailPath.String); err != nil {
			u.log.WithError(err).Warn("failed to delete thumbnail from storage")
		}
	}
	return nil
}

func (u *MediaUsecase) ClearAll(ctx context.Context, orgID uuid.UUID) error {
	assets, err := u.media.ListByOrg(ctx, orgID)
	if err != nil {
		return normalizeRepoErr(err)
	}

	var hasErr error
	for _, asset := range assets {
		if err := u.Delete(ctx, orgID, asset.ID); err != nil {
			u.log.WithError(err).WithField("media_id", asset.ID).Warn("failed to delete media asset during clear all")
			hasErr = err
		}
	}
	return hasErr
}

func (u *MediaUsecase) ResetAnalysis(ctx context.Context, orgID, mediaID uuid.UUID) error {
	if _, err := u.media.GetByIDForOrg(ctx, mediaID, orgID); err != nil {
		return normalizeRepoErr(err)
	}
	if err := u.analysisResults.DeleteByMedia(ctx, mediaID); err != nil {
		return err
	}
	if err := u.mediaEmbeddings.DeleteByMedia(ctx, mediaID); err != nil {
		return err
	}
	return u.media.UpdateStatus(ctx, mediaID, domain.MediaPending)
}

type EmbeddingSearchInput struct {
	Query string
	Limit int
}

type EditorMomentQueryInput struct {
	Query           string
	Limit           int
	ContextSegments int
	MergeGapSec     float64
}

type EditorMoment struct {
	MediaID        uuid.UUID  `json:"media_id"`
	Filename       string     `json:"filename"`
	StoragePath    string     `json:"storage_path"`
	StartSec       float64    `json:"start_sec"`
	EndSec         float64    `json:"end_sec"`
	Score          float64    `json:"score"`
	MatchedText    string     `json:"matched_text"`
	ContextText    string     `json:"context_text"`
	SegmentIndexes []int      `json:"segment_indexes"`
	DurationSec    *float64   `json:"duration_sec,omitempty"`
	CapturedAt     *time.Time `json:"captured_at,omitempty"`
}

func (u *MediaUsecase) ListProjectMoments(
	ctx context.Context,
	orgID, projectID uuid.UUID,
	page, pageSize int,
) ([]EditorMoment, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}

	assets, err := u.ListByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, 0, err
	}

	out := make([]EditorMoment, 0, len(assets)*4)
	for _, asset := range assets {
		if asset.Status != domain.MediaReady {
			continue
		}
		results, listErr := u.analysisResults.ListByMedia(ctx, asset.ID)
		if listErr != nil {
			return nil, 0, listErr
		}
		out = append(out, collectAssetMoments(asset, results)...)
	}

	slices.SortFunc(out, func(a, b EditorMoment) int {
		if a.Filename != b.Filename {
			return strings.Compare(a.Filename, b.Filename)
		}
		if a.StartSec < b.StartSec {
			return -1
		}
		if a.StartSec > b.StartSec {
			return 1
		}
		if a.EndSec < b.EndSec {
			return -1
		}
		if a.EndSec > b.EndSec {
			return 1
		}
		return strings.Compare(a.MatchedText, b.MatchedText)
	})

	total := len(out)
	start := (page - 1) * pageSize
	if start >= total {
		return []EditorMoment{}, total, nil
	}
	end := min(total, start+pageSize)
	return out[start:end], total, nil
}

func (u *MediaUsecase) embedQuery(ctx context.Context, orgID uuid.UUID, query string) ([]float64, error) {
	analyzers, err := u.analyzers.ListEnabledWithProviders(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, a := range analyzers {
		if a.AnalyzerType == domain.AnalyzerEmbedding {
			cfg := llm.Config{
				BaseURL:      a.BaseURL,
				APIKey:       a.APIKey.String,
				ModelName:    a.ModelName,
				ProviderType: a.ProviderType,
			}
			vec, err := u.llm.Embed(ctx, cfg, query)
			if err != nil {
				return nil, fmt.Errorf("embed query: %w", err)
			}
			return vec, nil
		}
	}
	return nil, fmt.Errorf("no embedding analyzer configured")
}

func (u *MediaUsecase) SearchEmbeddings(ctx context.Context, orgID, projectID uuid.UUID, in EmbeddingSearchInput) ([]domain.EmbeddingSearchHit, error) {
	if strings.TrimSpace(in.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	queryVec, err := u.embedQuery(ctx, orgID, strings.TrimSpace(in.Query))
	if err != nil {
		return nil, err
	}

	assets, err := u.ListByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	mediaIDs := make([]uuid.UUID, 0, len(assets))
	for _, a := range assets {
		if a.Status == domain.MediaReady {
			mediaIDs = append(mediaIDs, a.ID)
		}
	}
	if len(mediaIDs) == 0 {
		return []domain.EmbeddingSearchHit{}, nil
	}
	return u.mediaEmbeddings.SearchByMediaIDs(ctx, mediaIDs, queryVec, limit)
}

func (u *MediaUsecase) SearchProjectMoments(
	ctx context.Context,
	orgID, projectID uuid.UUID,
	in EditorMomentQueryInput,
) ([]EditorMoment, error) {
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	limit := in.Limit
	if limit <= 0 || limit > 50 {
		limit = 12
	}

	contextSegments := in.ContextSegments
	if contextSegments < 0 || contextSegments > 6 {
		contextSegments = 1
	}

	mergeGapSec := in.MergeGapSec
	if mergeGapSec <= 0 || mergeGapSec > 10 {
		mergeGapSec = 2.5
	}

	rawHits, err := u.SearchEmbeddings(ctx, orgID, projectID, EmbeddingSearchInput{
		Query: query,
		Limit: limit * 4,
	})
	if err != nil {
		return nil, err
	}
	if len(rawHits) == 0 {
		return []EditorMoment{}, nil
	}

	assets, err := u.ListByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	assetByID := make(map[uuid.UUID]domain.MediaAsset, len(assets))
	for _, asset := range assets {
		assetByID[asset.ID] = asset
	}

	type draftMoment struct {
		EditorMoment
		matchTexts map[string]struct{}
	}

	out := make([]EditorMoment, 0, limit)
	byMedia := make(map[uuid.UUID][]*draftMoment)

	for _, hit := range rawHits {
		asset, ok := assetByID[hit.MediaID]
		if !ok {
			continue
		}

		candidate, ok := u.buildMomentCandidate(ctx, asset, hit, contextSegments)
		if !ok {
			continue
		}

		current := byMedia[asset.ID]
		merged := false
		for _, existing := range current {
			if candidate.StartSec <= existing.EndSec+mergeGapSec && candidate.EndSec >= existing.StartSec-mergeGapSec {
				if candidate.StartSec < existing.StartSec {
					existing.StartSec = candidate.StartSec
				}
				if candidate.EndSec > existing.EndSec {
					existing.EndSec = candidate.EndSec
				}
				if candidate.Score > existing.Score {
					existing.Score = candidate.Score
				}
				for _, idx := range candidate.SegmentIndexes {
					if !containsInt(existing.SegmentIndexes, idx) {
						existing.SegmentIndexes = append(existing.SegmentIndexes, idx)
					}
				}
				if strings.TrimSpace(existing.ContextText) == "" {
					existing.ContextText = candidate.ContextText
				}
				if strings.TrimSpace(existing.MatchedText) == "" {
					existing.MatchedText = candidate.MatchedText
				} else if _, seen := existing.matchTexts[candidate.MatchedText]; !seen && strings.TrimSpace(candidate.MatchedText) != "" {
					existing.matchTexts[candidate.MatchedText] = struct{}{}
					existing.MatchedText = strings.TrimSpace(existing.MatchedText + "\n" + candidate.MatchedText)
				}
				merged = true
				break
			}
		}
		if merged {
			continue
		}

		byMedia[asset.ID] = append(byMedia[asset.ID], &draftMoment{
			EditorMoment: candidate,
			matchTexts:   newMatchTextSet(candidate.MatchedText),
		})
	}

	for _, moments := range byMedia {
		for _, moment := range moments {
			moment.SegmentIndexes = uniqueSortedInts(moment.SegmentIndexes)
			out = append(out, moment.EditorMoment)
		}
	}

	slices.SortFunc(out, func(a, b EditorMoment) int {
		if a.Score > b.Score {
			return -1
		}
		if a.Score < b.Score {
			return 1
		}
		if a.StartSec < b.StartSec {
			return -1
		}
		if a.StartSec > b.StartSec {
			return 1
		}
		return strings.Compare(a.Filename, b.Filename)
	})

	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (u *MediaUsecase) SearchEmbeddingsOrg(ctx context.Context, orgID uuid.UUID, in EmbeddingSearchInput) ([]domain.EmbeddingSearchHit, error) {
	if strings.TrimSpace(in.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	limit := in.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	queryVec, err := u.embedQuery(ctx, orgID, strings.TrimSpace(in.Query))
	if err != nil {
		return nil, err
	}

	assets, err := u.media.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	mediaIDs := make([]uuid.UUID, 0, len(assets))
	for _, a := range assets {
		if a.Status == domain.MediaReady {
			mediaIDs = append(mediaIDs, a.ID)
		}
	}
	if len(mediaIDs) == 0 {
		return []domain.EmbeddingSearchHit{}, nil
	}
	return u.mediaEmbeddings.SearchByMediaIDs(ctx, mediaIDs, queryVec, limit)
}

func (u *MediaUsecase) buildMomentCandidate(
	ctx context.Context,
	asset domain.MediaAsset,
	hit domain.EmbeddingSearchHit,
	contextSegments int,
) (EditorMoment, bool) {
	// A moment must be a timed window inside an asset.
	// Whole-asset embedding hits belong to asset-level retrieval, not moment search.
	if hit.SegmentIndex == nil && hit.StartSec == nil && hit.EndSec == nil {
		return EditorMoment{}, false
	}

	base := EditorMoment{
		MediaID:     asset.ID,
		Filename:    asset.Filename,
		StoragePath: asset.StoragePath,
		Score:       hit.Score,
	}
	if asset.DurationSec.Valid {
		d := asset.DurationSec.Float64
		base.DurationSec = &d
	}
	if asset.CapturedAt.Valid {
		t := asset.CapturedAt.Time
		base.CapturedAt = &t
	}

	results, err := u.analysisResults.ListByMedia(ctx, asset.ID)
	if err != nil {
		return EditorMoment{}, false
	}

	var transcript *llm.TranscriptResult
	for _, result := range results {
		if result.AnalyzerType == string(domain.AnalyzerTranscription) {
			transcript = decodeTranscript(result.Output)
			if transcript != nil {
				break
			}
		}
	}

	if transcript != nil && len(transcript.Segments) > 0 {
		matchIndex := locateTranscriptSegment(transcript.Segments, hit)
		if matchIndex >= 0 {
			startIdx := max(0, matchIndex-contextSegments)
			endIdx := min(len(transcript.Segments)-1, matchIndex+contextSegments)
			base.StartSec = transcript.Segments[startIdx].Start
			base.EndSec = transcript.Segments[endIdx].End
			base.ContextText = joinTranscriptSegments(transcript.Segments[startIdx : endIdx+1])
			base.MatchedText = strings.TrimSpace(hit.SourceText)
			base.SegmentIndexes = []int{matchIndex}
			return base, true
		}
	}

	if hit.StartSec != nil && hit.EndSec != nil {
		base.StartSec = *hit.StartSec
		base.EndSec = *hit.EndSec
		base.MatchedText = strings.TrimSpace(hit.SourceText)
		base.ContextText = strings.TrimSpace(hit.SourceText)
		return base, true
	}

	return EditorMoment{}, false
}

func collectAssetMoments(asset domain.MediaAsset, results []domain.MediaAnalysisResult) []EditorMoment {
	out := make([]EditorMoment, 0, 8)
	base := EditorMoment{
		MediaID:     asset.ID,
		Filename:    asset.Filename,
		StoragePath: asset.StoragePath,
		Score:       1,
	}
	if asset.DurationSec.Valid {
		d := asset.DurationSec.Float64
		base.DurationSec = &d
	}
	if asset.CapturedAt.Valid {
		t := asset.CapturedAt.Time
		base.CapturedAt = &t
	}

	for _, result := range results {
		switch result.AnalyzerType {
		case string(domain.AnalyzerTranscription):
			transcript := decodeTranscript(result.Output)
			if transcript == nil {
				continue
			}
			for idx, seg := range transcript.Segments {
				text := strings.TrimSpace(seg.Text)
				if text == "" || seg.End <= seg.Start {
					continue
				}
				moment := base
				moment.StartSec = seg.Start
				moment.EndSec = seg.End
				moment.MatchedText = text
				moment.ContextText = text
				moment.SegmentIndexes = []int{idx}
				out = append(out, moment)
			}
		case string(domain.AnalyzerVisionTags):
			segmented := decodeVisionSegmented(result.Output)
			if segmented == nil {
				continue
			}
			for idx, seg := range segmented.Segments {
				text := strings.TrimSpace(seg.Description)
				if text == "" {
					continue
				}
				start := seg.TimestampSec
				end := seg.TimestampSec + 4
				if base.DurationSec != nil && end > *base.DurationSec {
					end = *base.DurationSec
				}
				if end <= start {
					continue
				}
				moment := base
				moment.StartSec = start
				moment.EndSec = end
				moment.MatchedText = text
				moment.ContextText = text
				moment.SegmentIndexes = []int{idx}
				out = append(out, moment)
			}
		}
	}

	return out
}

func locateTranscriptSegment(segments []llm.TranscriptSegment, hit domain.EmbeddingSearchHit) int {
	if len(segments) == 0 {
		return -1
	}
	if hit.SegmentIndex != nil && *hit.SegmentIndex >= 0 && *hit.SegmentIndex < len(segments) {
		return *hit.SegmentIndex
	}

	if hit.StartSec == nil && hit.EndSec == nil {
		source := strings.TrimSpace(hit.SourceText)
		if source == "" {
			return -1
		}
		for i, seg := range segments {
			if strings.Contains(strings.ToLower(seg.Text), strings.ToLower(source)) {
				return i
			}
		}
		return -1
	}

	bestIndex := -1
	bestDistance := 1e9
	for i, seg := range segments {
		distance := 0.0
		if hit.StartSec != nil {
			distance += absFloat(seg.Start - *hit.StartSec)
		}
		if hit.EndSec != nil {
			distance += absFloat(seg.End - *hit.EndSec)
		}
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = i
		}
	}
	return bestIndex
}

func joinTranscriptSegments(segments []llm.TranscriptSegment) string {
	parts := make([]string, 0, len(segments))
	for _, seg := range segments {
		text := strings.TrimSpace(seg.Text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

func newMatchTextSet(raw string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, part := range strings.Split(raw, "\n") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = struct{}{}
		}
	}
	return out
}

func uniqueSortedInts(in []int) []int {
	if len(in) == 0 {
		return in
	}
	seen := make(map[int]struct{}, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	slices.Sort(out)
	return out
}

func containsInt(items []int, target int) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (u *MediaUsecase) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.MediaAsset, error) {
	assets, err := u.media.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, normalizeRepoErr(err)
	}
	return assets, nil
}

func nullTimePtr(v null.Time) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func (u *MediaUsecase) Upload(ctx context.Context, orgID uuid.UUID, in MediaUploadInput) (domain.MediaAsset, error) {
	ext := strings.ToLower(path.Ext(in.Filename))
	if _, ok := allowedExtensions[ext]; !ok {
		return domain.MediaAsset{}, ErrBadRequest.New("unsupported file type")
	}

	cfg, err := u.storage.GetActiveByOrg(ctx, orgID)
	if err != nil {
		return domain.MediaAsset{}, normalizeRepoErr(err)
	}

	adapter, err := adapterStorage.NewAdapter(cfg)
	if err != nil {
		return domain.MediaAsset{}, err
	}

	fileID := uuid.New()
	safeName := sanitizeFilename(in.Filename)
	if safeName == "" {
		safeName = fileID.String() + ext
	}
	filename := fileID.String() + "-" + safeName

	mediaBase := joinStoragePath(cfg.BasePath.String)
	thumbBaseRoot := cfg.OutputBasePath.String
	if strings.TrimSpace(thumbBaseRoot) == "" {
		thumbBaseRoot = cfg.BasePath.String
	}
	thumbBase := joinStoragePath(thumbBaseRoot)

	mediaStoragePath := joinStoragePath(mediaBase, filename)
	thumbStoragePath := joinStoragePath(thumbBase, "thumb-"+fileID.String()+".jpg")

	mimeType, fileSize, checksum, err := fileFingerprint(in.LocalPath)
	if err != nil {
		return domain.MediaAsset{}, err
	}

	capturedAt, durationSec := probeMediaMetadata(ctx, in.LocalPath)
	if capturedAt == nil {
		now := time.Now()
		capturedAt = &now
	}

	originalFile, err := os.Open(in.LocalPath)
	if err != nil {
		return domain.MediaAsset{}, err
	}
	defer originalFile.Close()

	if err := adapter.WriteFile(ctx, mediaStoragePath, originalFile, fileSize); err != nil {
		return domain.MediaAsset{}, err
	}

	kind := mediaKindFromExt(ext)
	thumbLocalPath, err := buildThumbnail(ctx, in.LocalPath, kind, in.Filename)
	if err != nil {
		return domain.MediaAsset{}, err
	}
	defer os.Remove(thumbLocalPath)

	thumbFile, err := os.Open(thumbLocalPath)
	if err != nil {
		return domain.MediaAsset{}, err
	}
	defer thumbFile.Close()

	thumbInfo, err := thumbFile.Stat()
	if err != nil {
		return domain.MediaAsset{}, err
	}
	if err := adapter.WriteFile(ctx, thumbStoragePath, thumbFile, thumbInfo.Size()); err != nil {
		return domain.MediaAsset{}, err
	}

	now := time.Now()
	asset := domain.MediaAsset{
		ID:             fileID,
		OrganizationID: orgID,
		ProjectID:      uuid.NullUUID{},
		Filename:       in.Filename,
		StoragePath:    mediaStoragePath,
		ThumbnailPath:  null.StringFrom(thumbStoragePath),
		MIMEType:       null.StringFrom(mimeType),
		SHA256:         null.StringFrom(checksum),
		DurationSec:    null.NewFloat(durationSec, durationSec > 0),
		FileSizeBytes:  null.IntFrom(fileSize),
		CapturedAt:     null.TimeFrom(*capturedAt),
		Status:         domain.MediaPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := u.media.Create(ctx, asset); err != nil {
		return domain.MediaAsset{}, normalizeRepoErr(err)
	}
	return asset, nil
}

func (u *MediaUsecase) OpenStream(
	ctx context.Context,
	orgID uuid.UUID,
	mediaID uuid.UUID,
) (io.ReadCloser, int64, string, string, error) {
	asset, err := u.media.GetByIDForOrg(ctx, mediaID, orgID)
	if err != nil {
		return nil, 0, "", "", normalizeRepoErr(err)
	}

	cfg, err := u.storage.GetActiveByOrg(ctx, orgID)
	if err != nil {
		return nil, 0, "", "", normalizeRepoErr(err)
	}
	adapter, err := adapterStorage.NewAdapter(cfg)
	if err != nil {
		return nil, 0, "", "", err
	}

	rc, size, err := adapter.GetFile(ctx, asset.StoragePath)
	if err != nil {
		return nil, 0, "", "", err
	}

	mimeType := "application/octet-stream"
	if asset.MIMEType.Valid && strings.TrimSpace(asset.MIMEType.String) != "" {
		mimeType = asset.MIMEType.String
	}
	return rc, size, mimeType, asset.Filename, nil
}

func (u *MediaUsecase) OpenThumbnail(
	ctx context.Context,
	orgID uuid.UUID,
	mediaID uuid.UUID,
) (io.ReadCloser, int64, string, error) {
	asset, err := u.media.GetByIDForOrg(ctx, mediaID, orgID)
	if err != nil {
		return nil, 0, "", normalizeRepoErr(err)
	}
	if !asset.ThumbnailPath.Valid || strings.TrimSpace(asset.ThumbnailPath.String) == "" {
		return nil, 0, "", ErrNotFound.New("thumbnail not found")
	}

	cfg, err := u.storage.GetActiveByOrg(ctx, orgID)
	if err != nil {
		return nil, 0, "", normalizeRepoErr(err)
	}
	adapter, err := adapterStorage.NewAdapter(cfg)
	if err != nil {
		return nil, 0, "", err
	}
	rc, size, err := adapter.GetFile(ctx, asset.ThumbnailPath.String)
	if err != nil {
		return nil, 0, "", err
	}
	return rc, size, "image/jpeg", nil
}

func (u *MediaUsecase) OpenSegmentFrame(ctx context.Context, orgID, mediaID uuid.UUID, frameNumber int) (io.ReadCloser, int64, string, error) {
	if _, err := u.media.GetByIDForOrg(ctx, mediaID, orgID); err != nil {
		return nil, 0, "", normalizeRepoErr(err)
	}

	results, err := u.analysisResults.ListByMedia(ctx, mediaID)
	if err != nil {
		return nil, 0, "", err
	}

	var thumbnailPath string
outer:
	for _, r := range results {
		if r.AnalyzerType != string(domain.AnalyzerVisionTags) {
			continue
		}
		var segmented llm.VisionSegmentedResult
		if err := json.Unmarshal(r.Output, &segmented); err != nil {
			continue
		}
		for _, seg := range segmented.Segments {
			if seg.FrameNumber == frameNumber && seg.ThumbnailStoragePath != "" {
				thumbnailPath = seg.ThumbnailStoragePath
				break outer
			}
		}
	}

	if thumbnailPath == "" {
		return nil, 0, "", ErrNotFound.New("segment frame not found")
	}

	cfg, err := u.storage.GetActiveByOrg(ctx, orgID)
	if err != nil {
		return nil, 0, "", normalizeRepoErr(err)
	}
	adapter, err := adapterStorage.NewAdapter(cfg)
	if err != nil {
		return nil, 0, "", err
	}
	rc, size, err := adapter.GetFile(ctx, thumbnailPath)
	if err != nil {
		return nil, 0, "", err
	}
	return rc, size, "image/jpeg", nil
}

type mediaKind string

const (
	mediaKindVideo mediaKind = "video"
	mediaKindImage mediaKind = "image"
	mediaKindAudio mediaKind = "audio"
)

var allowedExtensions = map[string]struct{}{
	".mov":  {},
	".mp4":  {},
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".wav":  {},
	".mp3":  {},
}

func mediaKindFromExt(ext string) mediaKind {
	switch ext {
	case ".mov", ".mp4":
		return mediaKindVideo
	case ".jpg", ".jpeg", ".png":
		return mediaKindImage
	default:
		return mediaKindAudio
	}
}

func sanitizeFilename(name string) string {
	base := path.Base(strings.TrimSpace(name))
	if base == "." || base == "/" {
		return ""
	}
	var b strings.Builder
	for _, r := range base {
		if r == ' ' {
			b.WriteByte('-')
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-_.")
}

func joinStoragePath(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.Trim(p, " /")
		if t != "" {
			clean = append(clean, t)
		}
	}
	return strings.Join(clean, "/")
}

func fileFingerprint(filePath string) (mimeType string, size int64, checksum string, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, "", err
	}
	defer f.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(f, head)
	mimeType = http.DetectContentType(head[:n])
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", 0, "", err
	}

	hash := sha256.New()
	size, err = io.Copy(hash, f)
	if err != nil {
		return "", 0, "", err
	}
	return mimeType, size, hex.EncodeToString(hash.Sum(nil)), nil
}

func buildThumbnail(ctx context.Context, sourcePath string, kind mediaKind, seed string) (string, error) {
	switch kind {
	case mediaKindVideo:
		return ffmpeg.ExtractThumbnail(ctx, sourcePath, 1)
	case mediaKindImage:
		return buildImageThumbnail(sourcePath)
	default:
		return buildAudioThumbnail(seed)
	}
}

func buildImageThumbnail(sourcePath string) (string, error) {
	f, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}

	dst := fitToMax(img, 640, 640)
	out, err := os.CreateTemp("", "agentra-upload-thumb-*.jpg")
	if err != nil {
		return "", err
	}
	defer out.Close()

	if err := jpeg.Encode(out, dst, &jpeg.Options{Quality: 82}); err != nil {
		return "", err
	}
	return out.Name(), nil
}

func fitToMax(src image.Image, maxW, maxH int) *image.RGBA {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return image.NewRGBA(image.Rect(0, 0, maxW, maxH))
	}

	scale := math.Min(float64(maxW)/float64(srcW), float64(maxH)/float64(srcH))
	if scale > 1 {
		scale = 1
	}
	dstW := max(1, int(math.Round(float64(srcW)*scale)))
	dstH := max(1, int(math.Round(float64(srcH)*scale)))

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		sy := bounds.Min.Y + (y*srcH)/dstH
		for x := 0; x < dstW; x++ {
			sx := bounds.Min.X + (x*srcW)/dstW
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func buildAudioThumbnail(seed string) (string, error) {
	const w = 640
	const h = 360
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	bg := color.RGBA{R: 15, G: 23, B: 42, A: 255}
	drawpkg.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, drawpkg.Src)

	sum := sha256.Sum256([]byte(seed))
	barColor := color.RGBA{R: 56, G: 189, B: 248, A: 255}
	shadowColor := color.RGBA{R: 14, G: 116, B: 144, A: 255}

	barW := 10
	gap := 8
	x := 40
	for i := 0; x < (w - 40); i++ {
		v := int(sum[i%len(sum)])
		barH := 26 + (v % 220)
		top := (h - barH) / 2
		drawpkg.Draw(img, image.Rect(x+2, top+2, x+barW+2, top+barH+2), &image.Uniform{C: shadowColor}, image.Point{}, drawpkg.Src)
		drawpkg.Draw(img, image.Rect(x, top, x+barW, top+barH), &image.Uniform{C: barColor}, image.Point{}, drawpkg.Src)
		x += barW + gap
	}

	out, err := os.CreateTemp("", "agentra-audio-thumb-*.jpg")
	if err != nil {
		return "", err
	}
	defer out.Close()

	if err := jpeg.Encode(out, img, &jpeg.Options{Quality: 82}); err != nil {
		return "", err
	}
	return out.Name(), nil
}

type ffprobeOutput struct {
	Format struct {
		Duration string            `json:"duration"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
	Streams []struct {
		Tags map[string]string `json:"tags"`
	} `json:"streams"`
}

func probeMediaMetadata(ctx context.Context, filePath string) (*time.Time, float64) {
	cmd := exec.CommandContext(
		ctx,
		"ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_entries", "format=duration:format_tags:stream_tags",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, 0
	}

	var payload ffprobeOutput
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, 0
	}

	var duration float64
	if payload.Format.Duration != "" {
		if v, err := strconv.ParseFloat(payload.Format.Duration, 64); err == nil && v > 0 {
			duration = v
		}
	}

	allTags := make([]map[string]string, 0, 1+len(payload.Streams))
	if payload.Format.Tags != nil {
		allTags = append(allTags, payload.Format.Tags)
	}
	for _, stream := range payload.Streams {
		if stream.Tags != nil {
			allTags = append(allTags, stream.Tags)
		}
	}

	for _, tags := range allTags {
		if ts := firstTimestamp(tags); ts != nil {
			return ts, duration
		}
	}
	return nil, duration
}

func firstTimestamp(tags map[string]string) *time.Time {
	if len(tags) == 0 {
		return nil
	}
	candidates := []string{
		"datetimeoriginal",
		"creation_time",
		"creationdate",
		"com.apple.quicktime.creationdate",
		"date",
		"datetime",
	}

	normalized := make(map[string]string, len(tags))
	for k, v := range tags {
		normalized[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}

	for _, key := range candidates {
		if raw, ok := normalized[key]; ok {
			if parsed, ok := parseMetadataTime(raw); ok {
				return &parsed
			}
		}
	}
	return nil
}

func parseMetadataTime(raw string) (time.Time, bool) {
	v := strings.TrimSpace(raw)
	v = strings.Trim(v, "'\"")
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006:01:02 15:04:05",
		"2006:01:02 15:04:05-0700",
		"2006:01:02 15:04:05Z07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, true
		}
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
