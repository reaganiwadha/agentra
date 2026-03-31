package domain

import (
	"time"

	"github.com/google/uuid"
)

type EditorConfig struct {
	ID                  uuid.UUID     `db:"id"                    json:"id"`
	OrganizationID      uuid.UUID     `db:"organization_id"       json:"organization_id"`
	ProviderID          uuid.NullUUID `db:"provider_id"          json:"provider_id"`
	ModelName           string        `db:"model_name"            json:"model_name"`
	BasePrompt          string        `db:"base_prompt"           json:"base_prompt"`
	MaxDurationSec      int           `db:"max_duration_sec"      json:"max_duration_sec"`
	IsAutonomousEnabled bool          `db:"is_autonomous_enabled" json:"is_autonomous_enabled"`
	CreatedAt           time.Time     `db:"created_at"            json:"created_at"`
	UpdatedAt           time.Time     `db:"updated_at"            json:"updated_at"`
}
