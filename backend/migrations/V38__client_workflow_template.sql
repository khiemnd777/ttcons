WITH template_seed(code, name, description, form_type) AS (
  VALUES (
    'client_design_branch_decision',
    'Quyết định nhánh thiết kế',
    'Form chốt workflow đi nhánh Có thiết kế hoặc Không thiết kế sau họp báo cáo khảo sát.',
    'generic_submission'
  )
)
INSERT INTO workflow_form_templates (id, code, name, description, form_type, status)
SELECT
  '80000000-0000-0000-0000-000000000020'::uuid,
  code,
  name,
  description,
  form_type,
  'active'
FROM template_seed
ON CONFLICT (code) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    form_type=EXCLUDED.form_type,
    status='active',
    updated_at=now();

INSERT INTO workflow_form_versions (id, template_id, version_no, status, schema_json, validation_json, published_at)
SELECT
  '80000000-0000-0000-0000-000000000021'::uuid,
  id,
  1,
  'published',
  '{"source":"client_official_workflow","purpose":"branch_routing"}'::jsonb,
  '{"routing_field":"needs_design"}'::jsonb,
  now()
FROM workflow_form_templates
WHERE code='client_design_branch_decision'
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH decision_version AS (
  SELECT v.id
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1
  WHERE t.code='client_design_branch_decision'
),
field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('needs_design', 'Nhánh xử lý sau khảo sát', 'select', true,
      '[{"value":"yes","label":"Có thiết kế"},{"value":"no","label":"Không thiết kế"}]'::jsonb,
      '{}'::jsonb,
      'Chọn đúng nhánh để workflow chuyển tiếp.',
      10),
    ('decision_note', 'Ghi chú quyết định', 'textarea', false,
      '[]'::jsonb,
      '{}'::jsonb,
      'Tóm tắt lý do chọn nhánh, người chốt và căn cứ họp.',
      20)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT v.id, f.field_key, f.label, f.field_type, f.required, f.options_json, f.validation_json, f.help_text, f.order_index
FROM field_seed f
CROSS JOIN decision_version v
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;

INSERT INTO pipeline_templates (id, name, description, project_type, version, status, created_by)
VALUES (
  '40000000-0000-0000-0000-000000000020',
  '[Client] Workflow hoàn thiện công trình chính thức',
  'Workflow chính thức từ vận hành thực tế của client: khảo sát, nhánh thiết kế/không thiết kế, BOQ, đầu vào bắt buộc, thi công, nghiệm thu, bàn giao và đóng tài chính.',
  'Hoàn thiện công trình',
  1,
  'published',
  NULL
)
ON CONFLICT (id) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    project_type=EXCLUDED.project_type,
    version=EXCLUDED.version,
    status=EXCLUDED.status,
    updated_at=now();

