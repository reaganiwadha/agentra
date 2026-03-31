-- +goose Up

-- pgvector extension must be available on the PostgreSQL server.
-- Install with: CREATE EXTENSION vector; (requires pgvector package on the server)
CREATE EXTENSION IF NOT EXISTS vector;

-- Stores vector embeddings for media assets at two granularities:
--   segment_index IS NULL  → whole-asset embedding (transcript + vision summary)
--   segment_index = N      → per-transcript-segment embedding with start/end timestamps
--
-- Embedding dimension is 1536 (OpenAI text-embedding-3-small / ada-002).
-- If your embedding model uses a different dimension, change 1536 below before running.
CREATE TABLE media_embeddings (
    id            UUID             PRIMARY KEY DEFAULT uuid_generate_v4(),
    media_id      UUID             NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
    analyzer_id   UUID             NOT NULL REFERENCES analyzers(id)    ON DELETE CASCADE,
    segment_index INT,
    start_sec     DOUBLE PRECISION,
    end_sec       DOUBLE PRECISION,
    source_text   TEXT             NOT NULL,
    embedding     vector(1536)     NOT NULL,
    created_at    TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

-- One whole-asset embedding per (media, analyzer)
CREATE UNIQUE INDEX media_embeddings_whole_asset_uniq
    ON media_embeddings (media_id, analyzer_id)
    WHERE segment_index IS NULL;

-- One segment embedding per (media, analyzer, segment)
CREATE UNIQUE INDEX media_embeddings_segment_uniq
    ON media_embeddings (media_id, analyzer_id, segment_index)
    WHERE segment_index IS NOT NULL;

-- HNSW index for fast approximate nearest-neighbour search on cosine distance.
-- ef_construction=64 is a good starting point; increase for better recall at index-build cost.
CREATE INDEX media_embeddings_hnsw
    ON media_embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- +goose Down
DROP TABLE IF EXISTS media_embeddings;
DROP EXTENSION IF EXISTS vector;
