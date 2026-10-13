-- Add explicit capture facts to existing risk assessments and field next plans.
-- No generic risk ledger, assignment authority or Work Item lifecycle is created.
DO $$
DECLARE native_form_code text; previous workflow_form_versions%ROWTYPE; revision_id uuid; fields jsonb; next_version integer;
BEGIN
 FOREACH native_form_code IN ARRAY ARRAY['client_site_risk_assessment','client_survey_work_package','builtin_construction_daily_report'] LOOP
  PERFORM 1 FROM workflow_form_templates WHERE code=native_form_code FOR UPDATE;
  SELECT version.* INTO previous FROM workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id
  WHERE template.code=native_form_code AND version.status='published' ORDER BY version.version_no DESC LIMIT 1;
  IF previous.id IS NULL THEN CONTINUE; END IF;
  revision_id:=gen_random_uuid();
  SELECT COALESCE(MAX(version_no),0)+1 INTO next_version FROM workflow_form_versions WHERE template_id=previous.template_id;
  INSERT INTO workflow_form_versions(id,template_id,version_no,status,schema_json,validation_json,published_at,created_by)
  VALUES(revision_id,previous.template_id,next_version,'published',previous.schema_json||jsonb_build_object('native_risk_assignment_revision',1),previous.validation_json,now(),previous.created_by);
  INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json)
  SELECT revision_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json FROM workflow_form_fields WHERE form_version_id=previous.id;
  IF native_form_code='builtin_construction_daily_report' THEN
   fields:='[
    {"key":"instruction_code","label":"Mã giao việc","type":"text","required":false,"max_length":80},
    {"key":"location","label":"Vị trí và hạng mục","type":"text","required":false,"max_length":200},
    {"key":"subcontractor","label":"Thầu phụ hoặc tổ đội nhận việc","type":"text","required":false,"max_length":160},
    {"key":"receiver","label":"Đại diện nhận việc (24/7)","type":"text","required":false,"max_length":120},
    {"key":"contact","label":"Liên hệ đại diện","type":"text","required":false,"max_length":100},
    {"key":"issued_date","label":"Ngày giao việc thực tế","type":"date","required":false},
    {"key":"progress_percent","label":"Tiến độ công việc ghi nhận (%)","type":"number","required":false,"min":0,"max":100},
    {"key":"state","label":"Tình trạng công việc ghi nhận","type":"text","required":false,"max_length":80}
   ]'::jsonb;
   UPDATE workflow_form_fields SET validation_json=jsonb_set(validation_json,'{item_fields}',COALESCE(validation_json->'item_fields','[]'::jsonb)||fields) WHERE form_version_id=revision_id AND field_key='next_plan_items';
  ELSE
   fields:='[
    {"key":"issue_code","label":"Mã vấn đề hoặc rủi ro","type":"text","required":false,"max_length":80},
    {"key":"detected_date","label":"Ngày phát hiện thực tế","type":"date","required":false},
    {"key":"cause","label":"Nguyên nhân","type":"textarea","required":false,"max_length":1000},
    {"key":"impact","label":"Ảnh hưởng","type":"textarea","required":false,"max_length":1000},
    {"key":"owner","label":"Người phụ trách xử lý","type":"text","required":false,"max_length":160},
    {"key":"due_date","label":"Thời hạn xử lý","type":"date","required":false},
    {"key":"status","label":"Tình trạng ghi nhận","type":"text","required":false,"max_length":80}
   ]'::jsonb;
   UPDATE workflow_form_fields SET validation_json=jsonb_set(validation_json,'{item_fields}',COALESCE(validation_json->'item_fields','[]'::jsonb)||fields) WHERE form_version_id=revision_id AND field_key='risk_items';
  END IF;
 END LOOP;
END $$;