WITH node_seed(id, name, description, type, order_index, position_x, position_y, roles, required_permission, permissions, exit_conditions, approval_rule, rework_enabled, replanning_enabled, fail_path) AS (
  VALUES
    ('41400000-0000-0000-0000-000000000001'::uuid, 'Khảo sát', 'Ghi nhận hiện trạng, kích thước và rủi ro ban đầu. Đầu ra là biên bản khảo sát.', 'survey', 1, 80, 260, '["PM","Giám sát","QS"]'::jsonb, 'planning.edit', '["planning.edit"]'::jsonb, '{"text":"Có yêu cầu chủ đầu tư, ảnh hiện trạng, kích thước, rủi ro và biên bản khảo sát"}'::jsonb, 'PM duyệt khảo sát', false, true, 'previous'),
    ('41400000-0000-0000-0000-000000000002'::uuid, 'Họp báo cáo khảo sát', 'Họp thống nhất vấn đề, lên báo giá, phân việc và chốt nhánh có thiết kế hay không thiết kế.', 'survey', 2, 360, 260, '["PM","QS","Giám sát"]'::jsonb, 'planning.edit', '["planning.edit","boq.view"]'::jsonb, '{"text":"Biên bản họp đã chốt nhánh Có thiết kế hoặc Không thiết kế"}'::jsonb, 'PM chốt biên bản họp', false, true, 'previous'),
    ('41400000-0000-0000-0000-000000000003'::uuid, 'Thiết kế', 'Chuẩn bị bản vẽ, phương án thiết kế và xác nhận với khách hàng.', 'design', 3, 640, 420, '["PM","Chủ đầu tư"]'::jsonb, 'planning.edit', '["planning.edit"]'::jsonb, '{"text":"Bản vẽ mặt bằng, 3D, bản vẽ thi công và chốt chủ đầu tư đã hoàn tất"}'::jsonb, 'PM và chủ đầu tư xác nhận thiết kế', false, true, 'replanning'),
    ('41400000-0000-0000-0000-000000000004'::uuid, 'BOQ (Không thiết kế)', 'Bóc tách, lấy giá và gửi BOQ khi công trình không cần thiết kế.', 'boq', 4, 640, 120, '["QS","PM","Giám đốc"]'::jsonb, 'boq.update', '["boq.view","boq.update","boq.approve"]'::jsonb, '{"text":"BOQ không thiết kế đã được kiểm tra và gửi chủ đầu tư"}'::jsonb, 'QS/PM/GĐ duyệt BOQ không thiết kế', false, true, 'previous'),
    ('41400000-0000-0000-0000-000000000005'::uuid, 'BOQ (Có thiết kế)', 'Gửi BOQ qua chủ đầu tư sau khi hoàn tất thiết kế.', 'boq', 5, 920, 420, '["QS","PM"]'::jsonb, 'boq.update', '["boq.view","boq.update"]'::jsonb, '{"text":"BOQ có thiết kế đã gửi chủ đầu tư"}'::jsonb, 'PM/QS xác nhận BOQ có thiết kế', false, true, 'previous'),
    ('41400000-0000-0000-0000-000000000006'::uuid, 'Đầu Vào: Tổ Đội / Thầu Phụ / Cung cấp vật tư', 'Bước bắt buộc để chốt khối lượng, mua hàng, hợp đồng/tạm ứng và review đầu vào.', 'procurement', 6, 1200, 260, '["QS","Mua hàng","PM","Giám đốc"]'::jsonb, 'procurement.create', '["boq.view","procurement.view","procurement.create","procurement.approve"]'::jsonb, '{"mandatory_gate":true,"text":"Đầu vào tổ đội/thầu phụ/nhà cung cấp vật tư đã được review và check"}'::jsonb, 'PM/GĐ review đầu vào', false, false, 'previous'),
    ('41400000-0000-0000-0000-000000000007'::uuid, 'Đàm phán giá', 'Đàm phán giá cùng với chủ đầu tư và các bộ phận liên quan.', 'budget_approval', 7, 1480, 260, '["PM","QS","Giám đốc","Kế toán"]'::jsonb, 'boq.approve', '["boq.view","boq.approve","accounting.budget_view"]'::jsonb, '{"text":"Đơn giá và định mức đã chốt"}'::jsonb, 'GĐ chốt báo cáo đơn giá', false, false, 'previous'),
    ('41400000-0000-0000-0000-000000000008'::uuid, 'Chốt Hợp đồng với chủ đầu tư', 'Lên báo giá, hợp đồng, pháp lý, ký duyệt và gửi bản cứng cho chủ đầu tư.', 'contract', 8, 1760, 260, '["PM","QS","Kế toán","Giám đốc","Chủ đầu tư"]'::jsonb, 'project.update', '["project.update","accounting.view"]'::jsonb, '{"text":"Hợp đồng và hồ sơ tạm ứng với chủ đầu tư đã ký/gửi"}'::jsonb, 'PM/GĐ ký duyệt hợp đồng', false, false, 'previous'),
    ('41400000-0000-0000-0000-000000000009'::uuid, 'QA & QC / Thi công', 'Họp phân công, chốt biên bản, đảm bảo chất lượng, kế hoạch thi công và báo cáo liên tục.', 'construction', 9, 2040, 260, '["PM","QS","Giám sát","QA/QC"]'::jsonb, 'operation.report', '["operation.view","operation.report","operation.issue_create"]'::jsonb, '{"text":"QA/QC và báo cáo thi công đã hoàn tất"}'::jsonb, 'PM/QA review thi công', true, true, 'rework'),
    ('41400000-0000-0000-0000-000000000010'::uuid, 'Nghiệm thu Đầu Vào', 'Nghiệm thu thầu phụ/tổ đội/vật tư, kiểm tra khối lượng, hồ sơ thanh toán và thanh toán thầu phụ.', 'incoming_acceptance', 10, 2320, 260, '["QA/QC","QS","Kế toán","Giám đốc"]'::jsonb, 'acceptance.approve', '["acceptance.view","acceptance.create","acceptance.approve","accounting.payable_view"]'::jsonb, '{"text":"Nghiệm thu đầu vào và thanh toán thầu phụ đã kiểm tra"}'::jsonb, 'GĐ ký nghiệm thu hồ sơ thanh toán', true, true, 'rework'),
    ('41400000-0000-0000-0000-000000000011'::uuid, 'Nghiệm thu Đầu Ra', 'Nghiệm thu công trình, hồ sơ nghiệm thu, quyết toán, hóa đơn và thanh toán với chủ đầu tư.', 'acceptance', 11, 2600, 260, '["QA/QC","QS","PM","Giám đốc","Kế toán"]'::jsonb, 'acceptance.approve', '["acceptance.view","acceptance.create","acceptance.approve","accounting.receivable_view"]'::jsonb, '{"text":"Nghiệm thu đầu ra, quyết toán và thanh toán chủ đầu tư đã ghi nhận"}'::jsonb, 'QA/QS/PM/GĐ ký nghiệm thu đầu ra', true, true, 'rework'),
    ('41400000-0000-0000-0000-000000000012'::uuid, 'Kết thúc: Bàn giao + Đóng tài chính', 'Bàn giao, xuất hóa đơn đầu vào/đầu ra và đóng tài chính đầy đủ.', 'financial_closure', 12, 2880, 260, '["Kế toán","PM","Giám đốc"]'::jsonb, 'accounting.payment_approve', '["handover.view","handover.approve","accounting.view","accounting.payment_approve"]'::jsonb, '{"handover_included":true,"financial_closure_included":true,"text":"Bàn giao và đóng tài chính đã hoàn tất"}'::jsonb, 'Kế toán/GĐ xác nhận đóng tài chính', false, false, 'none')
)
INSERT INTO pipeline_template_nodes (
  id, template_id, name, description, type, order_index, position_x, position_y,
  responsible_role_ids, required_permission, permissions, exit_conditions, approval_rule,
  rework_enabled, replanning_enabled, fail_path
)
SELECT id, '40000000-0000-0000-0000-000000000020', name, description, type, order_index, position_x, position_y,
  roles, required_permission, permissions, exit_conditions, approval_rule, rework_enabled, replanning_enabled, fail_path
