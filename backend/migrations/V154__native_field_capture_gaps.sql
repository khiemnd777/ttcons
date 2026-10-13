-- Native field records own these observations. No parallel export ledger,
-- retrospective values or automatic quality/readiness decisions are created.
ALTER TABLE construction_action_items
 ADD COLUMN fault_code text, ADD COLUMN location text, ADD COLUMN severity text,
 ADD COLUMN responsible_unit text, ADD COLUMN representative text, ADD COLUMN contact text,
 ADD CONSTRAINT construction_action_field_severity CHECK(severity IS NULL OR severity IN('low','medium','high','critical'));
ALTER TABLE handover_checklist_items
 ADD COLUMN requirement text, ADD COLUMN physical_result text, ADD COLUMN defect text, ADD COLUMN due_date date,
 ADD CONSTRAINT handover_item_physical_result CHECK(physical_result IS NULL OR physical_result IN('pass','fail','not_assessed'));

-- Clone into new revisions. Existing template steps, runtime instances, published
-- definitions and historical answers remain pinned to their original version.
DO $$
DECLARE native_form_code text; previous workflow_form_versions%ROWTYPE; revision_id uuid; fields jsonb; next_version integer;
BEGIN
 FOREACH native_form_code IN ARRAY ARRAY['client_site_measurement_record','client_survey_work_package','builtin_subcontractor_checklist','client_subcontractor_acceptance_package'] LOOP
  PERFORM 1 FROM workflow_form_templates WHERE code=native_form_code FOR UPDATE;
  SELECT version.* INTO previous FROM workflow_form_versions version JOIN workflow_form_templates template ON template.id=version.template_id
  WHERE template.code=native_form_code AND version.status='published' ORDER BY version.version_no DESC LIMIT 1;
  IF previous.id IS NULL THEN CONTINUE; END IF;
  revision_id:=gen_random_uuid();
  SELECT COALESCE(MAX(version_no),0)+1 INTO next_version FROM workflow_form_versions WHERE template_id=previous.template_id;
  INSERT INTO workflow_form_versions(id,template_id,version_no,status,schema_json,validation_json,published_at,created_by)
  VALUES(revision_id,previous.template_id,next_version,'published',previous.schema_json||jsonb_build_object('native_field_capture_revision',1),previous.validation_json,now(),previous.created_by);
  INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json)
  SELECT revision_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json FROM workflow_form_fields WHERE form_version_id=previous.id;
  IF native_form_code IN ('client_site_measurement_record','client_survey_work_package') THEN
   fields:='[
    {"key":"mep","label":"Điểm giao kỹ thuật MEP","type":"textarea","required":false,"max_length":1000},
    {"key":"existing_condition","label":"Kết cấu và hoàn thiện hiện hữu","type":"textarea","required":false,"max_length":1000},
    {"key":"assessment","label":"Đánh giá và yêu cầu xử lý","type":"textarea","required":false,"max_length":1000}
   ]'::jsonb;
   UPDATE workflow_form_fields SET validation_json=jsonb_set(validation_json,'{item_fields}',COALESCE(validation_json->'item_fields','[]'::jsonb)||fields) WHERE form_version_id=revision_id AND field_key='dimension_items';
   INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,validation_json,order_index)
   VALUES(revision_id,'survey_unit','Đơn vị khảo sát','text',false,'{"max_length":160}'::jsonb,390),
    (revision_id,'client_representative','Đại diện khách hàng','text',false,'{"max_length":120}'::jsonb,391);
  ELSE
   fields:='[
    {"key":"standard_reference","label":"Tiêu chuẩn áp dụng","type":"text","required":false,"max_length":300},
    {"key":"drawing_reference","label":"Bản vẽ và revision","type":"text","required":false,"max_length":300},
    {"key":"technical_requirement","label":"Yêu cầu kỹ thuật","type":"textarea","required":false,"max_length":1000},
    {"key":"design_quantity","label":"Khối lượng thiết kế","type":"number","required":false,"min":0},
    {"key":"actual_quantity","label":"Khối lượng thực tế đo","type":"number","required":false,"min":0},
    {"key":"measurement_note","label":"Căn cứ và ghi chú đo","type":"textarea","required":false,"max_length":1000}
   ]'::jsonb;
   UPDATE workflow_form_fields SET validation_json=jsonb_set(validation_json,'{item_fields}',COALESCE(validation_json->'item_fields','[]'::jsonb)||fields) WHERE form_version_id=revision_id AND field_key='checklist_items';
  END IF;
 END LOOP;
END $$;
