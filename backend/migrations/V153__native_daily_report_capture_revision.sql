-- Add a new native form revision. Published versions, pinned runtime forms,
-- source answers and immutable report/form revisions are not rewritten.
DO $$
DECLARE previous workflow_form_versions%ROWTYPE; revision_id uuid; crew_validation jsonb; next_version integer;
BEGIN
 PERFORM 1 FROM workflow_form_templates WHERE code='builtin_construction_daily_report' FOR UPDATE;
 SELECT version.* INTO previous FROM workflow_form_versions version
 JOIN workflow_form_templates template ON template.id=version.template_id
 WHERE template.code='builtin_construction_daily_report' AND version.status='published'
 ORDER BY version.version_no DESC LIMIT 1;
 IF previous.id IS NULL THEN RETURN; END IF;
 revision_id:=gen_random_uuid();
 SELECT COALESCE(MAX(version_no),0)+1 INTO next_version FROM workflow_form_versions WHERE template_id=previous.template_id;
 INSERT INTO workflow_form_versions(id,template_id,version_no,status,schema_json,validation_json,published_at,created_by)
 VALUES(revision_id,previous.template_id,next_version,'published',
  previous.schema_json || jsonb_build_object('native_capture_revision',1),previous.validation_json,now(),previous.created_by);
 INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json)
 SELECT revision_id,field_key,label,field_type,required,options_json,validation_json,help_text,order_index,visibility_condition_json
 FROM workflow_form_fields WHERE form_version_id=previous.id;
 SELECT validation_json INTO crew_validation FROM workflow_form_fields WHERE form_version_id=revision_id AND field_key='crew_items';
 UPDATE workflow_form_fields SET validation_json=jsonb_set(crew_validation,'{item_fields}',
  COALESCE(crew_validation->'item_fields','[]'::jsonb)||'[
   {"key":"representative","label":"Đại diện tổ đội (24/7)","type":"text","required":false,"max_length":120},
   {"key":"contact","label":"Liên hệ đại diện","type":"text","required":false,"max_length":100}
  ]'::jsonb)
 WHERE form_version_id=revision_id AND field_key='crew_items';
 INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required,validation_json,help_text,order_index)
 VALUES(revision_id,'supervision_assessments','Đánh giá giám sát trong ngày','structured_list',false,'{
  "max_items":12,"item_fields":[
   {"key":"code","label":"Nội dung giám sát","type":"select","required":true,"options":[
    {"value":"progress","label":"Tiến độ so với kế hoạch"},{"value":"workforce","label":"Nhân công và tổ đội"},
    {"value":"deliveries","label":"Vật tư và thiết bị đến hiện trường"},{"value":"quality","label":"Chất lượng thi công"},
    {"value":"completed_quantity","label":"Khối lượng hoàn thành"},{"value":"variation","label":"Phát sinh ngoài hợp đồng"},
    {"value":"mep","label":"Giao diện kỹ thuật MEP"},{"value":"safety","label":"An toàn và vệ sinh"},
    {"value":"acceptance","label":"Nghiệm thu nội bộ và tồn tại"},{"value":"risk","label":"Vấn đề và rủi ro"},
    {"value":"images","label":"Ảnh hiện trường"},{"value":"instructions","label":"Giao việc hiện trường"}]},
   {"key":"assessment","label":"Đánh giá","type":"select","required":true,"options":[
    {"value":"pass","label":"Đạt"},{"value":"fail","label":"Không đạt"},{"value":"not_applicable","label":"Không áp dụng"},{"value":"not_assessed","label":"Chưa đánh giá"}]},
   {"key":"note","label":"Ghi chú đánh giá","type":"textarea","required":false,"max_length":1000}
  ]}'::jsonb,'Chỉ ghi nội dung đã quan sát. Không bắt buộc đủ 12 nội dung; không thay thế quyết định nghiệm thu.',125),
 (revision_id,'material_arrival_items','Vật tư và thiết bị đến hiện trường','structured_list',false,'{
  "max_items":200,"item_fields":[
   {"key":"material_name","label":"Vật tư hoặc thiết bị","type":"text","required":true,"max_length":160},
   {"key":"specification","label":"Quy cách","type":"text","required":false,"max_length":300},
   {"key":"origin","label":"Nguồn gốc","type":"text","required":false,"max_length":200},
   {"key":"quantity","label":"Số lượng đến hiện trường","type":"number","required":false,"min":0},
   {"key":"unit","label":"Đơn vị tính","type":"text","required":false,"max_length":30},
   {"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":800}
  ]}'::jsonb,'Ghi nhận hàng đến hiện trường, tách biệt vật tư sử dụng. Không tạo phiếu kho.',130);
END $$;
