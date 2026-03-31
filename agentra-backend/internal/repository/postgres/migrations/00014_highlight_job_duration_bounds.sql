-- +goose Up

ALTER TABLE highlight_jobs
    ADD COLUMN IF NOT EXISTS min_duration_sec INT NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS max_duration_sec INT NOT NULL DEFAULT 60;

UPDATE highlight_jobs
SET min_duration_sec = 30
WHERE min_duration_sec IS NULL OR min_duration_sec < 1;

UPDATE highlight_jobs
SET max_duration_sec = 60
WHERE max_duration_sec IS NULL OR max_duration_sec < 1;

-- +goose Down

ALTER TABLE highlight_jobs
    DROP COLUMN IF EXISTS min_duration_sec,
    DROP COLUMN IF EXISTS max_duration_sec;
