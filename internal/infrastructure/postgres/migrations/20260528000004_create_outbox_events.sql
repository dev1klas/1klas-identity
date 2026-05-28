-- +goose Up
CREATE TABLE identity.outbox_events (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID         NOT NULL,
    aggregate_type  TEXT         NOT NULL,
    aggregate_id    UUID         NOT NULL,
    event_type      TEXT         NOT NULL,
    payload         JSONB        NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ
);
CREATE INDEX outbox_unpublished_idx ON identity.outbox_events (created_at)
    WHERE published_at IS NULL;
-- Tenant-scoped consumer reads need a (tenant_id, created_at) index so they
-- can scan a single tenant's events ordered by time without a full sequential
-- scan + filter. Partitioning by month is a separate follow-up.
CREATE INDEX outbox_tenant_created_idx ON identity.outbox_events (tenant_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS identity.outbox_events;
