-- Rev7 correction: the returned scan is recorded by Contract Dispatches and
-- must not repeat the hardcopy dispatch form already completed one step prior.

UPDATE pipeline_template_steps step
SET form_version_id=NULL,
    form_required=false,
    checklist_required=false,
    document_required=false,
    domain_action_key='contract.require_execution_ready'
FROM pipeline_template_nodes node
WHERE node.id=step.node_id
  AND node.template_id='40000000-0000-0000-0000-000000000026'
  AND step.business_code='outbound_contract.returned_scan';

DO $$
DECLARE
  actual integer;
BEGIN
  SELECT count(*) INTO actual
  FROM pipeline_template_nodes node
  JOIN pipeline_template_steps step ON step.node_id=node.id
  WHERE node.template_id='40000000-0000-0000-0000-000000000026'
    AND step.business_code='outbound_contract.returned_scan'
    AND step.form_version_id IS NULL
    AND NOT step.form_required
    AND NOT step.checklist_required
    AND NOT step.document_required
    AND step.domain_action_key='contract.require_execution_ready';
  IF actual<>1 THEN
    RAISE EXCEPTION 'Rev7 returned-scan step was not converted to the Contract source gate';
  END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_nodes node
  JOIN pipeline_template_steps step ON step.node_id=node.id
  JOIN workflow_form_fields field ON field.form_version_id=step.form_version_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000026';
  IF actual<>87 THEN
    RAISE EXCEPTION 'expected final Rev7 to expose 87 field occurrences, found %',actual;
  END IF;
END $$;
