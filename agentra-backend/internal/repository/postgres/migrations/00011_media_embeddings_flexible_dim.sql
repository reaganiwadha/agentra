-- +goose Up

-- Drop the dimension-constrained column and HNSW index.
-- Replacing with an untyped vector column accepts any embedding model dimension.
-- Existing rows are cleared — assets will be re-embedded on next analyzer run.
DROP INDEX IF EXISTS media_embeddings_hnsw;
TRUNCATE TABLE media_embeddings;
ALTER TABLE media_embeddings DROP COLUMN embedding;
ALTER TABLE media_embeddings ADD COLUMN embedding vector NOT NULL;

-- +goose Down
ALTER TABLE media_embeddings DROP COLUMN embedding;
ALTER TABLE media_embeddings ADD COLUMN embedding vector(1536) NOT NULL;
CREATE INDEX media_embeddings_hnsw
    ON media_embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);
