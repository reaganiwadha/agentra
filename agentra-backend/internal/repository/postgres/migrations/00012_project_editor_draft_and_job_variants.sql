-- +goose Up

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS editor_base_prompt TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS editor_variant_count INT NOT NULL DEFAULT 1;

ALTER TABLE highlight_jobs
    DROP CONSTRAINT IF EXISTS highlight_jobs_status_check;

ALTER TABLE highlight_jobs
    ADD COLUMN IF NOT EXISTS variant_index INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS variant_count INT NOT NULL DEFAULT 1;

ALTER TABLE highlight_jobs
    ADD CONSTRAINT highlight_jobs_status_check
    CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled'));

-- +goose Down

ALTER TABLE highlight_jobs
    DROP CONSTRAINT IF EXISTS highlight_jobs_status_check;

ALTER TABLE highlight_jobs
    DROP COLUMN IF EXISTS variant_index,
    DROP COLUMN IF EXISTS variant_count;

ALTER TABLE highlight_jobs
    ADD CONSTRAINT highlight_jobs_status_check
    CHECK (status IN ('queued', 'running', 'completed', 'failed'));

ALTER TABLE projects
    DROP COLUMN IF EXISTS editor_base_prompt,
    DROP COLUMN IF EXISTS editor_variant_count;
