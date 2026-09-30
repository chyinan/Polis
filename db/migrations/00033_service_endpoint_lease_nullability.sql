-- +goose Up
-- Preserve any legacy rows without rewriting immutable endpoint evidence while
-- enforcing the complete lease/nullability rule for every new observation.
ALTER TABLE service_endpoint_events
ADD CONSTRAINT service_endpoint_events_lease_consistency
CHECK (
 (readiness='revoked' AND lease_expires_at IS NULL) OR
 (readiness<>'revoked' AND lease_expires_at IS NOT NULL)
) NOT VALID;

-- +goose Down
ALTER TABLE service_endpoint_events DROP CONSTRAINT service_endpoint_events_lease_consistency;
