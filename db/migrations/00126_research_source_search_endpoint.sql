-- +goose Up
-- A search endpoint is optional source metadata. It is still inert until an
-- explicitly injected, qualified SearchBackend consumes it.
ALTER TABLE research_source_bindings ADD COLUMN search_endpoint text;
ALTER TABLE research_source_bindings ADD CONSTRAINT research_source_search_endpoint_check CHECK (
 search_endpoint IS NULL OR
 (search_endpoint ~ '^https://[^/?#@[:space:]]+/.+' AND search_endpoint !~ '[?#@[:space:]]'));

-- +goose Down
ALTER TABLE research_source_bindings DROP CONSTRAINT research_source_search_endpoint_check;
ALTER TABLE research_source_bindings DROP COLUMN search_endpoint;
