-- +goose Up
CREATE TABLE identity.sessions (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL,
    user_id         UUID         NOT NULL REFERENCES identity.users(id),
    token_hash      BYTEA        NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ  NOT NULL,
    revoked_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX sessions_token_hash_uq ON identity.sessions (token_hash);
CREATE        INDEX sessions_user_active_idx ON identity.sessions (tenant_id, user_id)
    WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS identity.sessions;
