-- +goose Up

ALTER TABLE editor_configs
    ADD COLUMN IF NOT EXISTS provider_id UUID,
    ADD COLUMN IF NOT EXISTS model_name TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_editor_configs_provider_id
    ON editor_configs (provider_id);

-- +goose Down

DROP INDEX IF EXISTS idx_editor_configs_provider_id;

ALTER TABLE editor_configs
    DROP COLUMN IF EXISTS model_name,
    DROP COLUMN IF EXISTS provider_id;

