-- +goose Up

CREATE TABLE IF NOT EXISTS activity_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    source TEXT NOT NULL,
    subject_type TEXT NOT NULL DEFAULT '',
    subject_id UUID,
    event_type TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activity_logs_org_created
    ON activity_logs (organization_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_activity_logs_org_active
    ON activity_logs (organization_id, is_active, created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_activity_logs_org_active;
DROP INDEX IF EXISTS idx_activity_logs_org_created;
DROP TABLE IF EXISTS activity_logs;

