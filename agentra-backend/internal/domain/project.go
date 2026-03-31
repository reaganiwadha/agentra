package domain

import (
	"time"

	"github.com/google/uuid"
	"gopkg.in/guregu/null.v4"
)

type ProjectStatus string

const (
	ProjectActive   ProjectStatus = "active"
	ProjectArchived ProjectStatus = "archived"
)

type MediaScopeMode string

const (
	MediaScopeGlobal    MediaScopeMode = "global"
	MediaScopeDateRange MediaScopeMode = "date_range"
	MediaScopeSelected  MediaScopeMode = "selected"
)

type Project struct {
	ID                   uuid.UUID      `db:"id"              json:"id"`
	OrganizationID       uuid.UUID      `db:"organization_id" json:"organization_id"`
	Name                 string         `db:"name"            json:"name"`
	Description          null.String    `db:"description"     json:"description"`
	EditorBasePrompt     string         `db:"editor_base_prompt" json:"editor_base_prompt"`
	EditorVariantCount   int            `db:"editor_variant_count" json:"editor_variant_count"`
	EditorMinDurationSec int            `db:"editor_min_duration_sec" json:"editor_min_duration_sec"`
	EditorMaxDurationSec int            `db:"editor_max_duration_sec" json:"editor_max_duration_sec"`
	StorageSubpath       null.String    `db:"storage_subpath" json:"storage_subpath"`
	MediaScopeMode       MediaScopeMode `db:"media_scope_mode" json:"media_scope_mode"`
	MediaScopeStartAt    null.Time      `db:"media_scope_start_at" json:"media_scope_start_at"`
	MediaScopeEndAt      null.Time      `db:"media_scope_end_at" json:"media_scope_end_at"`
	Status               ProjectStatus  `db:"status"          json:"status"`
	CreatedBy            uuid.UUID      `db:"created_by"      json:"created_by"`
	CreatedAt            time.Time      `db:"created_at"      json:"created_at"`
	UpdatedAt            time.Time      `db:"updated_at"      json:"updated_at"`
}
