-- +goose Up

ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS thumbnail_path TEXT,
    ADD COLUMN IF NOT EXISTS mime_type TEXT,
    ADD COLUMN IF NOT EXISTS captured_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_media_assets_project_captured_at
    ON media_assets (project_id, captured_at DESC, created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_media_assets_project_captured_at;

ALTER TABLE media_assets
    DROP COLUMN IF EXISTS captured_at,
    DROP COLUMN IF EXISTS mime_type,
    DROP COLUMN IF EXISTS thumbnail_path;
