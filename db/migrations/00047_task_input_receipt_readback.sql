-- +goose Up
ALTER TABLE task_input_delivery_attempts
 DROP CONSTRAINT task_input_delivery_attempts_outcome_check;
ALTER TABLE task_input_delivery_attempts
 ADD CONSTRAINT task_input_delivery_attempts_outcome_check
 CHECK(outcome IN ('prepared','provider_delivered','local_context_loaded','outcome_unknown','not_sent','not_required'));
ALTER TABLE task_input_delivery_attempts
 DROP CONSTRAINT task_input_delivery_attempts_check;
ALTER TABLE task_input_delivery_attempts
 ADD CONSTRAINT task_input_delivery_attempts_check
 CHECK(
  (phase='prepared' AND outcome='prepared' AND provider_egress=0)
  OR (phase='final' AND outcome IN ('provider_delivered','local_context_loaded','outcome_unknown','not_sent','not_required')
      AND ((outcome IN ('provider_delivered','outcome_unknown') AND provider_egress>0)
        OR (outcome IN ('local_context_loaded','not_sent','not_required') AND provider_egress=0)))
 );

-- +goose Down
ALTER TABLE task_input_delivery_attempts
 DROP CONSTRAINT task_input_delivery_attempts_check;
ALTER TABLE task_input_delivery_attempts
 ADD CONSTRAINT task_input_delivery_attempts_check
 CHECK(
  (phase='prepared' AND outcome='prepared' AND provider_egress=0)
  OR (phase='final' AND outcome IN ('provider_delivered','local_context_loaded','outcome_unknown','not_sent')
      AND ((outcome IN ('provider_delivered','outcome_unknown') AND provider_egress>0)
        OR (outcome IN ('local_context_loaded','not_sent') AND provider_egress=0)))
 );
ALTER TABLE task_input_delivery_attempts
 DROP CONSTRAINT task_input_delivery_attempts_outcome_check;
ALTER TABLE task_input_delivery_attempts
 ADD CONSTRAINT task_input_delivery_attempts_outcome_check
 CHECK(outcome IN ('prepared','provider_delivered','local_context_loaded','outcome_unknown','not_sent'));
