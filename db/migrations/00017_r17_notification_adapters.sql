-- +goose Up
ALTER TABLE notification_routes
  ADD COLUMN config jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE notification_routes
  DROP CONSTRAINT IF EXISTS notification_routes_adapter_check;

ALTER TABLE notification_routes
  ADD CONSTRAINT notification_routes_adapter_check CHECK (adapter IN ('local', 'webhook', 'qq_official'));

ALTER TABLE notification_routes
  DROP CONSTRAINT IF EXISTS notification_routes_status_check;

ALTER TABLE notification_routes
  ADD CONSTRAINT notification_routes_status_check CHECK (status IN ('unverified', 'configured', 'ready', 'failed'));

-- +goose Down
ALTER TABLE notification_routes
  DROP CONSTRAINT IF EXISTS notification_routes_adapter_check;

ALTER TABLE notification_routes
  ADD CONSTRAINT notification_routes_adapter_check CHECK (adapter IN ('local', 'webhook'));

ALTER TABLE notification_routes
  DROP CONSTRAINT IF EXISTS notification_routes_status_check;

ALTER TABLE notification_routes
  ADD CONSTRAINT notification_routes_status_check CHECK (status IN ('unverified', 'configured', 'ready', 'failed'));

ALTER TABLE notification_routes
  DROP COLUMN IF EXISTS config;
