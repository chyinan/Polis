-- +goose Up
-- The read projection is bounded per delivery; lead with the delivery key so
-- the cap does not require scanning the whole Company backlog.
CREATE INDEX delivery_feedback_backlog_delivery_latest
 ON delivery_feedback_backlog_events(company_id,delivery_id,event_seq DESC);

-- +goose Down
DROP INDEX delivery_feedback_backlog_delivery_latest;
