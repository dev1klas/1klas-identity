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
-- Deliberately global (no tenant_id prefix). Session tokens come from a CSPRNG
-- with 256 bits of entropy (see internal/infrastructure/tokens) so the chance
-- of collision across tenants is negligible, and the cookie-based lookup at
-- request time happens BEFORE tenant is resolved (the cookie value is the
-- only thing we have to identify the caller). Do not "fix" this to include
-- tenant_id — doing so would force the lookup to know the tenant up front,
-- which is precisely what session resolution is meant to discover.
CREATE UNIQUE INDEX sessions_token_hash_uq ON identity.sessions (token_hash);
CREATE        INDEX sessions_user_active_idx ON identity.sessions (tenant_id, user_id)
    WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS identity.sessions;
