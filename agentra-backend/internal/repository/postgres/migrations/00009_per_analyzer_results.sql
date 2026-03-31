-- +goose Up
CREATE TABLE media_analysis_results (
    id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    media_id      UUID        NOT NULL REFERENCES media_assets(id)  ON DELETE CASCADE,
    analyzer_id   UUID        NOT NULL REFERENCES analyzers(id)     ON DELETE CASCADE,
    analyzer_name TEXT        NOT NULL,
    analyzer_type TEXT        NOT NULL,
    output        JSONB       NOT NULL,
    analyzed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (media_id, analyzer_id)
);
DROP TABLE IF EXISTS media_analysis;

-- +goose Down
DROP TABLE IF EXISTS media_analysis_results;
CREATE TABLE media_analysis (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    media_id       UUID        NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    transcript     JSONB,
    vision_tags    JSONB,
    embedding      JSONB,
    analysis_model TEXT        NOT NULL,
    analyzed_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
