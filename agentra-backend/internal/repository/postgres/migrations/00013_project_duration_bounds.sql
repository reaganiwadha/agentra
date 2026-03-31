-- +goose Up

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS editor_min_duration_sec INT NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS editor_max_duration_sec INT NOT NULL DEFAULT 60;

UPDATE projects
SET editor_min_duration_sec = 30
WHERE editor_min_duration_sec IS NULL OR editor_min_duration_sec < 1;

UPDATE projects
SET editor_max_duration_sec = 60
WHERE editor_max_duration_sec IS NULL OR editor_max_duration_sec < 1;

-- +goose Down

ALTER TABLE projects
    DROP COLUMN IF EXISTS editor_min_duration_sec,
    DROP COLUMN IF EXISTS editor_max_duration_sec;
