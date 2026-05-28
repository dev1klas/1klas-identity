-- +goose Up
CREATE TABLE identity.users (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL,
    email           TEXT         NOT NULL,
    password_hash   TEXT         NOT NULL,
    status          TEXT         NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_tenant_email_uq ON identity.users (tenant_id, email);

-- +goose Down
DROP TABLE IF EXISTS identity.users;
