-- +goose Up
ALTER TABLE service_endpoint_events
ADD CONSTRAINT service_endpoint_events_requires_lease
CHECK(readiness='revoked' OR lease_expires_at IS NOT NULL);

-- +goose Down
ALTER TABLE service_endpoint_events DROP CONSTRAINT service_endpoint_events_requires_lease;
