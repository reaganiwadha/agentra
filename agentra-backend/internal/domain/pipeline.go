package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gopkg.in/guregu/null.v4"
)

type ProviderType string

const (
	ProviderOpenAICompat ProviderType = "openai_compat"
	ProviderDeepgram     ProviderType = "deepgram"
	ProviderOther        ProviderType = "other"
)

type AnalyzerType string

const (
	AnalyzerTranscription AnalyzerType = "transcription"
	AnalyzerVisionTags    AnalyzerType = "vision_tags"
	AnalyzerEmbedding     AnalyzerType = "embedding"
)

type ModelProvider struct {
	ID             uuid.UUID    `db:"id"              json:"id"`
	OrganizationID uuid.UUID    `db:"organization_id" json:"organization_id"`
	Name           string       `db:"name"            json:"name"`
	ProviderType   ProviderType `db:"provider_type"   json:"provider_type"`
	BaseURL        string       `db:"base_url"        json:"base_url"`
	APIKey         null.String  `db:"api_key"         json:"-"`
	IsActive       bool         `db:"is_active"       json:"is_active"`
	HasKey         bool         `db:"-"               json:"has_api_key"`
	CreatedAt      time.Time    `db:"created_at"      json:"created_at"`
	UpdatedAt      time.Time    `db:"updated_at"      json:"updated_at"`
}

type Analyzer struct {
	ID               uuid.UUID       `db:"id"                 json:"id"`
	OrganizationID   uuid.UUID       `db:"organization_id"    json:"organization_id"`
	Name             string          `db:"name"               json:"name"`
	AnalyzerType     AnalyzerType    `db:"analyzer_type"      json:"analyzer_type"`
	ProviderID       uuid.UUID       `db:"provider_id"        json:"provider_id"`
	ModelName        string          `db:"model_name"         json:"model_name"`
	ConfigJSON       json.RawMessage `db:"config_json"     json:"config_json"`
	IsEnabled        bool            `db:"is_enabled"         json:"is_enabled"`
	CreatedAt        time.Time       `db:"created_at"         json:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at"         json:"updated_at"`
	ProviderName     string          `db:"provider_name"      json:"provider_name"`
	ProviderType     ProviderType    `db:"provider_type"      json:"provider_type"`
	ProviderIsActive bool            `db:"provider_is_active" json:"provider_is_active"`
}

type EnabledAnalyzer struct {
	ID           uuid.UUID       `db:"id"`
	Name         string          `db:"name"`
	AnalyzerType AnalyzerType    `db:"analyzer_type"`
	ProviderID   uuid.UUID       `db:"provider_id"`
	ProviderName string          `db:"provider_name"`
	ProviderType ProviderType    `db:"provider_type"`
	BaseURL      string          `db:"base_url"`
	APIKey       null.String     `db:"api_key"`
	ModelName    string          `db:"model_name"`
	ConfigJSON   json.RawMessage `db:"config_json"`
	CreatedAt    time.Time       `db:"created_at"`
}
