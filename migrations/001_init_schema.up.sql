CREATE TABLE IF NOT EXISTS comments (
    id         BIGSERIAL PRIMARY KEY,
    parent_id  BIGINT NULL REFERENCES comments(id) ON DELETE CASCADE,
    path_id    UUID NOT NULL UNIQUE,
    path       TEXT NOT NULL,
    comment    TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);