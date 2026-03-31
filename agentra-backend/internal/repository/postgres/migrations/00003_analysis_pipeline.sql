-- +goose Up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE model_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name TEXT NOT NULL,
    provider_type TEXT NOT NULL CHECK (provider_type IN ('openai_compat', 'deepgram', 'other')),
    base_url TEXT NOT NULL,
    api_key TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);

CREATE TABLE analyzers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name TEXT NOT NULL,
    analyzer_type TEXT NOT NULL CHECK (analyzer_type IN ('transcription', 'vision_tags', 'embedding')),
    provider_id UUID NOT NULL REFERENCES model_providers(id),
    model_name TEXT NOT NULL,
    config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, name)
);

CREATE INDEX idx_analyzers_org_enabled_type ON analyzers (organization_id, is_enabled, analyzer_type);
CREATE INDEX idx_model_providers_org_active ON model_providers (organization_id, is_active);

-- +goose Down

DROP INDEX IF EXISTS idx_model_providers_org_active;
DROP INDEX IF EXISTS idx_analyzers_org_enabled_type;
DROP TABLE IF EXISTS analyzers;
DROP TABLE IF EXISTS model_providers;
