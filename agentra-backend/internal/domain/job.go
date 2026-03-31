package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gopkg.in/guregu/null.v4"
)

type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

type HighlightJob struct {
	ID             uuid.UUID   `db:"id"            json:"id"`
	ProjectID      uuid.UUID   `db:"project_id"    json:"project_id"`
	RequestedBy    uuid.UUID   `db:"requested_by"  json:"requested_by"`
	Prompt         string      `db:"prompt"        json:"prompt"`
	VariantIndex   int         `db:"variant_index" json:"variant_index"`
	VariantCount   int         `db:"variant_count" json:"variant_count"`
	MinDurationSec int         `db:"min_duration_sec" json:"min_duration_sec"`
	MaxDurationSec int         `db:"max_duration_sec" json:"max_duration_sec"`
	Status         JobStatus   `db:"status"        json:"status"`
	ErrorMessage   null.String `db:"error_message" json:"error_message"`
	StartedAt      null.Time   `db:"started_at"    json:"started_at"`
	FinishedAt     null.Time   `db:"finished_at"   json:"finished_at"`
	CreatedAt      time.Time   `db:"created_at"    json:"created_at"`
}

type Timeline struct {
	ID        uuid.UUID `db:"id"         json:"id"`
	JobID     uuid.UUID `db:"job_id"     json:"job_id"`
	OTIOJson  []byte    `db:"otio_json"  json:"otio_json"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Render struct {
	ID            uuid.UUID  `db:"id"              json:"id"`
	JobID         uuid.UUID  `db:"job_id"          json:"job_id"`
	OutputPath    string     `db:"output_path"     json:"output_path"`
	DurationSec   null.Float `db:"duration_sec"    json:"duration_sec"`
	FileSizeBytes null.Int   `db:"file_size_bytes" json:"file_size_bytes"`
	CreatedAt     time.Time  `db:"created_at"      json:"created_at"`
}

type HighlightJobTrace struct {
	ID        uuid.UUID       `db:"id"         json:"id"`
	JobID     uuid.UUID       `db:"job_id"     json:"job_id"`
	Phase     string          `db:"phase"      json:"phase"`
	Message   string          `db:"message"    json:"message"`
	Payload   json.RawMessage `db:"payload"    json:"payload"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
}
