-- +goose Up

ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS organization_id UUID;

UPDATE media_assets AS m
SET organization_id = p.organization_id
FROM projects AS p
WHERE m.project_id = p.id
  AND m.organization_id IS NULL;

ALTER TABLE media_assets
    ALTER COLUMN project_id DROP NOT NULL;

ALTER TABLE media_assets
    DROP CONSTRAINT IF EXISTS media_assets_project_id_storage_path_key;

CREATE INDEX IF NOT EXISTS idx_media_assets_org_storage_path
    ON media_assets (organization_id, storage_path);

CREATE INDEX IF NOT EXISTS idx_media_assets_org_captured_at
    ON media_assets (organization_id, captured_at DESC, created_at DESC);

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS media_scope_mode TEXT NOT NULL DEFAULT 'global',
    ADD COLUMN IF NOT EXISTS media_scope_start_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS media_scope_end_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS project_media_scope_items (
    project_id UUID NOT NULL,
    media_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_media_scope_items_project_id
    ON project_media_scope_items (project_id);

CREATE INDEX IF NOT EXISTS idx_project_media_scope_items_media_id
    ON project_media_scope_items (media_id);

-- +goose Down

DROP INDEX IF EXISTS idx_project_media_scope_items_media_id;
DROP INDEX IF EXISTS idx_project_media_scope_items_project_id;
DROP TABLE IF EXISTS project_media_scope_items;

ALTER TABLE projects
    DROP COLUMN IF EXISTS media_scope_end_at,
    DROP COLUMN IF EXISTS media_scope_start_at,
    DROP COLUMN IF EXISTS media_scope_mode;

DROP INDEX IF EXISTS idx_media_assets_org_captured_at;
DROP INDEX IF EXISTS idx_media_assets_org_storage_path;

ALTER TABLE media_assets
    ADD CONSTRAINT media_assets_project_id_storage_path_key UNIQUE (project_id, storage_path);

ALTER TABLE media_assets
    ALTER COLUMN project_id SET NOT NULL;

ALTER TABLE media_assets
    DROP COLUMN IF EXISTS organization_id;
