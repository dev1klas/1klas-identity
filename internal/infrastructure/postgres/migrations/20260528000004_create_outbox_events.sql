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

-- +goose Down
DROP TABLE IF EXISTS identity.outbox_events;
