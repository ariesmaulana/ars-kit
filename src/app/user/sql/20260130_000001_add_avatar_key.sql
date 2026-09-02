-- +goose Up
-- Avatar key: storage-relative key of the uploaded profile photo (e.g.
-- "avatars/<xid>.jpg"). NULL = no avatar set. The bytes themselves live in the
-- upload backend (S3/R2/local), not in the database.
ALTER TABLE users
    ADD COLUMN avatar_key VARCHAR(255) NULL;

-- +goose Down
