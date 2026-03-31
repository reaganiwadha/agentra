package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/reaganiwadha/agentra/internal/adapter/llm"
	"github.com/reaganiwadha/agentra/internal/assets"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type TranscriptionTestEvent struct {
	Event        string         `json:"event"`
	Message      string         `json:"message"`
	TimestampUTC time.Time      `json:"timestamp_utc"`
	AnalyzerID   string         `json:"analyzer_id"`
	AnalyzerName string         `json:"analyzer_name"`
	SampleID     string         `json:"sample_id,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
}

type testSample struct {
	ID                            string   `json:"id"`
	Name                          string   `json:"name"`
	URL                           string   `json:"url"`
	InputText                     string   `json:"input_text,omitempty"`
	ExpectedContains              []string `json:"expected_contains,omitempty"`
	ExpectedAnyTags               []string `json:"expected_any_tags,omitempty"`
	ExpectedDescriptionAnyContain []string `json:"expected_description_any_contains,omitempty"`
	ExpectedVectorMinLength       int      `json:"expected_vector_min_length,omitempty"`
}

type testManifest struct {
	Samples []testSample `json:"samples"`
}

func (u *AnalyzerAdminUsecase) StartAnalyzerTest(ctx context.Context, orgID, analyzerID uuid.UUID) (<-chan TranscriptionTestEvent, error) {
	analyzerType, err := u.resolveAnalyzerTypeForTest(ctx, orgID, analyzerID)
	if err != nil {
		return nil, err
	}

	switch analyzerType {
	case domain.AnalyzerTranscription:
		return u.startTest(ctx, orgID, analyzerID, domain.AnalyzerTranscription, assets.TranscriptionSampleManifest, u.runTranscriptionTest)
	case domain.AnalyzerVisionTags:
		return u.startTest(ctx, orgID, analyzerID, domain.AnalyzerVisionTags, assets.VisionTagSampleManifest, u.runVisionTagsTest)
	case domain.AnalyzerEmbedding:
		return u.startEmbeddingTest(ctx, orgID, analyzerID)
	default:
		return nil, ErrForbidden.New("analyzer tests support only transcription, vision_tags, and embedding types")
	}
}

func (u *AnalyzerAdminUsecase) resolveAnalyzerTypeForTest(ctx context.Context, orgID, analyzerID uuid.UUID) (domain.AnalyzerType, error) {
	analyzer, err := u.analyzers.GetByID(ctx, orgID, analyzerID)
	if err != nil {
		return "", normalizeRepoErr(err)
	}
	provider, err := u.getActiveProvider(ctx, orgID, analyzer.ProviderID)
	if err != nil {
		return "", err
	}

	normalized, err := normalizeAnalyzerInput(provider, AnalyzerInput{
		Name:         analyzer.Name,
		AnalyzerType: analyzer.AnalyzerType,
		ProviderID:   analyzer.ProviderID,
		ModelName:    analyzer.ModelName,
		ConfigJSON:   string(analyzer.ConfigJSON),
		IsEnabled:    analyzer.IsEnabled,
	})
	if err != nil {
		return "", err
	}
	return normalized.AnalyzerType, nil
}

func (u *AnalyzerAdminUsecase) startTest(
	ctx context.Context,
	orgID, analyzerID uuid.UUID,
	expectedType domain.AnalyzerType,
	loadManifest func() ([]byte, error),
	runner func(context.Context, domain.Analyzer, domain.ModelProvider, []testSample, chan<- TranscriptionTestEvent),
) (<-chan TranscriptionTestEvent, error) {
	analyzer, err := u.analyzers.GetByID(ctx, orgID, analyzerID)
	if err != nil {
		return nil, normalizeRepoErr(err)
	}

	provider, err := u.getActiveProvider(ctx, orgID, analyzer.ProviderID)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizeAnalyzerInput(provider, AnalyzerInput{
		Name:         analyzer.Name,
		AnalyzerType: analyzer.AnalyzerType,
		ProviderID:   analyzer.ProviderID,
		ModelName:    analyzer.ModelName,
		ConfigJSON:   string(analyzer.ConfigJSON),
		IsEnabled:    analyzer.IsEnabled,
	})
	if err != nil {
		return nil, err
	}
	if normalized.AnalyzerType != expectedType {
		return nil, ErrForbidden.New(fmt.Sprintf("%s test only supports %s analyzers", expectedType, expectedType))
	}

	analyzer.Name, analyzer.ModelName, analyzer.AnalyzerType = normalized.Name, normalized.ModelName, normalized.AnalyzerType

	raw, err := loadManifest()
	if err != nil {
		return nil, ErrForbidden.New("failed to load test assets")
	}
	var manifest testManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, ErrForbidden.New("failed to parse test assets")
	}

	samples := make([]testSample, 0, len(manifest.Samples))
	for _, s := range manifest.Samples {
		if s.ID != "" && s.Name != "" && s.URL != "" {
			samples = append(samples, s)
		}
	}
	if len(samples) == 0 {
		return nil, ErrForbidden.New("no test assets configured")
	}

	events := make(chan TranscriptionTestEvent, 32)
	go runner(ctx, analyzer, provider, samples, events)
	return events, nil
}

func (u *AnalyzerAdminUsecase) startEmbeddingTest(ctx context.Context, orgID, analyzerID uuid.UUID) (<-chan TranscriptionTestEvent, error) {
	analyzer, err := u.analyzers.GetByID(ctx, orgID, analyzerID)
	if err != nil {
		return nil, normalizeRepoErr(err)
	}
	provider, err := u.getActiveProvider(ctx, orgID, analyzer.ProviderID)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizeAnalyzerInput(provider, AnalyzerInput{
		Name:         analyzer.Name,
		AnalyzerType: analyzer.AnalyzerType,
		ProviderID:   analyzer.ProviderID,
		ModelName:    analyzer.ModelName,
		ConfigJSON:   string(analyzer.ConfigJSON),
		IsEnabled:    analyzer.IsEnabled,
	})
	if err != nil {
		return nil, err
	}
	if normalized.AnalyzerType != domain.AnalyzerEmbedding {
		return nil, ErrForbidden.New("embedding test only supports embedding analyzers")
	}
	analyzer.Name, analyzer.ModelName, analyzer.AnalyzerType = normalized.Name, normalized.ModelName, normalized.AnalyzerType

	raw, err := assets.EmbeddingSampleManifest()
	if err != nil {
		return nil, ErrForbidden.New("failed to load embedding test assets")
	}
	var manifest testManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, ErrForbidden.New("failed to parse embedding test assets")
	}

	samples := make([]testSample, 0, len(manifest.Samples))
	for _, s := range manifest.Samples {
		if s.ID != "" && s.Name != "" && strings.TrimSpace(s.InputText) != "" {
			samples = append(samples, s)
		}
	}
	if len(samples) == 0 {
		return nil, ErrForbidden.New("no embedding test assets configured")
	}

	events := make(chan TranscriptionTestEvent, 32)
	go u.runEmbeddingTest(ctx, analyzer, provider, samples, events)
	return events, nil
}

func (u *AnalyzerAdminUsecase) runTranscriptionTest(ctx context.Context, a domain.Analyzer, p domain.ModelProvider, s []testSample, e chan<- TranscriptionTestEvent) {
	u.runTest(ctx, a, p, "transcription", s, e, func(ctx context.Context, cfg llm.Config, path string, sample testSample) (string, map[string]any, bool, bool) {
		res, err := u.llm.Transcribe(ctx, cfg, path)
		if err != nil {
			return fmt.Sprintf("Transcription failed: %v", err), nil, false, false
		}
		ok := transcriptMatchesExpectations(res.Text, sample.ExpectedContains)
		return "Transcription completed.", map[string]any{
			"segment_count":    len(res.Segments),
			"text_preview":     compactPreview(res.Text, 200),
			"expectation_ok":   ok,
			"expected_phrases": sample.ExpectedContains,
		}, true, !ok
	})
}

func (u *AnalyzerAdminUsecase) runVisionTagsTest(ctx context.Context, a domain.Analyzer, p domain.ModelProvider, s []testSample, e chan<- TranscriptionTestEvent) {
	u.runTest(ctx, a, p, "vision tag", s, e, func(ctx context.Context, cfg llm.Config, path string, sample testSample) (string, map[string]any, bool, bool) {
		res, err := u.llm.AnalyzeFrames(ctx, cfg, []string{path})
		if err != nil {
			return fmt.Sprintf("Vision analysis failed: %v", err), nil, false, false
		}
		tags := normalizeTags(res.Tags)
		okTag := visionTagsMatchAnyExpectation(tags, sample.ExpectedAnyTags)
		okDesc := textContainsAny(res.Description, sample.ExpectedDescriptionAnyContain)
		ok := okTag && okDesc
		return "Vision analysis completed.", map[string]any{
			"tag_count":                         len(tags),
			"tags":                              tags,
			"description_preview":               compactPreview(res.Description, 200),
			"expect_tag_ok":                     okTag,
			"expect_description_ok":             okDesc,
			"expected_any_tags":                 sample.ExpectedAnyTags,
			"expected_description_any_contains": sample.ExpectedDescriptionAnyContain,
		}, true, !ok
	})
}

func (u *AnalyzerAdminUsecase) runEmbeddingTest(
	ctx context.Context,
	analyzer domain.Analyzer,
	provider domain.ModelProvider,
	samples []testSample,
	events chan<- TranscriptionTestEvent,
) {
	defer close(events)
	cfg := llm.Config{
		BaseURL:      provider.BaseURL,
		APIKey:       provider.APIKey.String,
		ModelName:    analyzer.ModelName,
		ProviderType: provider.ProviderType,
	}

	emitEvent(ctx, events, TranscriptionTestEvent{
		Event:        "run.started",
		Message:      fmt.Sprintf("Running embedding tests for analyzer %q.", analyzer.Name),
		TimestampUTC: time.Now().UTC(),
		AnalyzerID:   analyzer.ID.String(),
		AnalyzerName: analyzer.Name,
		Payload: map[string]any{
			"provider_type": provider.ProviderType,
			"provider_name": provider.Name,
			"model_name":    analyzer.ModelName,
			"sample_count":  len(samples),
		},
	})

	var stats struct{ success, failure, warning int }
	for i, sample := range samples {
		if ctx.Err() != nil {
			emitEvent(ctx, events, TranscriptionTestEvent{
				Event:        "run.cancelled",
				Message:      "Test stream disconnected.",
				TimestampUTC: time.Now().UTC(),
				AnalyzerID:   analyzer.ID.String(),
				AnalyzerName: analyzer.Name,
			})
			return
		}

		emitEvent(ctx, events, TranscriptionTestEvent{
			Event:        "sample.started",
			Message:      fmt.Sprintf("Testing sample %d/%d: %s", i+1, len(samples), sample.Name),
			TimestampUTC: time.Now().UTC(),
			AnalyzerID:   analyzer.ID.String(),
			AnalyzerName: analyzer.Name,
			SampleID:     sample.ID,
			Payload: map[string]any{
				"sample_name":      sample.Name,
				"input_char_count": len(sample.InputText),
				"input_preview":    compactPreview(sample.InputText, 120),
			},
		})

		vector, err := u.llm.Embed(ctx, cfg, sample.InputText)
		if err != nil {
			stats.failure++
			emitEvent(ctx, events, TranscriptionTestEvent{
				Event:        "sample.failed",
				Message:      fmt.Sprintf("Embedding failed: %v", err),
				TimestampUTC: time.Now().UTC(),
				AnalyzerID:   analyzer.ID.String(),
				AnalyzerName: analyzer.Name,
				SampleID:     sample.ID,
			})
			continue
		}

		dimensions := len(vector)
		if dimensions == 0 {
			stats.failure++
			emitEvent(ctx, events, TranscriptionTestEvent{
				Event:        "sample.failed",
				Message:      "Embedding returned empty vector.",
				TimestampUTC: time.Now().UTC(),
				AnalyzerID:   analyzer.ID.String(),
				AnalyzerName: analyzer.Name,
				SampleID:     sample.ID,
			})
			continue
		}

		minDimensions := sample.ExpectedVectorMinLength
		if minDimensions <= 0 {
			minDimensions = 1
		}
		meetsDimensionExpectation := dimensions >= minDimensions
		isZeroVector := embeddingIsZeroVector(vector)
		warn := !meetsDimensionExpectation || isZeroVector

		if warn {
			stats.warning++
		}
		stats.success++

		emitEvent(ctx, events, TranscriptionTestEvent{
			Event:        "sample.completed",
			Message:      "Embedding completed.",
			TimestampUTC: time.Now().UTC(),
			AnalyzerID:   analyzer.ID.String(),
			AnalyzerName: analyzer.Name,
			SampleID:     sample.ID,
			Payload: map[string]any{
				"vector_dimensions":     dimensions,
				"vector_preview":        previewVector(vector, 8),
				"expect_min_dimensions": minDimensions,
				"expect_dimensions_ok":  meetsDimensionExpectation,
				"is_all_zero_vector":    isZeroVector,
				"input_preview":         compactPreview(sample.InputText, 120),
			},
		})
	}

	finalEvent := "run.completed"
	if stats.success == 0 {
		finalEvent = "run.failed"
	}
	emitEvent(ctx, events, TranscriptionTestEvent{
		Event:        finalEvent,
		Message:      "Embedding test run finished.",
		TimestampUTC: time.Now().UTC(),
		AnalyzerID:   analyzer.ID.String(),
		AnalyzerName: analyzer.Name,
		Payload: map[string]any{
			"successes": stats.success,
			"failures":  stats.failure,
			"warnings":  stats.warning,
		},
	})
}

func (u *AnalyzerAdminUsecase) runTest(
	ctx context.Context,
	analyzer domain.Analyzer,
	provider domain.ModelProvider,
	testName string,
	samples []testSample,
	events chan<- TranscriptionTestEvent,
	execute func(context.Context, llm.Config, string, testSample) (string, map[string]any, bool, bool),
) {
	defer close(events)
	cfg := llm.Config{BaseURL: provider.BaseURL, APIKey: provider.APIKey.String, ModelName: analyzer.ModelName, ProviderType: provider.ProviderType}

	emitEvent(ctx, events, TranscriptionTestEvent{
		Event: "run.started", Message: fmt.Sprintf("Running %s tests for analyzer %q.", testName, analyzer.Name),
		TimestampUTC: time.Now().UTC(), AnalyzerID: analyzer.ID.String(), AnalyzerName: analyzer.Name,
		Payload: map[string]any{"provider_type": provider.ProviderType, "provider_name": provider.Name, "model_name": analyzer.ModelName, "sample_count": len(samples)},
	})

	var stats struct{ success, failure, warning int }
	for i, s := range samples {
		if ctx.Err() != nil {
			emitEvent(ctx, events, TranscriptionTestEvent{Event: "run.cancelled", Message: "Test stream disconnected.", TimestampUTC: time.Now().UTC(), AnalyzerID: analyzer.ID.String(), AnalyzerName: analyzer.Name})
			return
		}

		emitEvent(ctx, events, TranscriptionTestEvent{
			Event: "sample.started", Message: fmt.Sprintf("Testing sample %d/%d: %s", i+1, len(samples), s.Name),
			TimestampUTC: time.Now().UTC(), AnalyzerID: analyzer.ID.String(), AnalyzerName: analyzer.Name, SampleID: s.ID,
			Payload: map[string]any{"sample_name": s.Name, "source_url": s.URL},
		})

		path, size, err := downloadSample(ctx, s.URL)
		if err != nil {
			stats.failure++
			emitEvent(ctx, events, TranscriptionTestEvent{Event: "sample.failed", Message: fmt.Sprintf("Fetch failed: %v", err), TimestampUTC: time.Now().UTC(), AnalyzerID: analyzer.ID.String(), AnalyzerName: analyzer.Name, SampleID: s.ID})
			continue
		}

		emitEvent(ctx, events, TranscriptionTestEvent{Event: "sample.downloaded", Message: fmt.Sprintf("Fetched %s (%d bytes).", s.Name, size), TimestampUTC: time.Now().UTC(), AnalyzerID: analyzer.ID.String(), AnalyzerName: analyzer.Name, SampleID: s.ID})

		msg, payload, ok, warn := execute(ctx, cfg, path, s)
		_ = os.Remove(path)

		if !ok {
			stats.failure++
			emitEvent(ctx, events, TranscriptionTestEvent{Event: "sample.failed", Message: msg, TimestampUTC: time.Now().UTC(), AnalyzerID: analyzer.ID.String(), AnalyzerName: analyzer.Name, SampleID: s.ID})
			continue
		}

		if warn {
			stats.warning++
		}
		stats.success++
		emitEvent(ctx, events, TranscriptionTestEvent{Event: "sample.completed", Message: msg, TimestampUTC: time.Now().UTC(), AnalyzerID: analyzer.ID.String(), AnalyzerName: analyzer.Name, SampleID: s.ID, Payload: payload})
	}

	finalEvent := "run.completed"
	if stats.success == 0 {
		finalEvent = "run.failed"
	}
	emitEvent(ctx, events, TranscriptionTestEvent{
		Event: finalEvent, Message: fmt.Sprintf("%s test run finished.", strings.Title(testName)),
		TimestampUTC: time.Now().UTC(), AnalyzerID: analyzer.ID.String(), AnalyzerName: analyzer.Name,
		Payload: map[string]any{"successes": stats.success, "failures": stats.failure, "warnings": stats.warning},
	})
}

func emitEvent(ctx context.Context, events chan<- TranscriptionTestEvent, event TranscriptionTestEvent) {
	select {
	case <-ctx.Done():
	case events <- event:
	}
}

func downloadSample(ctx context.Context, sourceURL string) (string, int64, error) {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return "", 0, err
	}
	res, err := http.DefaultClient.Do(MustNewRequest(ctx, "GET", sourceURL, nil))
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", 0, fmt.Errorf("status %d", res.StatusCode)
	}

	ext := strings.ToLower(path.Ext(parsed.Path))
	tmp, err := os.CreateTemp("", "agentra-test-*"+ext)
	if err != nil {
		return "", 0, err
	}
	defer tmp.Close()
	n, err := io.Copy(tmp, res.Body)
	return tmp.Name(), n, err
}

func MustNewRequest(ctx context.Context, method, url string, body io.Reader) *http.Request {
	req, _ := http.NewRequestWithContext(ctx, method, url, body)
	if req != nil {
		req.Header.Set("User-Agent", "Agentra/1.0 (https://github.com/reaganiwadha/agentra)")
	}
	return req
}

func transcriptMatchesExpectations(text string, expected []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if len(expected) > 0 && normalized == "" {
		return false
	}
	for _, phrase := range expected {
		if !strings.Contains(normalized, strings.ToLower(strings.TrimSpace(phrase))) {
			return false
		}
	}
	return true
}

func normalizeTags(tags []string) []string {
	out, seen := []string{}, map[string]struct{}{}
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if key := strings.ToLower(tag); tag != "" {
			if _, exists := seen[key]; !exists {
				seen[key], out = struct{}{}, append(out, tag)
			}
		}
	}
	return out
}

func visionTagsMatchAnyExpectation(tags []string, expectedAny []string) bool {
	if len(expectedAny) == 0 {
		return true
	}
	for _, want := range expectedAny {
		if target := strings.ToLower(strings.TrimSpace(want)); target != "" {
			for _, tag := range tags {
				if strings.Contains(strings.ToLower(strings.TrimSpace(tag)), target) {
					return true
				}
			}
		}
	}
	return false
}

func textContainsAny(text string, expectedAny []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if len(expectedAny) == 0 {
		return true
	}
	for _, want := range expectedAny {
		if target := strings.ToLower(strings.TrimSpace(want)); target != "" && strings.Contains(normalized, target) {
			return true
		}
	}
	return false
}

func previewVector(vector []float64, maxItems int) []float64 {
	if maxItems <= 0 || len(vector) <= maxItems {
		return vector
	}
	out := make([]float64, maxItems)
	copy(out, vector[:maxItems])
	return out
}

func embeddingIsZeroVector(vector []float64) bool {
	for _, v := range vector {
		if math.Abs(v) > 1e-12 {
			return false
		}
	}
	return true
}

func compactPreview(text string, maxLen int) string {
	clean := strings.Join(strings.Fields(text), " ")
	if len(clean) <= maxLen {
		return clean
	}
	return clean[:maxLen-3] + "..."
}
