CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'user',
    avatar_url TEXT
);

CREATE TABLE IF NOT EXISTS groups (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    created_by_id UUID NOT NULL REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS group_memberships (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL REFERENCES users(id),
    group_id UUID NOT NULL REFERENCES groups(id),
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    CONSTRAINT idx_user_group UNIQUE (user_id, group_id)
);

CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL DEFAULT 0,
    is_directory BOOLEAN NOT NULL DEFAULT FALSE,
    parent_id UUID REFERENCES files(id),
    owner_id UUID NOT NULL REFERENCES users(id),
    storage_path TEXT NOT NULL,
    thumbnail_path TEXT
);

CREATE TABLE IF NOT EXISTS shares (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    file_id UUID NOT NULL REFERENCES files(id),
    shared_by_id UUID NOT NULL REFERENCES users(id),
    shared_with_user_id UUID REFERENCES users(id),
    shared_with_group_id UUID REFERENCES groups(id),
    permission VARCHAR(20) NOT NULL DEFAULT 'view',
    expires_at TIMESTAMPTZ,
    CONSTRAINT share_target_check CHECK (
        (shared_with_user_id IS NOT NULL AND shared_with_group_id IS NULL)
        OR
        (shared_with_user_id IS NULL AND shared_with_group_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_groups_deleted_at ON groups(deleted_at);
CREATE INDEX IF NOT EXISTS idx_group_memberships_deleted_at ON group_memberships(deleted_at);
CREATE INDEX IF NOT EXISTS idx_files_deleted_at ON files(deleted_at);
CREATE INDEX IF NOT EXISTS idx_shares_deleted_at ON shares(deleted_at);
