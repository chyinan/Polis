INSERT INTO human_interventions(
 company_id,id,mission_id,problem_key,occurrence,severity,reason_code,affected_scope,
 protection_action,required_action,evidence_refs,state,state_revision,expires_at
)
VALUES(
 'r1-input-browser','abcdef0123456789abcdef0123456789',NULL,'browser-manual-seed-v2',1,
 'high','handover_required','company','no_mutation','reauthorize_in_workbench','[]'::jsonb,'open',1,clock_timestamp()+interval '1 day'
);

INSERT INTO notification_intents(
 company_id,id,event_kind,subject_id,payload,state,intervention_id,dedupe_key,expires_at
)
VALUES(
 'r1-input-browser','notification-browser-seed-v2','human_intervention.open','abcdef0123456789abcdef0123456789',
 '{"reason_code":"handover_required"}'::jsonb,'pending','abcdef0123456789abcdef0123456789','browser-seed-key-v2',clock_timestamp()+interval '1 day'
);