FROM node_seed
ON CONFLICT (id) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    type=EXCLUDED.type,
    order_index=EXCLUDED.order_index,
    position_x=EXCLUDED.position_x,
    position_y=EXCLUDED.position_y,
    responsible_role_ids=EXCLUDED.responsible_role_ids,
    required_permission=EXCLUDED.required_permission,
    permissions=EXCLUDED.permissions,
    exit_conditions=EXCLUDED.exit_conditions,
    approval_rule=EXCLUDED.approval_rule,
    rework_enabled=EXCLUDED.rework_enabled,
    replanning_enabled=EXCLUDED.replanning_enabled,
    fail_path=EXCLUDED.fail_path;

WITH form_versions AS (
  SELECT t.code, v.id AS version_id
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1
  WHERE t.code IN ('builtin_generic_submission', 'client_design_branch_decision')
),
step_seed(id, node_id, order_index, name, description, step_type, roles, permission_key, checklist_required, document_required, photo_required, form_code, form_required) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000001'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,1,'K/s Yêu cầu của chủ đầu tư (Primary: PM, GĐ. Secondary: Qs, Sale, Đội kỹ thuật)','Ghi nhận yêu cầu chính của chủ đầu tư và phạm vi khảo sát.','task','["PM","Giám đốc","QS","Giám sát"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000002'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,2,'Khảo sát / Chụp / Upload ảnh hiện trạng','Chụp và upload ảnh hiện trạng.','photo_upload','["Giám sát"]'::jsonb,'planning.edit',false,false,true,NULL,false),
    ('41500000-0000-0000-0000-000000000003'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,3,'Thiết kế nắm phong cách (Thiết kế)','Ghi nhận phong cách thiết kế mong muốn.','task','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000004'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,4,'Ghi nhận kích thước chính (Qs, Thiết kế)','Ghi nhận kích thước chính phục vụ BOQ/thiết kế.','task','["QS","PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000005'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,5,'Xác định rủi ro mặt bằng (Qs, Đội kỹ thuật)','Xác định rủi ro mặt bằng và lưu ý kỹ thuật.','task','["QS","Giám sát"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000006'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,6,'Biên bản khảo sát','Hoàn tất biên bản khảo sát.','document_upload','["PM","Giám sát"]'::jsonb,'planning.edit',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000007'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,1,'QS / Bốc khối lượng không Thiết kế','Bóc khối lượng phục vụ nhánh không thiết kế.','task','["QS"]'::jsonb,'boq.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000008'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,2,'Thiết kế / Lên bảng bố trí mặt bằng','Lên bố trí mặt bằng sơ bộ nếu cần thiết kế.','task','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000009'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,3,'Thiết kế / Lên bảng vẽ 3D','Lên bản vẽ 3D sơ bộ nếu cần thiết kế.','task','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000010'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,4,'Thiết kế / Lên bảng vẽ thi công','Lên bản vẽ thi công sơ bộ nếu cần thiết kế.','task','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000011'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,5,'Biên bản cuộc họp','Chốt biên bản cuộc họp và nhánh xử lý tiếp theo.','meeting_minutes','["PM","QS","Giám sát"]'::jsonb,'planning.edit',true,false,false,'client_design_branch_decision',true),
    ('41500000-0000-0000-0000-000000000012'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,1,'Trảo đổi / Lên bảng vẽ bố trí mặt bằng','Trao đổi và lên bản vẽ bố trí mặt bằng.','design_review','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000013'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,2,'Trảo đổi / Lên bảng vẽ 3D','Trao đổi và lên bản vẽ 3D.','design_review','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000014'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,3,'Trảo đổi / Lên bảng vẽ thi công','Trao đổi và upload bản vẽ thi công.','document_upload','["PM"]'::jsonb,'planning.edit',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000015'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,4,'Chốt với Sale / Chủ đầu tư','Chốt phương án với sale/chủ đầu tư.','approval','["PM","Chủ đầu tư"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000016'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,5,'Đưa bảng vẽ qua bên QS.','Bàn giao bản vẽ cho QS để làm BOQ.','task','["PM","QS"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000017'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,1,'QS Lấy giá từ thầu phụ / giá tổ đội / vật tư','Lấy giá từ thầu phụ, tổ đội và vật tư.','task','["QS"]'::jsonb,'boq.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000018'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,2,'QS / PM / GĐ Kiểm tra đơn giá để ra được đầu vào hợp lý.','Kiểm tra đơn giá và đầu vào hợp lý.','budget_check','["QS","PM","Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000019'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,3,'GĐ Review định mức quản lý','Giám đốc review định mức quản lý.','approval','["Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000020'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,4,'QS / PM / GĐ Gửi BOQ qua chủ đầu tư','Gửi BOQ qua chủ đầu tư.','document_upload','["QS","PM","Giám đốc"]'::jsonb,'boq.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000021'::uuid,'41400000-0000-0000-0000-000000000005'::uuid,1,'QS / Sale / PM Gửi BOQ qua Chủ đầu tư','Gửi BOQ qua chủ đầu tư sau thiết kế.','document_upload','["QS","PM"]'::jsonb,'boq.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000022'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,1,'QS / Bốc Khối Lượng','Bóc khối lượng đầu vào.','task','["QS"]'::jsonb,'boq.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000023'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,2,'Mua Hàng / QS Mua hàng với Tổ Đội / Thầu Phụ / Cung cấp vật tư','Làm việc mua hàng với tổ đội, thầu phụ, nhà cung cấp vật tư.','material_check','["Mua hàng","QS"]'::jsonb,'procurement.create',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000024'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,3,'Mua hàng / QS Ký hợp đồng / Hồ sơ tạm ứng với Tổ Đội / Thầu Phụ / Nhà Cung Cấp Vật Tự','Ký hợp đồng và chuẩn bị hồ sơ tạm ứng đầu vào.','document_upload','["Mua hàng","QS","Kế toán"]'::jsonb,'procurement.create',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000025'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,4,'PM / GĐ Review & Check Đầu Vào','PM/GĐ review và check đầu vào.','approval','["PM","Giám đốc"]'::jsonb,'procurement.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000026'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,1,'Sale / QS / PM / GĐ Đàm phán Thầu phụ để có giá hợp lý','Đàm phán để có giá hợp lý.','budget_check','["QS","PM","Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000027'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,2,'QS / PM Chốt lại định mức','Chốt lại định mức sau đàm phán.','budget_check','["QS","PM"]'::jsonb,'boq.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000028'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,3,'GĐ Review & Chốt báo cáo đơn giá','Giám đốc review và chốt báo cáo đơn giá.','approval','["Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000029'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,1,'QS Lên báo giá & hợp đồng','Lên báo giá và hợp đồng với chủ đầu tư.','document_upload','["QS","PM"]'::jsonb,'project.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000030'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,2,'Kế toán / Pháp lý check lại hợp đồng','Kế toán/pháp lý kiểm tra hợp đồng.','task','["Kế toán","PM"]'::jsonb,'accounting.view',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000031'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,3,'Sale / PM / QS Gửi nội dung file mềm qua (bảng tiến độ / đề nghị tạm ứng / ký duyệt của 2 bên) Chủ đầu tư','Gửi file mềm hợp đồng, tiến độ, tạm ứng và ký duyệt hai bên cho chủ đầu tư.','document_upload','["PM","QS"]'::jsonb,'project.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000032'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,4,'PM / GĐ review và ký duyệt','PM/GĐ review và ký duyệt.','approval','["PM","Giám đốc"]'::jsonb,'project.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000033'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,5,'In và gửi bảng cứng (hợp đồng / hồ sơ tạm ứng) cho Chủ đầu tư','In và gửi bản cứng hợp đồng/hồ sơ tạm ứng cho chủ đầu tư.','document_upload','["PM","Kế toán"]'::jsonb,'project.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000034'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,1,'QS / PM / Thiết Kế / QA Họp & phân công','Họp và phân công QA/QC, QS, PM, thiết kế.','task','["QS","PM","QA/QC"]'::jsonb,'operation.report',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000035'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,2,'Chốt biên bản cuộc họp','Chốt biên bản cuộc họp thi công/QA.','document_upload','["PM","QA/QC"]'::jsonb,'operation.report',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000036'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,3,'[QA] Đảm bảo chất lượng / kế hoạch thi công','QA đảm bảo chất lượng và kế hoạch thi công.','daily_report','["QA/QC","Giám sát"]'::jsonb,'operation.report',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000037'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,4,'[QA] Báo cáo liên tục trong quá trình thi công','QA báo cáo liên tục trong quá trình thi công.','daily_report','["QA/QC","Giám sát"]'::jsonb,'operation.report',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000038'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,1,'[QA, QS] Nghiệm thu Thầu Phụ / Tổ Đội ( Đưa số liệu cho QS kiểm tra)','Nghiệm thu thầu phụ/tổ đội và đưa số liệu cho QS kiểm tra.','acceptance_check','["QA/QC","QS"]'::jsonb,'acceptance.create',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000039'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,2,'[QA, QS] Nghiệm thu Vật tư (Đầu vào vs Đầu ra) (Đưa số liệu cho QS kiểm tra)','Nghiệm thu vật tư đầu vào/đầu ra và đưa số liệu cho QS kiểm tra.','acceptance_check','["QA/QC","QS"]'::jsonb,'acceptance.create',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000040'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,3,'[QS] Hồ sơ kiểm tra khối lượng thầu phụ (Áp giá vào)','QS lập hồ sơ kiểm tra khối lượng thầu phụ và áp giá.','document_upload','["QS"]'::jsonb,'boq.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000041'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,4,'[GĐ] Review & Check hồ sơ kiểm tra khối lượng thầu phụ','Giám đốc review/check hồ sơ khối lượng thầu phụ.','approval','["Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000042'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,5,'[QS] Hồ sơ thanh toán cho thầu phụ','QS lập hồ sơ thanh toán cho thầu phụ.','document_upload','["QS","Kế toán"]'::jsonb,'accounting.payable_view',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000043'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,6,'[GĐ] Ký nghiệm thu hồ sơ thanh toán','Giám đốc ký nghiệm thu hồ sơ thanh toán.','approval','["Giám đốc"]'::jsonb,'accounting.payable_approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000044'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,7,'[Kế toán] Kiểm tra hồ sơ thanh toán','Kế toán kiểm tra hồ sơ thanh toán.','accounting_check','["Kế toán"]'::jsonb,'accounting.payable_view',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000045'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,8,'Thanh toán cho Thầu Phụ (thanh toán đợt {n})','Thanh toán cho thầu phụ theo đợt.','accounting_check','["Kế toán"]'::jsonb,'accounting.payment_create',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000046'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,1,'[QA / QS / PM / GĐ] Nghiệm thu & ký duyệt công trình','Nghiệm thu và ký duyệt công trình.','acceptance_check','["QA/QC","QS","PM","Giám đốc"]'::jsonb,'acceptance.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000047'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,2,'[GĐ] Hồ sơ nghiệm thu & Ký duyệt','Giám đốc ký duyệt hồ sơ nghiệm thu.','approval','["Giám đốc"]'::jsonb,'acceptance.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000048'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,3,'[Kế toán / QS / PM / GĐ] Hồ sơ thanh toán & quyết toán & Ký duyệt (các bên)','Hồ sơ thanh toán, quyết toán và ký duyệt các bên.','document_upload','["Kế toán","QS","PM","Giám đốc"]'::jsonb,'accounting.receivable_view',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000049'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,4,'Xuất Hoá Đơn','Xuất hóa đơn cho chủ đầu tư.','accounting_check','["Kế toán"]'::jsonb,'accounting.receivable_approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000050'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,5,'Thanh Toán (thanh toán nhiều kỳ)','Ghi nhận thanh toán nhiều kỳ từ chủ đầu tư.','accounting_check','["Kế toán"]'::jsonb,'accounting.payment_create',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000051'::uuid,'41400000-0000-0000-0000-000000000012'::uuid,1,'Kế toán Thầu phụ Xuất hoá đơn về công ty','Ghi nhận hóa đơn thầu phụ xuất về công ty.','accounting_check','["Kế toán"]'::jsonb,'accounting.payable_view',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000052'::uuid,'41400000-0000-0000-0000-000000000012'::uuid,2,'Kế toán Xuất Hoá đơn cho Chủ đầu tư','Ghi nhận hóa đơn xuất cho chủ đầu tư và đóng tài chính.','accounting_check','["Kế toán","Giám đốc"]'::jsonb,'accounting.payment_approve',true,false,false,'builtin_generic_submission',true)
)
INSERT INTO pipeline_template_steps (
  id, node_id, name, description, step_type, required_role_ids, required_permission,
  checklist_required, document_required, photo_required, form_version_id, form_required, order_index
)
SELECT s.id, s.node_id, s.name, s.description, s.step_type, s.roles, s.permission_key,
  s.checklist_required, s.document_required, s.photo_required, fv.version_id, s.form_required, s.order_index
FROM step_seed s
LEFT JOIN form_versions fv ON fv.code=s.form_code
ON CONFLICT (id) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    step_type=EXCLUDED.step_type,
    required_role_ids=EXCLUDED.required_role_ids,
    required_permission=EXCLUDED.required_permission,
    checklist_required=EXCLUDED.checklist_required,
    document_required=EXCLUDED.document_required,
    photo_required=EXCLUDED.photo_required,
    form_version_id=EXCLUDED.form_version_id,
    form_required=EXCLUDED.form_required,
    order_index=EXCLUDED.order_index;

WITH edge_seed(id, from_node_id, to_node_id, condition_type, condition_config, label, priority) AS (
  VALUES
    ('41600000-0000-0000-0000-000000000001'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,'always','{}'::jsonb,'Hoàn tất khảo sát',100),
    ('41600000-0000-0000-0000-000000000002'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,'workflow_form_answer_equals','{"field_key":"needs_design","value":"yes"}'::jsonb,'Có thiết kế',10),
    ('41600000-0000-0000-0000-000000000003'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,'workflow_form_answer_equals','{"field_key":"needs_design","value":"no"}'::jsonb,'Không thiết kế',20),
    ('41600000-0000-0000-0000-000000000004'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,'41400000-0000-0000-0000-000000000005'::uuid,'always','{}'::jsonb,'Hoàn tất thiết kế',100),
    ('41600000-0000-0000-0000-000000000005'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,'always','{}'::jsonb,'Qua đầu vào bắt buộc',100),
    ('41600000-0000-0000-0000-000000000006'::uuid,'41400000-0000-0000-0000-000000000005'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,'always','{}'::jsonb,'Qua đầu vào bắt buộc',100),
    ('41600000-0000-0000-0000-000000000007'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,'always','{}'::jsonb,'Đầu vào đã check',100),
    ('41600000-0000-0000-0000-000000000008'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,'always','{}'::jsonb,'Chốt giá',100),
    ('41600000-0000-0000-0000-000000000009'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,'always','{}'::jsonb,'Hợp đồng đã chốt',100),
    ('41600000-0000-0000-0000-000000000010'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,'always','{}'::jsonb,'Thi công xong',100),
    ('41600000-0000-0000-0000-000000000011'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,'always','{}'::jsonb,'Nghiệm thu đầu vào xong',100),
    ('41600000-0000-0000-0000-000000000012'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,'41400000-0000-0000-0000-000000000012'::uuid,'always','{}'::jsonb,'Nghiệm thu đầu ra xong',100)
)
INSERT INTO pipeline_template_edges (id, template_id, from_node_id, to_node_id, condition_type, condition_config, label, priority)
SELECT id, '40000000-0000-0000-0000-000000000020', from_node_id, to_node_id, condition_type, condition_config, label, priority
FROM edge_seed
ON CONFLICT (id) DO UPDATE
SET from_node_id=EXCLUDED.from_node_id,
    to_node_id=EXCLUDED.to_node_id,
    condition_type=EXCLUDED.condition_type,
    condition_config=EXCLUDED.condition_config,
    label=EXCLUDED.label,
    priority=EXCLUDED.priority;
