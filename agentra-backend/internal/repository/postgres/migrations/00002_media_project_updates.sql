-- +goose Up

-- Projects get a configurable storage subfolder for the scanner
ALTER TABLE projects ADD COLUMN storage_subpath TEXT;

-- SHA256 is computed during analysis, not during scanning
ALTER TABLE media_assets ALTER COLUMN sha256 DROP NOT NULL;
ALTER TABLE media_assets DROP CONSTRAINT media_assets_project_id_sha256_key;
ALTER TABLE media_assets ADD CONSTRAINT media_assets_project_id_storage_path_key UNIQUE (project_id, storage_path);

-- +goose Down

ALTER TABLE media_assets DROP CONSTRAINT IF EXISTS media_assets_project_id_storage_path_key;
ALTER TABLE media_assets ADD CONSTRAINT media_assets_project_id_sha256_key UNIQUE (project_id, sha256);
ALTER TABLE media_assets ALTER COLUMN sha256 SET NOT NULL;
ALTER TABLE projects DROP COLUMN IF EXISTS storage_subpath;
