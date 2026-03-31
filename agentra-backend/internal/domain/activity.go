package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type ActivitySource string

const (
	ActivitySourceAnalyzerLoop ActivitySource = "analyzer.loop"
)

type ActivityEventType string

const (
	ActivityEventStarted   ActivityEventType = "started"
	ActivityEventProgress  ActivityEventType = "progress"
	ActivityEventCompleted ActivityEventType = "completed"
	ActivityEventFailed    ActivityEventType = "failed"
	ActivityEventInfo      ActivityEventType = "info"
)

type ActivityLog struct {
	ID             uuid.UUID         `db:"id"               json:"id"`
	OrganizationID uuid.UUID         `db:"organization_id"  json:"organization_id"`
	Source         ActivitySource    `db:"source"           json:"source"`
	SubjectType    string            `db:"subject_type"     json:"subject_type"`
	SubjectID      uuid.NullUUID     `db:"subject_id"       json:"subject_id"`
	EventType      ActivityEventType `db:"event_type"       json:"event_type"`
	Level          string            `db:"level"            json:"level"`
	Message        string            `db:"message"          json:"message"`
	Payload        json.RawMessage   `db:"payload"          json:"payload"`
	IsActive       bool              `db:"is_active"        json:"is_active"`
	CreatedAt      time.Time         `db:"created_at"       json:"created_at"`
}
