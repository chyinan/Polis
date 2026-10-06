-- +goose Up
-- Delivery feedback on a live Mission enters the existing formal
-- MissionChangeRequest lifecycle. These append-only route records make the
-- eventual successor Mission and revision Task causally visible without
-- creating a second executable Task in the current Mission.
ALTER TABLE delivery_manifest_revisions
 ADD CONSTRAINT delivery_manifest_revisions_route_identity UNIQUE(company_id,delivery_id,revision,mission_id);
ALTER TABLE mission_change_requests
 ADD CONSTRAINT mission_change_requests_route_identity UNIQUE(company_id,change_request_id,mission_id);

CREATE TABLE delivery_revision_routes (
 company_id text NOT NULL,
 route_id text NOT NULL,
 delivery_id text NOT NULL,
 manifest_revision bigint NOT NULL CHECK(manifest_revision>0),
 disposition_revision bigint NOT NULL CHECK(disposition_revision>0),
 mission_id text NOT NULL,
 change_request_id text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,route_id),
 UNIQUE(company_id,delivery_id,manifest_revision,disposition_revision),
 FOREIGN KEY(company_id,delivery_id,manifest_revision,mission_id) REFERENCES delivery_manifest_revisions(company_id,delivery_id,revision,mission_id),
 FOREIGN KEY(company_id,delivery_id,manifest_revision,disposition_revision) REFERENCES delivery_user_dispositions(company_id,delivery_id,manifest_revision,revision),
 FOREIGN KEY(company_id,change_request_id,mission_id) REFERENCES mission_change_requests(company_id,change_request_id,mission_id)
);
CREATE INDEX delivery_revision_routes_delivery ON delivery_revision_routes(company_id,delivery_id,created_at DESC,route_id);
CREATE TABLE delivery_revision_route_events (
 company_id text NOT NULL,
 event_seq bigserial NOT NULL,
 event_id text NOT NULL,
 route_id text NOT NULL,
 state text NOT NULL CHECK(state IN ('change_request_pending','successor_mission_created','revision_task_ready')),
 successor_mission_id text,
 task_id text,
 reason_code text NOT NULL CHECK(reason_code ~ '^[a-z0-9_:-]{1,128}$'),
 request_id text NOT NULL CHECK(request_id ~ '^[A-Za-z0-9_-]{1,80}$'),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(company_id,event_seq),
 UNIQUE(company_id,event_id),
 UNIQUE(company_id,request_id),
 FOREIGN KEY(company_id,route_id) REFERENCES delivery_revision_routes(company_id,route_id),
 FOREIGN KEY(company_id,successor_mission_id) REFERENCES missions(company_id,id),
 FOREIGN KEY(company_id,task_id,successor_mission_id) REFERENCES tasks(company_id,id,mission_id),
 CHECK(state='change_request_pending' OR successor_mission_id IS NOT NULL),
 CHECK(state<>'revision_task_ready' OR task_id IS NOT NULL)
);
CREATE INDEX delivery_revision_route_events_latest ON delivery_revision_route_events(company_id,route_id,event_seq DESC);
-- +goose StatementBegin
CREATE FUNCTION validate_delivery_revision_route_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
 manifest_mission text;
 disposition_state text;
 change_mission text;
BEGIN
 SELECT mission_id INTO manifest_mission
 FROM delivery_manifest_revisions
 WHERE company_id=NEW.company_id AND delivery_id=NEW.delivery_id AND revision=NEW.manifest_revision;
 SELECT state INTO disposition_state
 FROM delivery_user_dispositions
 WHERE company_id=NEW.company_id AND delivery_id=NEW.delivery_id AND manifest_revision=NEW.manifest_revision AND revision=NEW.disposition_revision;
 SELECT mission_id INTO change_mission
 FROM mission_change_requests
 WHERE company_id=NEW.company_id AND change_request_id=NEW.change_request_id;
 IF manifest_mission IS NULL OR change_mission IS NULL OR manifest_mission<>NEW.mission_id OR change_mission<>NEW.mission_id OR disposition_state<>'changes_requested' THEN
  RAISE EXCEPTION 'delivery revision route scope is invalid' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER delivery_revision_routes_scope
 BEFORE INSERT ON delivery_revision_routes
 FOR EACH ROW EXECUTE FUNCTION validate_delivery_revision_route_scope();
