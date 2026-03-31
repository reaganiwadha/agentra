package llm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/reaganiwadha/agentra/internal/domain"
)

type Client struct {
	r *resty.Client
}

func NewClient() *Client {
	return &Client{r: resty.New()}
}

type Config struct {
	BaseURL      string
	APIKey       string
	ModelName    string
	ProviderType domain.ProviderType
}

type TranscriptResult struct {
	Text     string              `json:"text"`
	Segments []TranscriptSegment `json:"segments"`
}

type TranscriptSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type VisionResult struct {
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
}

type VisionSegment struct {
	FrameNumber          int     `json:"frame_number"`
	TimestampSec         float64 `json:"timestamp_sec"`
	Description          string  `json:"description"`
	ThumbnailStoragePath string  `json:"thumbnail_storage_path"`
}

type VisionSegmentedResult struct {
	Segments []VisionSegment `json:"segments"`
	Summary  string          `json:"summary"`
}

type ChatResult struct {
	Content string `json:"content"`
}

func (c *Client) Transcribe(ctx context.Context, cfg Config, audioPath string) (TranscriptResult, error) {
	if cfg.ProviderType == domain.ProviderDeepgram {
		return c.transcribeDeepgram(ctx, cfg, audioPath)
	}
	return c.transcribeOpenAICompat(ctx, cfg, audioPath)
}

func (c *Client) transcribeOpenAICompat(ctx context.Context, cfg Config, audioPath string) (TranscriptResult, error) {
	var result TranscriptResult
	resp, err := c.r.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+cfg.APIKey).
		SetFile("file", audioPath).
		SetFormData(map[string]string{
			"model":           cfg.ModelName,
			"response_format": "verbose_json",
		}).
		SetResult(&result).
		Post(strings.TrimRight(cfg.BaseURL, "/") + "/audio/transcriptions")
	if err != nil {
		return TranscriptResult{}, err
	}
	if resp.IsError() {
		return TranscriptResult{}, fmt.Errorf("transcription API %d: %s", resp.StatusCode(), resp.String())
	}
	return result, nil
}

