-- +goose Up
UPDATE media_assets SET status = 'pending' WHERE status IN ('analyzing', 'failed');
ALTER TABLE media_assets DROP CONSTRAINT media_assets_status_check;
ALTER TABLE media_assets ADD CONSTRAINT media_assets_status_check
    CHECK (status IN ('pending', 'ready'));

-- +goose Down
ALTER TABLE media_assets DROP CONSTRAINT media_assets_status_check;
ALTER TABLE media_assets ADD CONSTRAINT media_assets_status_check
    CHECK (status IN ('pending', 'analyzing', 'ready', 'failed'));
