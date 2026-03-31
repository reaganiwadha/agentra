package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gopkg.in/guregu/null.v4"
)

type MediaStatus string

const (
	MediaPending MediaStatus = "pending"
	MediaReady   MediaStatus = "ready"
)

type MediaAsset struct {
	ID             uuid.UUID     `db:"id"              json:"id"`
	OrganizationID uuid.UUID     `db:"organization_id" json:"organization_id"`
	ProjectID      uuid.NullUUID `db:"project_id"      json:"project_id"`
	Filename       string        `db:"filename"        json:"filename"`
	StoragePath    string        `db:"storage_path"    json:"storage_path"`
	ThumbnailPath  null.String   `db:"thumbnail_path"  json:"thumbnail_path"`
	MIMEType       null.String   `db:"mime_type"       json:"mime_type"`
	SHA256         null.String   `db:"sha256"          json:"sha256"`
	DurationSec    null.Float    `db:"duration_sec"    json:"duration_sec"`
	FileSizeBytes  null.Int      `db:"file_size_bytes" json:"file_size_bytes"`
	CapturedAt     null.Time     `db:"captured_at"     json:"captured_at"`
	Status         MediaStatus   `db:"status"          json:"status"`
	CreatedAt      time.Time     `db:"created_at"      json:"created_at"`
	UpdatedAt      time.Time     `db:"updated_at"      json:"updated_at"`
}

type MediaAnalysisResult struct {
	ID           uuid.UUID       `db:"id"            json:"id"`
	MediaID      uuid.UUID       `db:"media_id"      json:"media_id"`
	AnalyzerID   uuid.UUID       `db:"analyzer_id"   json:"analyzer_id"`
	AnalyzerName string          `db:"analyzer_name" json:"analyzer_name"`
	AnalyzerType string          `db:"analyzer_type" json:"analyzer_type"`
	Output       json.RawMessage `db:"output"        json:"output"`
	AnalyzedAt   time.Time       `db:"analyzed_at"   json:"analyzed_at"`
}

type MediaEmbedding struct {
	ID           uuid.UUID  `db:"id"`
	MediaID      uuid.UUID  `db:"media_id"`
	AnalyzerID   uuid.UUID  `db:"analyzer_id"`
	SegmentIndex *int       `db:"segment_index"`
	StartSec     *float64   `db:"start_sec"`
	EndSec       *float64   `db:"end_sec"`
	SourceText   string     `db:"source_text"`
	CreatedAt    time.Time  `db:"created_at"`
}

type EmbeddingSearchHit struct {
	MediaID      uuid.UUID `db:"media_id"      json:"media_id"`
	Filename     string    `db:"filename"      json:"filename"`
	StoragePath  string    `db:"storage_path"  json:"storage_path"`
	SegmentIndex *int      `db:"segment_index" json:"segment_index,omitempty"`
	StartSec     *float64  `db:"start_sec"     json:"start_sec,omitempty"`
	EndSec       *float64  `db:"end_sec"       json:"end_sec,omitempty"`
	SourceText   string    `db:"source_text"   json:"source_text"`
	Score        float64   `db:"score"         json:"score"`
}
