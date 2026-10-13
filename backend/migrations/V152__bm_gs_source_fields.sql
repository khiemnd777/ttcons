-- Enrichment is owned by the canonical source; no report ledger or business-state mutation.
DO $$
DECLARE owner_table text;
BEGIN
 FOREACH owner_table IN ARRAY ARRAY['workflow_form_instances','operation_daily_reports','acceptance_records','construction_action_items','contract_variations','work_items','evidence_records','handover_records'] LOOP
  EXECUTE format('ALTER TABLE %I ADD COLUMN bm_gs_json jsonb NOT NULL DEFAULT ''{}''::jsonb, ADD COLUMN bm_gs_version integer NOT NULL DEFAULT 0, ADD COLUMN bm_gs_updated_by uuid REFERENCES users(id), ADD COLUMN bm_gs_updated_at timestamptz, ADD CONSTRAINT %I CHECK (jsonb_typeof(bm_gs_json) = ''object'' AND bm_gs_version >= 0)', owner_table, owner_table || '_bm_gs_valid');
 END LOOP;
END $$;

-- Only current editable sources receive stable crew identities. Immutable report
-- revisions and submitted/acknowledged/historical sources remain byte-identical.
WITH candidates AS (
 SELECT source.id,source.project_id,source.reported_by,
  jsonb_set(source.structured_json,'{crew_items}',(
   SELECT jsonb_agg(CASE WHEN NULLIF(crew.item->>'row_id','') IS NULL
    THEN crew.item || jsonb_build_object('row_id',gen_random_uuid()::text) ELSE crew.item END ORDER BY crew.ordinality)
   FROM jsonb_array_elements(source.structured_json->'crew_items') WITH ORDINALITY crew(item,ordinality)
  )) AS enriched
 FROM operation_daily_reports source
 LEFT JOIN work_reports report ON report.source_kind='operation_daily_report' AND report.source_id=source.id
 LEFT JOIN workflow_form_instances form ON form.id=source.workflow_form_instance_id
 WHERE source.report_delivery_eligible
  AND (report.id IS NULL OR report.status='needs_changes')
  AND (form.id IS NULL OR form.status IN ('draft','reopened','submitted'))
  AND jsonb_typeof(source.structured_json->'crew_items')='array'
  AND EXISTS(SELECT 1 FROM jsonb_array_elements(source.structured_json->'crew_items') crew WHERE NULLIF(crew->>'row_id','') IS NULL)
), updated AS (
 UPDATE operation_daily_reports source SET structured_json=candidate.enriched
 FROM candidates candidate WHERE source.id=candidate.id
 RETURNING source.id,source.project_id,source.reported_by
)
INSERT INTO audit_logs(project_id,actor_id,event_type,target_type,target_id,description,metadata_json)
SELECT project_id,reported_by,'bm_gs.crew_identity.backfilled','daily_report',id,
 'Bổ sung định danh tổ đội cho báo cáo hiện tại',jsonb_build_object('migration','V152','scope','editable_current_sources') FROM updated;

-- Synchronize identity metadata into the editable current form, not its immutable
-- revisions. Require exact crew facts before attaching the source row identities.
WITH synchronized AS (
 UPDATE workflow_form_instances form
 SET answers_snapshot_json=jsonb_set(form.answers_snapshot_json,'{crew_items}',source.structured_json->'crew_items')
 FROM operation_daily_reports source
 LEFT JOIN work_reports report ON report.source_kind='operation_daily_report' AND report.source_id=source.id
 WHERE source.workflow_form_instance_id=form.id
  AND source.report_delivery_eligible
  AND (report.id IS NULL OR report.status='needs_changes')
  AND form.status IN ('draft','reopened','submitted')
  AND jsonb_typeof(form.answers_snapshot_json->'crew_items')='array'
  AND jsonb_typeof(source.structured_json->'crew_items')='array'
  AND EXISTS(SELECT 1 FROM jsonb_array_elements(form.answers_snapshot_json->'crew_items') crew WHERE NULLIF(crew->>'row_id','') IS NULL)
  AND (SELECT jsonb_agg(crew.item - 'row_id' ORDER BY crew.ordinality) FROM jsonb_array_elements(form.answers_snapshot_json->'crew_items') WITH ORDINALITY crew(item,ordinality))
   = (SELECT jsonb_agg(crew.item - 'row_id' ORDER BY crew.ordinality) FROM jsonb_array_elements(source.structured_json->'crew_items') WITH ORDINALITY crew(item,ordinality))
 RETURNING form.id,form.project_id,form.created_by,form.answers_snapshot_json->'crew_items' AS crews
), answers_updated AS (
 UPDATE workflow_form_answers answer SET value_json=synchronized.crews
 FROM synchronized WHERE answer.instance_id=synchronized.id AND answer.field_key='crew_items'
 RETURNING answer.instance_id
)
INSERT INTO audit_logs(project_id,actor_id,event_type,target_type,target_id,description,metadata_json)
SELECT project_id,created_by,'bm_gs.crew_identity.backfilled','workflow_form_instance',id,
 'Đồng bộ định danh tổ đội vào hồ sơ hiện tại',jsonb_build_object('migration','V152','scope','editable_current_source_metadata') FROM synchronized;
