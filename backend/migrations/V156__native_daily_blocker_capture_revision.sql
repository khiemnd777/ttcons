-- Expand capture on a new published native daily form revision only. Existing
-- field requirements, published definitions and runtime/history pins stay intact.
DO $$
DECLARE previous workflow_form_versions%ROWTYPE; revision_id uuid; next_version integer;
BEGIN
 PERFORM 1 FROM workflow_form_templates WHERE code='builtin_construction_daily_report' FOR UPDATE;
 SELECT version.* INTO previous FROM workflow_form_versions version
 JOIN workflow_form_templates template ON template.id=version.template_id
 WHERE template.code='builtin_construction_daily_report' AND version.status='published'
 ORDER BY version.version_no DESC LIMIT 1;
 IF previous.id IS NULL THEN RETURN; END IF;
 SELECT COALESCE(MAX(version_no),0)+1 INTO next_version FROM workflow_form_versions WHERE template_id=previous.template_id;
 revision_id:=gen_random_uuid();
 INSERT INTO workflow_form_versions(id,template_id,version_no,status,schema_json,validation_json,published_at,created_by)
 VALUES(revision_id,previous.template_id,next_version,'published',
  previous.schema_json||jsonb_build_object('native_risk_assignment_revision',1,'native_daily_blocker_revision',1),previous.validation_json,now(),previous.created_by);
 INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json)
 SELECT revision_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json
 FROM workflow_form_fields WHERE form_version_id=previous.id;
 UPDATE workflow_form_fields SET validation_json=jsonb_set(validation_json,'{item_fields}',
  COALESCE(validation_json->'item_fields','[]'::jsonb)||'[
   {"key":"risk_code","label":"Mã vấn đề hoặc rủi ro","type":"text","required":false,"max_length":80},
   {"key":"detected_at","label":"Ngày phát hiện thực tế","type":"date","required":false},
   {"key":"cause","label":"Nguyên nhân","type":"textarea","required":false,"max_length":1000},
   {"key":"proposed_action","label":"Hướng xử lý đề xuất","type":"textarea","required":false,"max_length":1000},
   {"key":"owner","label":"Người phụ trách xử lý","type":"text","required":false,"max_length":160},
   {"key":"status","label":"Tình trạng ghi nhận","type":"text","required":false,"max_length":80}
  ]'::jsonb)
 WHERE form_version_id=revision_id AND field_key='blockers';
END $$;