func (c *Client) transcribeDeepgram(ctx context.Context, cfg Config, audioPath string) (TranscriptResult, error) {
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return TranscriptResult{}, fmt.Errorf("read audio file: %w", err)
	}

	model := strings.TrimSpace(cfg.ModelName)
	if model == "" {
		model = "nova-3"
	}

	var raw struct {
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
					Words      []struct {
						Word  string  `json:"word"`
						Start float64 `json:"start"`
						End   float64 `json:"end"`
					} `json:"words"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}

	resp, err := c.r.R().
		SetContext(ctx).
		SetHeader("Authorization", deepgramAuthorization(cfg.APIKey)).
		SetHeader("Content-Type", detectAudioContentType(audioPath)).
		SetQueryParam("model", model).
		SetQueryParam("smart_format", "true").
		SetQueryParam("detect_language", "true").
		SetBody(audioData).
		SetResult(&raw).
		Post(deepgramListenURL(cfg.BaseURL))
	if err != nil {
		return TranscriptResult{}, err
	}
	if resp.IsError() {
		return TranscriptResult{}, fmt.Errorf("deepgram API %d: %s", resp.StatusCode(), resp.String())
	}
	if len(raw.Results.Channels) == 0 || len(raw.Results.Channels[0].Alternatives) == 0 {
		return TranscriptResult{}, fmt.Errorf("deepgram API: no alternatives returned")
	}

	alt := raw.Results.Channels[0].Alternatives[0]
	out := TranscriptResult{
		Text:     strings.TrimSpace(alt.Transcript),
		Segments: make([]TranscriptSegment, 0),
	}

	words := alt.Words
	if len(words) > 0 {
		segStart := words[0].Start
		segEnd := words[0].End
		segWords := []string{strings.TrimSpace(words[0].Word)}

		flush := func() {
			text := strings.Join(segWords, " ")
			if strings.TrimSpace(text) != "" {
				out.Segments = append(out.Segments, TranscriptSegment{
					Start: segStart,
					End:   segEnd,
					Text:  text,
				})
			}
		}

		for i := 1; i < len(words); i++ {
			w := words[i]
			prev := words[i-1]
			word := strings.TrimSpace(w.Word)
			if word == "" {
				continue
			}
			prevWord := prev.Word
			gap := w.Start - prev.End
			prevEndsWithPunct := len(prevWord) > 0 && strings.ContainsRune(".?!,", rune(prevWord[len(prevWord)-1]))
			spanTooLong := w.End-segStart > 8

			if gap > 0.5 || prevEndsWithPunct || spanTooLong {
				flush()
				segStart = w.Start
				segWords = []string{word}
			} else {
				segWords = append(segWords, word)
			}
			segEnd = w.End
		}
		flush()
	}

	if out.Text == "" && len(out.Segments) > 0 {
		parts := make([]string, 0, len(out.Segments))
		for _, seg := range out.Segments {
			parts = append(parts, seg.Text)
		}
		out.Text = strings.Join(parts, " ")
	}

	return out, nil
}

func (c *Client) AnalyzeFrames(ctx context.Context, cfg Config, framePaths []string) (VisionResult, error) {
	content := make([]map[string]any, 0, len(framePaths)+1)
	for _, path := range framePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]string{
				"url": "data:image/jpeg;base64," + b64,
			},
		})
	}
	content = append(content, map[string]any{
		"type": "text",
		"text": visionPrompt,
	})

	body := map[string]any{
		"model": cfg.ModelName,
		"messages": []map[string]any{
			{"role": "user", "content": content},
		},
		"max_tokens":      1024,
		"response_format": map[string]string{"type": "json_object"},
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	resp, err := c.r.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+cfg.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		SetResult(&raw).
		Post(strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions")
	if err != nil {
		return VisionResult{}, err
	}
	if resp.IsError() {
		return VisionResult{}, fmt.Errorf("vision API %d: %s", resp.StatusCode(), resp.String())
	}
	if len(raw.Choices) == 0 {
		return VisionResult{}, fmt.Errorf("vision API: no choices returned")
	}

	var result VisionResult
	if err := json.Unmarshal([]byte(raw.Choices[0].Message.Content), &result); err != nil {
		return VisionResult{}, fmt.Errorf("vision API: invalid JSON response: %w", err)
	}
	return result, nil
}

func (c *Client) AnalyzeSingleFrame(ctx context.Context, cfg Config, framePath string, frameNum int, timestampSec float64) (VisionSegment, error) {
	data, err := os.ReadFile(framePath)
	if err != nil {
		return VisionSegment{}, fmt.Errorf("read frame: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(data)

	content := []map[string]any{
		{
			"type": "image_url",
			"image_url": map[string]string{
				"url": "data:image/jpeg;base64," + b64,
			},
		},
		{
			"type": "text",
			"text": "Describe what is happening in this video frame in 1-2 sentences. Be specific about people, actions, objects, and setting.",
		},
	}

	body := map[string]any{
		"model": cfg.ModelName,
		"messages": []map[string]any{
			{"role": "user", "content": content},
		},
		"max_tokens": 1024,
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	resp, err := c.r.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+cfg.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		SetResult(&raw).
		Post(strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions")
	if err != nil {
		return VisionSegment{}, err
	}
	if resp.IsError() {
		return VisionSegment{}, fmt.Errorf("vision API %d: %s", resp.StatusCode(), resp.String())
	}
	if len(raw.Choices) == 0 {
		return VisionSegment{}, fmt.Errorf("vision API: no choices returned")
	}

	return VisionSegment{
		FrameNumber:  frameNum,
		TimestampSec: timestampSec,
		Description:  strings.TrimSpace(raw.Choices[0].Message.Content),
	}, nil
}

func (c *Client) SummarizeVisionSegments(ctx context.Context, cfg Config, segments []VisionSegment) (string, error) {
	var sb strings.Builder
	for _, seg := range segments {
		sb.WriteString(fmt.Sprintf("Frame at %.1fs: %s\n", seg.TimestampSec, seg.Description))
	}

	body := map[string]any{
		"model": cfg.ModelName,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": "Based on these timestamped video frame descriptions, write a 2-3 sentence summary of the full video content:\n\n" + sb.String(),
			},
		},
		"max_tokens": 512,
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	resp, err := c.r.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+cfg.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		SetResult(&raw).
		Post(strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions")
	if err != nil {
		return "", err
	}
	if resp.IsError() {
		return "", fmt.Errorf("vision summarize API %d: %s", resp.StatusCode(), resp.String())
	}
	if len(raw.Choices) == 0 {
		return "", fmt.Errorf("vision summarize API: no choices returned")
	}
	return strings.TrimSpace(raw.Choices[0].Message.Content), nil
}

func (c *Client) EmbedBatch(ctx context.Context, cfg Config, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	var raw struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	resp, err := c.r.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+cfg.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]any{
			"model": cfg.ModelName,
			"input": texts,
		}).
		SetResult(&raw).
		Post(strings.TrimRight(cfg.BaseURL, "/") + "/embeddings")
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("embeddings API %d: %s", resp.StatusCode(), resp.String())
	}
	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("embeddings API: no data returned")
	}

	out := make([][]float64, len(texts))
	for _, d := range raw.Data {
		if d.Index >= 0 && d.Index < len(out) {
			out[d.Index] = d.Embedding
		}
	}
	return out, nil
}

func (c *Client) Embed(ctx context.Context, cfg Config, text string) ([]float64, error) {
	var raw struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}

	resp, err := c.r.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+cfg.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]any{
			"model": cfg.ModelName,
			"input": text,
		}).
		SetResult(&raw).
		Post(strings.TrimRight(cfg.BaseURL, "/") + "/embeddings")
	if err != nil {
		return nil, err
	}
	if resp.IsError() {
		return nil, fmt.Errorf("embeddings API %d: %s", resp.StatusCode(), resp.String())
	}
	if len(raw.Data) == 0 {
		return nil, fmt.Errorf("embeddings API: no data returned")
	}
	return raw.Data[0].Embedding, nil
}

func (c *Client) Chat(ctx context.Context, cfg Config, systemPrompt, userPrompt string) (ChatResult, error) {
	if cfg.ProviderType == domain.ProviderDeepgram {
		return ChatResult{}, fmt.Errorf("provider type %q does not support chat completions", cfg.ProviderType)
	}

	body := map[string]any{
		"model": cfg.ModelName,
		"messages": []map[string]any{
			{"role": "system", "content": strings.TrimSpace(systemPrompt)},
			{"role": "user", "content": strings.TrimSpace(userPrompt)},
		},
		"max_tokens": 256,
	}

	var raw struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	resp, err := c.r.R().
		SetContext(ctx).
		SetHeader("Authorization", "Bearer "+cfg.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		SetResult(&raw).
		Post(strings.TrimRight(cfg.BaseURL, "/") + "/chat/completions")
	if err != nil {
		return ChatResult{}, err
	}
	if resp.IsError() {
		return ChatResult{}, fmt.Errorf("chat API %d: %s", resp.StatusCode(), resp.String())
	}
	if len(raw.Choices) == 0 {
		return ChatResult{}, fmt.Errorf("chat API: no choices returned")
	}

	content := strings.TrimSpace(raw.Choices[0].Message.Content)
	if content == "" {
		return ChatResult{}, fmt.Errorf("chat API: empty response content")
	}
	return ChatResult{Content: content}, nil
}

const visionPrompt = `Analyze these video frames. Respond with a JSON object only — no other text:
{
  "tags": ["array", "of", "descriptive", "tags", "for", "people", "actions", "objects", "locations"],
  "description": "One paragraph describing the video content, activities, people, and setting."
}`

func deepgramListenURL(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(strings.ToLower(trimmed), "/v1/listen") {
		return trimmed
	}
	return trimmed + "/v1/listen"
}

func deepgramAuthorization(apiKey string) string {
	key := strings.TrimSpace(apiKey)
	lower := strings.ToLower(key)
	if strings.HasPrefix(lower, "token ") || strings.HasPrefix(lower, "bearer ") {
		return key
	}
	return "Token " + key
}

func detectAudioContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".m4a":
		return "audio/mp4"
	case ".ogg":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}
