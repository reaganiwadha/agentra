-- +goose Up

CREATE TABLE IF NOT EXISTS highlight_job_traces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id UUID NOT NULL REFERENCES highlight_jobs(id) ON DELETE CASCADE,
    phase TEXT NOT NULL,
    message TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_highlight_job_traces_job_id_created_at
    ON highlight_job_traces (job_id, created_at ASC);

-- +goose Down

DROP TABLE IF EXISTS highlight_job_traces;