-- +goose StatementBegin
CREATE FUNCTION validate_delivery_revision_route_event_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
 successor text;
BEGIN
 IF NEW.state='successor_mission_created' THEN
  IF NOT EXISTS(
   SELECT 1 FROM delivery_revision_routes r
   JOIN mission_change_request_events e ON e.company_id=r.company_id AND e.change_request_id=r.change_request_id
   WHERE r.company_id=NEW.company_id AND r.route_id=NEW.route_id AND e.state='applied' AND e.successor_mission_id=NEW.successor_mission_id
  ) THEN
   RAISE EXCEPTION 'delivery revision route successor is not an applied ChangeRequest successor' USING ERRCODE='23514';
  END IF;
 ELSIF NEW.state='revision_task_ready' THEN
  SELECT e.successor_mission_id INTO successor
  FROM delivery_revision_route_events e
  WHERE e.company_id=NEW.company_id AND e.route_id=NEW.route_id AND e.state='successor_mission_created'
  ORDER BY e.event_seq DESC LIMIT 1;
  IF successor IS NULL OR successor<>NEW.successor_mission_id OR NOT EXISTS(
   SELECT 1 FROM tasks t WHERE t.company_id=NEW.company_id AND t.id=NEW.task_id AND t.mission_id=NEW.successor_mission_id
  ) THEN
   RAISE EXCEPTION 'delivery revision route task is not bound to its successor Mission' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER delivery_revision_route_events_scope
 BEFORE INSERT ON delivery_revision_route_events
 FOR EACH ROW EXECUTE FUNCTION validate_delivery_revision_route_event_scope();
CREATE TRIGGER delivery_revision_routes_immutable
 BEFORE UPDATE OR DELETE ON delivery_revision_routes
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER delivery_revision_routes_no_truncate
 BEFORE TRUNCATE ON delivery_revision_routes FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER delivery_revision_route_events_immutable
 BEFORE UPDATE OR DELETE ON delivery_revision_route_events
 FOR EACH ROW EXECUTE FUNCTION reject_task_validation_binding_mutation();
CREATE TRIGGER delivery_revision_route_events_no_truncate
 BEFORE TRUNCATE ON delivery_revision_route_events FOR EACH STATEMENT
 EXECUTE FUNCTION reject_task_validation_binding_mutation();

-- +goose Down
DO $$
BEGIN
 IF EXISTS(SELECT 1 FROM delivery_revision_route_events) OR EXISTS(SELECT 1 FROM delivery_revision_routes) THEN
  RAISE EXCEPTION 'cannot discard delivery revision route history';
 END IF;
END
$$;
DROP TRIGGER delivery_revision_route_events_no_truncate ON delivery_revision_route_events;
DROP TRIGGER delivery_revision_route_events_immutable ON delivery_revision_route_events;
DROP TRIGGER delivery_revision_route_events_scope ON delivery_revision_route_events;
DROP FUNCTION validate_delivery_revision_route_event_scope();
DROP INDEX delivery_revision_route_events_latest;
DROP TABLE delivery_revision_route_events;
DROP TRIGGER delivery_revision_routes_no_truncate ON delivery_revision_routes;
DROP TRIGGER delivery_revision_routes_immutable ON delivery_revision_routes;
DROP TRIGGER delivery_revision_routes_scope ON delivery_revision_routes;
DROP FUNCTION validate_delivery_revision_route_scope();
DROP INDEX delivery_revision_routes_delivery;
DROP TABLE delivery_revision_routes;
ALTER TABLE mission_change_requests DROP CONSTRAINT mission_change_requests_route_identity;
ALTER TABLE delivery_manifest_revisions DROP CONSTRAINT delivery_manifest_revisions_route_identity;
