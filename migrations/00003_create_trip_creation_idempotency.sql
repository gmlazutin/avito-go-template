-- +goose Up
CREATE TABLE trip_creation_idempotency (
    key          UUID PRIMARY KEY,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    trip_id      UUID NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    expires_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT trip_creation_idempotency_trip_fk
        FOREIGN KEY (trip_id)
        REFERENCES trips(id)
        ON DELETE CASCADE
        DEFERRABLE INITIALLY DEFERRED,
    CHECK (expires_at > created_at)
);

CREATE INDEX trip_creation_idempotency_expires_at_idx
    ON trip_creation_idempotency (expires_at);

-- +goose Down
DROP TABLE trip_creation_idempotency;
