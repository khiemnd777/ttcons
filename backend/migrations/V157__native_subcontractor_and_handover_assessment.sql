-- Optional field observations and explicit decisions do not alter readiness.
ALTER TABLE handover_records
 ADD COLUMN handover_stage text,
 ADD COLUMN handover_conclusion text,
 ADD COLUMN handover_conclusion_by uuid REFERENCES users(id),
 ADD COLUMN handover_conclusion_at timestamptz,
 ADD CONSTRAINT handover_stage_length CHECK(handover_stage IS NULL OR char_length(handover_stage)<=160),
 ADD CONSTRAINT handover_conclusion_value CHECK(handover_conclusion IS NULL OR handover_conclusion IN('eligible','not_eligible'));

-- Publish new definitions; never rewrite old versions, answers or runtime pins.
DO $$
DECLARE form_code text; previous workflow_form_versions%ROWTYPE; revision_id uuid; next_version integer; first_order integer; last_order integer;
BEGIN
 FOREACH form_code IN ARRAY ARRAY['builtin_subcontractor_checklist','client_subcontractor_acceptance_package'] LOOP
  PERFORM 1 FROM workflow_form_templates WHERE code=form_code FOR UPDATE;
  SELECT v.* INTO previous FROM workflow_form_versions v JOIN workflow_form_templates t ON t.id=v.template_id
   WHERE t.code=form_code AND v.status='published' ORDER BY v.version_no DESC LIMIT 1;
  IF previous.id IS NULL THEN CONTINUE; END IF;
  SELECT COALESCE(MAX(version_no),0)+1 INTO next_version FROM workflow_form_versions WHERE template_id=previous.template_id;
  revision_id:=gen_random_uuid();
  INSERT INTO workflow_form_versions(id,template_id,version_no,status,schema_json,validation_json,published_at,created_by)
   VALUES(revision_id,previous.template_id,next_version,'published',previous.schema_json||jsonb_build_object('native_subcontractor_assessment_revision',1),previous.validation_json,now(),previous.created_by);
  INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json)
   SELECT revision_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json FROM workflow_form_fields WHERE form_version_id=previous.id;
  SELECT COALESCE(MIN(order_index),0),COALESCE(MAX(order_index),0) INTO first_order,last_order FROM workflow_form_fields WHERE form_version_id=previous.id;
  INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,options_json,validation_json,order_index)
   VALUES(revision_id,'reference_no','Số biên bản','text',false,'[]'::jsonb,'{"max_length":160}'::jsonb,first_order-1),
    (revision_id,'acceptance_conclusion','Kết luận nghiệm thu','select',false,'[{"value":"accepted","label":"Đồng ý nghiệm thu"},{"value":"rework_required","label":"Cần sửa chữa tồn tại"}]'::jsonb,'{}'::jsonb,last_order+1);
  UPDATE workflow_form_fields SET validation_json=jsonb_set(validation_json,'{item_fields}',COALESCE(validation_json->'item_fields','[]'::jsonb)||'[
   {"key":"quality_result","label":"Kết quả","type":"select","required":false,"options":[{"value":"pass","label":"Đạt"},{"value":"fail","label":"Không đạt"},{"value":"not_assessed","label":"Chưa đánh giá"}]},
   {"key":"quality_note","label":"Nhận xét nghiệm thu","type":"textarea","required":false,"max_length":1000}
  ]'::jsonb) WHERE form_version_id=revision_id AND field_key='checklist_items';
 END LOOP;
END $$;
