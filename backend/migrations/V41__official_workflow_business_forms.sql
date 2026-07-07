WITH template_seed(code, name, description, form_type) AS (
  VALUES
    ('client_survey_requirement_intake', 'Khảo sát yêu cầu chủ đầu tư', 'Ghi nhận yêu cầu, phạm vi, deadline và ràng buộc ban đầu từ chủ đầu tư.', 'generic_submission'),
    ('client_design_style_brief', 'Brief phong cách thiết kế', 'Ghi nhận phong cách, vật liệu, màu sắc, tiêu chuẩn tham chiếu và điểm loại trừ.', 'generic_submission'),
    ('client_site_measurement_record', 'Biên bản đo kích thước hiện trạng', 'Ghi nhận khu vực đo, kích thước chính, phương pháp đo và minh chứng hiện trạng.', 'generic_submission'),
    ('client_site_risk_assessment', 'Đánh giá rủi ro mặt bằng', 'Checklist rủi ro hiện trạng, kỹ thuật, mặt bằng, an toàn, tiến độ và chi phí.', 'checklist'),
    ('client_site_survey_minutes', 'Biên bản khảo sát hiện trạng', 'Biên bản tổng hợp khảo sát hiện trạng, người tham dự, kết luận và hồ sơ đính kèm.', 'meeting_minutes'),
    ('client_boq_takeoff_summary', 'Bóc khối lượng sơ bộ', 'Tóm tắt phạm vi bóc khối lượng, cơ sở tính, giả định và điểm cần xác nhận.', 'generic_submission'),
    ('client_design_layout_submission', 'Nộp bản bố trí mặt bằng', 'Nộp bản layout, phiên bản, giả định thiết kế, file minh chứng và phản hồi cần chốt.', 'generic_submission'),
    ('client_design_3d_submission', 'Nộp bản vẽ 3D', 'Nộp phối cảnh 3D, phiên bản, phạm vi render, file minh chứng và trạng thái sẵn sàng.', 'generic_submission'),
    ('client_construction_drawing_submission', 'Nộp bản vẽ thi công', 'Nộp bộ bản vẽ thi công, revision, trạng thái issued-for và minh chứng tài liệu.', 'generic_submission'),
    ('client_design_review_layout', 'Rà soát bản bố trí mặt bằng', 'Ghi nhận vòng trao đổi layout, phản hồi, blocker và yêu cầu revision tiếp theo.', 'generic_submission'),
    ('client_design_review_3d', 'Rà soát bản vẽ 3D', 'Ghi nhận vòng trao đổi 3D, phản hồi, tiêu chí đạt và yêu cầu revision tiếp theo.', 'generic_submission'),
    ('client_design_review_construction_drawing', 'Rà soát bản vẽ thi công', 'Ghi nhận vòng rà soát bản vẽ thi công, revision, vấn đề tồn và file liên quan.', 'generic_submission'),
    ('client_design_owner_approval', 'Chốt thiết kế với chủ đầu tư', 'Ghi nhận quyết định chốt thiết kế với sale/chủ đầu tư, điều kiện kèm theo và minh chứng.', 'generic_submission'),
    ('client_design_to_qs_handoff', 'Bàn giao thiết kế cho QS', 'Bàn giao bộ bản vẽ, revision, giả định BOQ và xác nhận người nhận.', 'generic_submission'),
    ('client_boq_supplier_price_collection', 'Thu thập giá thầu phụ / tổ đội / vật tư', 'Ghi nhận nguồn giá, phạm vi, giá chào, điều kiện thương mại và minh chứng báo giá.', 'generic_submission'),
    ('client_boq_price_check', 'Kiểm tra đơn giá đầu vào', 'QS/PM/GĐ kiểm tra đơn giá, biên độ, chênh lệch và kết luận đầu vào.', 'generic_submission'),
    ('client_director_management_rate_review', 'GĐ rà soát định mức quản lý', 'Giám đốc rà soát định mức quản lý, điều kiện áp dụng và quyết định phê duyệt.', 'generic_submission'),
    ('client_boq_owner_submission', 'Gửi BOQ cho chủ đầu tư', 'Ghi nhận bộ BOQ gửi chủ đầu tư, phiên bản, người nhận, kênh gửi và minh chứng.', 'generic_submission'),
    ('client_procurement_takeoff_review', 'Rà soát khối lượng đầu vào', 'Rà soát khối lượng đầu vào làm cơ sở mua hàng, hợp đồng và thanh toán.', 'generic_submission'),
    ('client_procurement_sourcing', 'Mua hàng / chọn tổ đội / thầu phụ / NCC', 'Ghi nhận vendor, phạm vi mua hàng, so sánh báo giá, lead time và phương án chọn.', 'generic_submission'),
    ('client_procurement_contract_advance_package', 'Hợp đồng và hồ sơ tạm ứng đầu vào', 'Ghi nhận hợp đồng, hồ sơ tạm ứng, điều kiện thanh toán và tài liệu liên quan.', 'generic_submission'),
    ('client_procurement_input_approval', 'Rà soát và duyệt đầu vào', 'PM/GĐ rà soát phạm vi, giá, hợp đồng, tạm ứng, rủi ro và quyết định đầu vào.', 'generic_submission'),
    ('client_price_negotiation_log', 'Biên bản đàm phán giá', 'Ghi nhận quá trình đàm phán, giá trước/sau, nhượng bộ, điều kiện và kết luận.', 'generic_submission'),
    ('client_final_rate_confirmation', 'Chốt định mức sau đàm phán', 'QS/PM chốt định mức, biên độ, giả định và điều kiện áp dụng sau đàm phán.', 'generic_submission'),
    ('client_director_final_price_approval', 'GĐ chốt báo cáo đơn giá', 'Giám đốc rà soát và chốt báo cáo đơn giá cuối cùng.', 'generic_submission'),
    ('client_owner_contract_package', 'Báo giá và hợp đồng chủ đầu tư', 'Lập gói báo giá/hợp đồng với chủ đầu tư, payment terms và tài liệu đi kèm.', 'generic_submission'),
    ('client_contract_legal_finance_review', 'Kế toán / pháp lý kiểm tra hợp đồng', 'Checklist pháp lý và tài chính cho hợp đồng chủ đầu tư trước khi ký/gửi.', 'checklist'),
    ('client_owner_soft_file_submission', 'Gửi file mềm hồ sơ hợp đồng', 'Ghi nhận hồ sơ file mềm gửi chủ đầu tư, danh mục tài liệu, người nhận và kênh gửi.', 'generic_submission'),
    ('client_internal_contract_approval', 'Rà soát và ký duyệt hợp đồng nội bộ', 'PM/GĐ rà soát, ký duyệt nội bộ và điều kiện trước khi gửi bản cứng.', 'generic_submission'),
    ('client_hardcopy_dispatch', 'Gửi bản cứng hợp đồng / hồ sơ tạm ứng', 'Ghi nhận bộ hồ sơ bản cứng, phương thức gửi, người nhận và minh chứng bàn giao.', 'generic_submission'),
    ('client_construction_assignment_meeting', 'Họp phân công thi công / QA', 'Biên bản họp phân công thi công, QA/QC, trách nhiệm, deadline và action items.', 'meeting_minutes'),
    ('client_qa_construction_plan_checklist', 'Checklist QA kế hoạch thi công', 'Checklist QA cho kế hoạch thi công, tiêu chí kiểm tra, rủi ro và điều kiện bắt đầu.', 'checklist'),
    ('client_material_incoming_acceptance_summary', 'Nghiệm thu vật tư đầu vào / đầu ra', 'Tóm tắt nghiệm thu vật tư, chênh lệch đầu vào/đầu ra và minh chứng kiểm nhận.', 'generic_submission'),
    ('client_subcontractor_quantity_dossier', 'Hồ sơ kiểm tra khối lượng thầu phụ', 'QS lập hồ sơ kiểm tra khối lượng thầu phụ, áp giá và minh chứng kèm theo.', 'generic_submission'),
    ('client_director_subcontractor_quantity_approval', 'GĐ duyệt hồ sơ khối lượng thầu phụ', 'Giám đốc rà soát khối lượng, giá áp dụng, chênh lệch và quyết định duyệt.', 'generic_submission'),
    ('client_subcontractor_payment_dossier', 'Hồ sơ thanh toán thầu phụ', 'QS/Kế toán lập hồ sơ thanh toán thầu phụ, số tiền, kỳ thanh toán và chứng từ.', 'generic_submission'),
    ('client_payment_acceptance_approval', 'Ký nghiệm thu hồ sơ thanh toán', 'GĐ ký nghiệm thu hồ sơ thanh toán, điều kiện kèm theo và minh chứng chữ ký.', 'generic_submission'),
    ('client_payable_document_review', 'Kế toán kiểm tra hồ sơ thanh toán', 'Kế toán kiểm tra hồ sơ thanh toán đầu vào, chứng từ, công nợ và trạng thái xử lý.', 'generic_submission'),
    ('client_subcontractor_payment_execution', 'Thanh toán cho thầu phụ', 'Ghi nhận kế hoạch/thực hiện thanh toán thầu phụ theo đợt, số tiền và chứng từ.', 'generic_submission'),
    ('client_outgoing_acceptance_director_approval', 'GĐ ký duyệt hồ sơ nghiệm thu đầu ra', 'Giám đốc ký duyệt hồ sơ nghiệm thu đầu ra, điều kiện và tài liệu kèm theo.', 'generic_submission'),
    ('client_output_settlement_package', 'Hồ sơ thanh toán / quyết toán chủ đầu tư', 'Lập hồ sơ thanh toán, quyết toán, ký duyệt các bên và chứng từ liên quan.', 'generic_submission'),
    ('client_output_invoice_issue', 'Xuất hóa đơn cho chủ đầu tư', 'Ghi nhận hóa đơn đầu ra, số tiền, ngày phát hành, trạng thái và tài liệu PDF.', 'generic_submission'),
    ('client_output_payment_collection', 'Thu thanh toán chủ đầu tư', 'Ghi nhận kế hoạch/thu tiền nhiều kỳ từ chủ đầu tư, số tiền và trạng thái đối soát.', 'generic_submission'),
    ('client_input_invoice_closure', 'Hóa đơn thầu phụ xuất về công ty', 'Ghi nhận hóa đơn đầu vào từ thầu phụ/NCC, giá trị, trạng thái kiểm tra và PDF.', 'generic_submission'),
    ('client_output_invoice_closure', 'Hóa đơn đầu ra và đóng tài chính', 'Ghi nhận hóa đơn đầu ra cuối, trạng thái AR, cashflow, thuế và đóng tài chính.', 'generic_submission')
)
INSERT INTO workflow_form_templates (code, name, description, form_type, status)
SELECT code, name, description, form_type, 'active'
FROM template_seed
ON CONFLICT (code) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    form_type=EXCLUDED.form_type,
    status='active',
    updated_at=now();

WITH version_seed(code, version_no, variant, validation_json) AS (
  SELECT code, 1, code || '_v1', '{"source":"official_client_workflow","template_id":"40000000-0000-0000-0000-000000000020"}'::jsonb
  FROM (VALUES
    ('client_survey_requirement_intake'),
    ('client_design_style_brief'),
    ('client_site_measurement_record'),
    ('client_site_risk_assessment'),
    ('client_site_survey_minutes'),
    ('client_boq_takeoff_summary'),
    ('client_design_layout_submission'),
    ('client_design_3d_submission'),
    ('client_construction_drawing_submission'),
    ('client_design_review_layout'),
    ('client_design_review_3d'),
    ('client_design_review_construction_drawing'),
    ('client_design_owner_approval'),
    ('client_design_to_qs_handoff'),
    ('client_boq_supplier_price_collection'),
    ('client_boq_price_check'),
    ('client_director_management_rate_review'),
    ('client_boq_owner_submission'),
    ('client_procurement_takeoff_review'),
    ('client_procurement_sourcing'),
    ('client_procurement_contract_advance_package'),
    ('client_procurement_input_approval'),
    ('client_price_negotiation_log'),
    ('client_final_rate_confirmation'),
    ('client_director_final_price_approval'),
    ('client_owner_contract_package'),
    ('client_contract_legal_finance_review'),
    ('client_owner_soft_file_submission'),
    ('client_internal_contract_approval'),
    ('client_hardcopy_dispatch'),
    ('client_construction_assignment_meeting'),
    ('client_qa_construction_plan_checklist'),
    ('client_material_incoming_acceptance_summary'),
    ('client_subcontractor_quantity_dossier'),
    ('client_director_subcontractor_quantity_approval'),
    ('client_subcontractor_payment_dossier'),
    ('client_payment_acceptance_approval'),
    ('client_payable_document_review'),
    ('client_subcontractor_payment_execution'),
    ('client_outgoing_acceptance_director_approval'),
    ('client_output_settlement_package'),
    ('client_output_invoice_issue'),
    ('client_output_payment_collection'),
    ('client_input_invoice_closure'),
    ('client_output_invoice_closure')
  ) AS codes(code)
)
INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, published_at)
SELECT t.id, vs.version_no, 'published',
       jsonb_build_object('source', 'official_client_workflow', 'code', vs.code, 'variant', vs.variant),
       vs.validation_json,
       now()
FROM version_seed vs
JOIN workflow_form_templates t ON t.code=vs.code
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH decision_template AS (
  SELECT id
  FROM workflow_form_templates
  WHERE code='client_design_branch_decision'
)
INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, validation_json, published_at)
SELECT
  id,
  2,
  'published',
  '{"source":"official_client_workflow","code":"client_design_branch_decision","variant":"meeting_minutes_branch_v2"}'::jsonb,
  '{"routing_field":"needs_design","creates_work_items_from":"action_items"}'::jsonb,
  now()
FROM decision_template
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH version_rows AS (
  SELECT t.code, v.id AS version_id
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1
  WHERE t.code LIKE 'client_%'
    AND t.code <> 'client_design_branch_decision'
),
common_field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('scope_summary', 'Phạm vi / hạng mục xử lý', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, 'Ghi đúng phạm vi của bước này để người duyệt và audit hiểu bối cảnh.', 80),
    ('work_result', 'Kết quả xử lý', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":5000}'::jsonb, 'Tóm tắt kết quả thực tế sau khi xử lý bước.', 90),
    ('linked_evidence_id', 'Minh chứng liên quan', 'evidence_reference', false, '[]'::jsonb, '{}'::jsonb, 'Chọn ảnh/tài liệu đã tải lên đầu việc này nếu có.', 100),
    ('next_action', 'Việc cần làm tiếp theo', 'textarea', false, '[]'::jsonb, '{"max_length":2500}'::jsonb, NULL, 110),
    ('confirmed', 'Xác nhận hồ sơ nghiệp vụ', 'checkbox', true, '[]'::jsonb, '{}'::jsonb, 'Xác nhận dữ liệu đã đủ để gửi rà soát/duyệt.', 120)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT vr.version_id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM version_rows vr
CROSS JOIN common_field_seed fs
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;

WITH version_rows AS (
  SELECT t.code, v.id AS version_id
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1
  WHERE t.code LIKE 'client_%'
    AND t.code <> 'client_design_branch_decision'
),
field_seed(code, field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('client_survey_requirement_intake', 'owner_requirement', 'Yêu cầu chính của chủ đầu tư', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":5000}'::jsonb, NULL, 10),
    ('client_survey_requirement_intake', 'requested_areas', 'Khu vực / hạng mục cần khảo sát', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 20),
    ('client_survey_requirement_intake', 'target_deadline', 'Deadline mong muốn', 'date', false, '[]'::jsonb, '{}'::jsonb, NULL, 30),
    ('client_survey_requirement_intake', 'constraints', 'Ràng buộc đặc biệt', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, 'Ví dụ: giờ thi công, mặt bằng, ngân sách, pháp lý, vận hành hiện hữu.', 40),

    ('client_design_style_brief', 'style_direction', 'Phong cách mong muốn', 'select', true, '[{"value":"modern","label":"Hiện đại"},{"value":"minimal","label":"Tối giản"},{"value":"classic","label":"Cổ điển"},{"value":"industrial","label":"Industrial"},{"value":"luxury","label":"Cao cấp"},{"value":"other","label":"Khác"}]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_design_style_brief', 'reference_links', 'Link / mô tả mẫu tham khảo', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 20),
    ('client_design_style_brief', 'material_preferences', 'Vật liệu / màu sắc ưu tiên', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 30),
    ('client_design_style_brief', 'design_exclusions', 'Điểm không mong muốn', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 40),

    ('client_site_measurement_record', 'measurement_date', 'Ngày đo', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_site_measurement_record', 'measured_by', 'Người đo', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":120}'::jsonb, NULL, 20),
    ('client_site_measurement_record', 'dimension_items', 'Kích thước chính', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":100,"item_fields":[{"key":"area","label":"Khu vực","type":"text","required":true,"min_length":2,"max_length":160},{"key":"dimension","label":"Kích thước","type":"text","required":true,"min_length":1,"max_length":160},{"key":"unit","label":"ĐVT","type":"text","required":true,"min_length":1,"max_length":20},{"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":800}]}'::jsonb, 'Ghi từng khu vực/phòng/hạng mục cần dùng cho BOQ hoặc thiết kế.', 30),

    ('client_site_risk_assessment', 'risk_items', 'Danh sách rủi ro', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":100,"item_fields":[{"key":"risk","label":"Rủi ro","type":"textarea","required":true,"min_length":3,"max_length":1000},{"key":"category","label":"Nhóm","type":"select","required":true,"options":[{"value":"structure","label":"Kết cấu"},{"value":"mep","label":"MEP"},{"value":"access","label":"Mặt bằng"},{"value":"safety","label":"An toàn"},{"value":"schedule","label":"Tiến độ"},{"value":"cost","label":"Chi phí"},{"value":"other","label":"Khác"}]},{"key":"severity","label":"Mức độ","type":"select","required":true,"options":[{"value":"low","label":"Thấp"},{"value":"medium","label":"Trung bình"},{"value":"high","label":"Cao"},{"value":"critical","label":"Nghiêm trọng"}]},{"key":"mitigation","label":"Hướng xử lý","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 10),

    ('client_site_survey_minutes', 'survey_date', 'Ngày khảo sát', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_site_survey_minutes', 'participants', 'Thành phần tham dự', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 20),
    ('client_site_survey_minutes', 'survey_conclusion', 'Kết luận khảo sát', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":5000}'::jsonb, NULL, 30),

    ('client_boq_takeoff_summary', 'takeoff_basis', 'Cơ sở bóc khối lượng', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 10),
    ('client_boq_takeoff_summary', 'quantity_items', 'Hạng mục khối lượng chính', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":200,"item_fields":[{"key":"item","label":"Hạng mục","type":"text","required":true,"min_length":2,"max_length":180},{"key":"quantity","label":"Khối lượng","type":"number","required":false,"min":0},{"key":"unit","label":"ĐVT","type":"text","required":false,"max_length":30},{"key":"assumption","label":"Giả định","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 20),

    ('client_design_layout_submission', 'layout_version', 'Phiên bản layout', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":80}'::jsonb, NULL, 10),
    ('client_design_layout_submission', 'layout_status', 'Trạng thái layout', 'select', true, '[{"value":"draft","label":"Nháp"},{"value":"ready_for_review","label":"Sẵn sàng rà soát"},{"value":"approved","label":"Đã chốt"},{"value":"needs_revision","label":"Cần chỉnh"}]'::jsonb, '{}'::jsonb, NULL, 20),
    ('client_design_layout_submission', 'owner_feedback', 'Phản hồi cần chốt', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 30),

    ('client_design_3d_submission', 'render_version', 'Phiên bản 3D', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":80}'::jsonb, NULL, 10),
    ('client_design_3d_submission', 'render_scope', 'Phạm vi render', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 20),
    ('client_design_3d_submission', 'approval_readiness', 'Mức sẵn sàng trình duyệt', 'select', true, '[{"value":"draft","label":"Nháp"},{"value":"ready","label":"Sẵn sàng"},{"value":"approved","label":"Đã duyệt"},{"value":"needs_revision","label":"Cần chỉnh"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_construction_drawing_submission', 'drawing_set', 'Bộ bản vẽ', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 10),
    ('client_construction_drawing_submission', 'revision_no', 'Revision', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":80}'::jsonb, NULL, 20),
    ('client_construction_drawing_submission', 'issued_for', 'Mục đích phát hành', 'select', true, '[{"value":"review","label":"Review"},{"value":"boq","label":"Làm BOQ"},{"value":"construction","label":"Thi công"},{"value":"as_built","label":"As-built"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_design_review_layout', 'review_round', 'Vòng review layout', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":80}'::jsonb, NULL, 10),
    ('client_design_review_layout', 'review_comments', 'Ý kiến review', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":5000}'::jsonb, NULL, 20),
    ('client_design_review_layout', 'revision_required', 'Cần chỉnh revision', 'select', true, '[{"value":"yes","label":"Có"},{"value":"no","label":"Không"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_design_review_3d', 'review_round', 'Vòng review 3D', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":80}'::jsonb, NULL, 10),
    ('client_design_review_3d', 'acceptance_criteria', 'Tiêu chí cần đạt', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 20),
    ('client_design_review_3d', 'revision_required', 'Cần chỉnh revision', 'select', true, '[{"value":"yes","label":"Có"},{"value":"no","label":"Không"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_design_review_construction_drawing', 'drawing_review_scope', 'Phạm vi review bản vẽ thi công', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 10),
    ('client_design_review_construction_drawing', 'open_issues', 'Vấn đề còn mở', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 20),
    ('client_design_review_construction_drawing', 'ready_for_qs', 'Sẵn sàng chuyển QS', 'select', true, '[{"value":"yes","label":"Có"},{"value":"no","label":"Chưa"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_design_owner_approval', 'decision', 'Quyết định của chủ đầu tư', 'select', true, '[{"value":"approved","label":"Chốt"},{"value":"conditional","label":"Chốt có điều kiện"},{"value":"revise","label":"Yêu cầu chỉnh"},{"value":"rejected","label":"Không duyệt"}]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_design_owner_approval', 'decision_by', 'Người chốt', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":160}'::jsonb, NULL, 20),
    ('client_design_owner_approval', 'decision_conditions', 'Điều kiện / ghi chú chốt', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 30),

    ('client_design_to_qs_handoff', 'handoff_package', 'Bộ hồ sơ bàn giao', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":4000}'::jsonb, NULL, 10),
    ('client_design_to_qs_handoff', 'receiver', 'Người nhận QS', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":120}'::jsonb, NULL, 20),
    ('client_design_to_qs_handoff', 'boq_assumptions', 'Giả định BOQ cần lưu ý', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 30),

    ('client_boq_supplier_price_collection', 'quote_items', 'Báo giá thu thập', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":200,"item_fields":[{"key":"partner","label":"Thầu phụ/NCC","type":"text","required":true,"min_length":2,"max_length":160},{"key":"scope","label":"Phạm vi","type":"text","required":true,"min_length":2,"max_length":180},{"key":"amount","label":"Giá chào","type":"number","required":false,"min":0},{"key":"terms","label":"Điều kiện","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 10),

    ('client_boq_price_check', 'price_variance_note', 'Chênh lệch / điểm cần kiểm tra', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":4000}'::jsonb, NULL, 10),
    ('client_boq_price_check', 'margin_status', 'Biên độ sau kiểm tra', 'select', true, '[{"value":"ok","label":"Đạt"},{"value":"watch","label":"Cần theo dõi"},{"value":"risk","label":"Rủi ro"},{"value":"reject","label":"Không đạt"}]'::jsonb, '{}'::jsonb, NULL, 20),
    ('client_boq_price_check', 'review_decision', 'Kết luận kiểm tra', 'select', true, '[{"value":"approved","label":"Duyệt"},{"value":"revise","label":"Cần chỉnh"},{"value":"escalate","label":"Trình GĐ"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_director_management_rate_review', 'management_rate', 'Định mức quản lý', 'number', false, '[]'::jsonb, '{"min":0}'::jsonb, NULL, 10),
    ('client_director_management_rate_review', 'director_decision', 'Quyết định GĐ', 'select', true, '[{"value":"approved","label":"Duyệt"},{"value":"conditional","label":"Duyệt có điều kiện"},{"value":"revise","label":"Yêu cầu chỉnh"}]'::jsonb, '{}'::jsonb, NULL, 20),

    ('client_boq_owner_submission', 'boq_version', 'Phiên bản BOQ', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":80}'::jsonb, NULL, 10),
    ('client_boq_owner_submission', 'sent_to', 'Người nhận phía chủ đầu tư', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":160}'::jsonb, NULL, 20),
    ('client_boq_owner_submission', 'sent_channel', 'Kênh gửi', 'select', true, '[{"value":"email","label":"Email"},{"value":"zalo","label":"Zalo"},{"value":"hardcopy","label":"Bản cứng"},{"value":"meeting","label":"Trong cuộc họp"},{"value":"other","label":"Khác"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_procurement_takeoff_review', 'takeoff_source', 'Nguồn khối lượng đầu vào', 'select', true, '[{"value":"boq_no_design","label":"BOQ không thiết kế"},{"value":"boq_with_design","label":"BOQ có thiết kế"},{"value":"site_measurement","label":"Đo hiện trạng"},{"value":"supplier_recheck","label":"Thầu phụ/NCC kiểm tra lại"},{"value":"other","label":"Khác"}]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_procurement_takeoff_review', 'input_quantity_items', 'Khối lượng đầu vào cần mua/khoán', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":200,"item_fields":[{"key":"item","label":"Hạng mục/vật tư","type":"text","required":true,"min_length":2,"max_length":180},{"key":"quantity","label":"Khối lượng","type":"number","required":false,"min":0},{"key":"unit","label":"ĐVT","type":"text","required":false,"max_length":30},{"key":"source_note","label":"Nguồn/ghi chú","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 20),
    ('client_procurement_takeoff_review', 'procurement_readiness', 'Sẵn sàng chuyển mua hàng', 'select', true, '[{"value":"ready","label":"Sẵn sàng"},{"value":"needs_check","label":"Cần kiểm tra thêm"},{"value":"blocked","label":"Đang vướng"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_procurement_sourcing', 'sourcing_items', 'Phương án mua hàng / chọn thầu phụ', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":100,"item_fields":[{"key":"partner","label":"Tổ đội/Thầu phụ/NCC","type":"text","required":true,"min_length":2,"max_length":160},{"key":"scope","label":"Phạm vi","type":"text","required":true,"min_length":2,"max_length":180},{"key":"quoted_amount","label":"Giá chào","type":"number","required":false,"min":0},{"key":"lead_time","label":"Lead time","type":"text","required":false,"max_length":120},{"key":"selected","label":"Chọn","type":"select","required":true,"options":[{"value":"yes","label":"Chọn"},{"value":"backup","label":"Dự phòng"},{"value":"no","label":"Không chọn"}]}]}'::jsonb, NULL, 10),

    ('client_procurement_contract_advance_package', 'contract_partner', 'Đối tác hợp đồng', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":160}'::jsonb, NULL, 10),
    ('client_procurement_contract_advance_package', 'advance_amount', 'Số tiền tạm ứng', 'number', false, '[]'::jsonb, '{"min":0}'::jsonb, NULL, 20),
    ('client_procurement_contract_advance_package', 'payment_terms', 'Điều kiện thanh toán', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 30),

    ('client_procurement_input_approval', 'approval_decision', 'Quyết định đầu vào', 'select', true, '[{"value":"approved","label":"Duyệt"},{"value":"conditional","label":"Duyệt có điều kiện"},{"value":"revise","label":"Cần chỉnh"},{"value":"reject","label":"Không duyệt"}]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_procurement_input_approval', 'risk_note', 'Rủi ro đầu vào', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 20),

    ('client_price_negotiation_log', 'negotiation_items', 'Nội dung đàm phán', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":100,"item_fields":[{"key":"party","label":"Bên đàm phán","type":"text","required":true,"min_length":2,"max_length":160},{"key":"before_amount","label":"Giá trước","type":"number","required":false,"min":0},{"key":"after_amount","label":"Giá sau","type":"number","required":false,"min":0},{"key":"concession","label":"Nhượng bộ/điều kiện","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 10),

    ('client_final_rate_confirmation', 'final_rate_basis', 'Cơ sở chốt định mức', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":4000}'::jsonb, NULL, 10),
    ('client_final_rate_confirmation', 'margin_note', 'Ghi chú biên độ', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 20),

    ('client_director_final_price_approval', 'director_decision', 'Quyết định GĐ', 'select', true, '[{"value":"approved","label":"Chốt"},{"value":"conditional","label":"Chốt có điều kiện"},{"value":"revise","label":"Yêu cầu chỉnh"}]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_director_final_price_approval', 'approval_conditions', 'Điều kiện chốt giá', 'textarea', false, '[]'::jsonb, '{"max_length":3000}'::jsonb, NULL, 20),

    ('client_owner_contract_package', 'quote_version', 'Phiên bản báo giá', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":80}'::jsonb, NULL, 10),
    ('client_owner_contract_package', 'contract_value', 'Giá trị hợp đồng dự kiến', 'number', false, '[]'::jsonb, '{"min":0}'::jsonb, NULL, 20),
    ('client_owner_contract_package', 'payment_terms', 'Điều khoản thanh toán', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 30),

    ('client_contract_legal_finance_review', 'review_items', 'Checklist pháp lý / tài chính', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":80,"item_fields":[{"key":"item","label":"Nội dung kiểm tra","type":"text","required":true,"min_length":2,"max_length":180},{"key":"status","label":"Trạng thái","type":"select","required":true,"options":[{"value":"done","label":"Đạt"},{"value":"not_applicable","label":"N/A"},{"value":"needs_follow_up","label":"Cần xử lý"},{"value":"blocked","label":"Blocked"}]},{"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 10),

    ('client_owner_soft_file_submission', 'document_list', 'Danh mục file đã gửi', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":4000}'::jsonb, NULL, 10),
    ('client_owner_soft_file_submission', 'recipient', 'Người nhận', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":160}'::jsonb, NULL, 20),
    ('client_owner_soft_file_submission', 'delivery_channel', 'Kênh gửi', 'select', true, '[{"value":"email","label":"Email"},{"value":"zalo","label":"Zalo"},{"value":"drive","label":"Drive/Link"},{"value":"other","label":"Khác"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_internal_contract_approval', 'internal_decision', 'Quyết định ký duyệt', 'select', true, '[{"value":"approved","label":"Duyệt ký"},{"value":"conditional","label":"Duyệt có điều kiện"},{"value":"revise","label":"Yêu cầu chỉnh"}]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_internal_contract_approval', 'signer', 'Người ký duyệt', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":160}'::jsonb, NULL, 20),

    ('client_hardcopy_dispatch', 'hardcopy_set', 'Bộ hồ sơ bản cứng', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":4000}'::jsonb, NULL, 10),
    ('client_hardcopy_dispatch', 'receiver', 'Người nhận', 'text', true, '[]'::jsonb, '{"min_length":2,"max_length":160}'::jsonb, NULL, 20),
    ('client_hardcopy_dispatch', 'delivery_method', 'Cách gửi', 'select', true, '[{"value":"hand_delivery","label":"Giao trực tiếp"},{"value":"courier","label":"Chuyển phát"},{"value":"client_pickup","label":"CĐT nhận tại văn phòng"},{"value":"other","label":"Khác"}]'::jsonb, '{}'::jsonb, NULL, 30),

    ('client_construction_assignment_meeting', 'meeting_date', 'Ngày họp', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_construction_assignment_meeting', 'attendees', 'Thành phần tham dự', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 20),
    ('client_construction_assignment_meeting', 'assignment_items', 'Phân công thi công / QA', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":120,"item_fields":[{"key":"task","label":"Việc phân công","type":"text","required":true,"min_length":3,"max_length":180},{"key":"owner_role","label":"Bộ phận phụ trách","type":"text","required":true,"min_length":2,"max_length":80},{"key":"due_date","label":"Hạn","type":"date","required":false},{"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 30),

    ('client_qa_construction_plan_checklist', 'qa_items', 'Checklist QA kế hoạch thi công', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":120,"item_fields":[{"key":"item","label":"Tiêu chí","type":"text","required":true,"min_length":3,"max_length":180},{"key":"status","label":"Trạng thái","type":"select","required":true,"options":[{"value":"done","label":"Đạt"},{"value":"not_applicable","label":"N/A"},{"value":"needs_follow_up","label":"Cần xử lý"},{"value":"blocked","label":"Blocked"}]},{"key":"evidence_id","label":"Minh chứng","type":"evidence_reference","required":false},{"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 10),

    ('client_material_incoming_acceptance_summary', 'material_items', 'Vật tư nghiệm thu', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":200,"item_fields":[{"key":"material","label":"Vật tư","type":"text","required":true,"min_length":2,"max_length":160},{"key":"delivered_quantity","label":"KL giao","type":"number","required":false,"min":0},{"key":"accepted_quantity","label":"KL nhận","type":"number","required":false,"min":0},{"key":"unit","label":"ĐVT","type":"text","required":false,"max_length":30},{"key":"decision","label":"Quyết định","type":"select","required":true,"options":[{"value":"accepted","label":"Nhận"},{"value":"conditional","label":"Nhận có điều kiện"},{"value":"rejected","label":"Từ chối"}]}]}'::jsonb, NULL, 10),

    ('client_subcontractor_quantity_dossier', 'quantity_items', 'Khối lượng thầu phụ', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":200,"item_fields":[{"key":"item","label":"Hạng mục","type":"text","required":true,"min_length":2,"max_length":180},{"key":"claimed_quantity","label":"KL claim","type":"number","required":false,"min":0},{"key":"approved_quantity","label":"KL duyệt","type":"number","required":false,"min":0},{"key":"unit_price","label":"Đơn giá","type":"number","required":false,"min":0},{"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 10),

    ('client_director_subcontractor_quantity_approval', 'director_decision', 'Quyết định GĐ', 'select', true, '[{"value":"approved","label":"Duyệt"},{"value":"conditional","label":"Duyệt có điều kiện"},{"value":"revise","label":"Cần chỉnh"},{"value":"reject","label":"Không duyệt"}]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_payment_acceptance_approval', 'director_decision', 'Quyết định ký nghiệm thu thanh toán', 'select', true, '[{"value":"approved","label":"Ký duyệt"},{"value":"conditional","label":"Ký có điều kiện"},{"value":"revise","label":"Cần bổ sung"}]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_outgoing_acceptance_director_approval', 'director_decision', 'Quyết định ký nghiệm thu đầu ra', 'select', true, '[{"value":"approved","label":"Ký duyệt"},{"value":"conditional","label":"Ký có điều kiện"},{"value":"revise","label":"Cần bổ sung"}]'::jsonb, '{}'::jsonb, NULL, 10),

    ('client_subcontractor_payment_dossier', 'payment_period', 'Kỳ thanh toán', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":120}'::jsonb, NULL, 10),
    ('client_subcontractor_payment_dossier', 'payment_amount', 'Số tiền đề nghị', 'number', false, '[]'::jsonb, '{"min":0}'::jsonb, NULL, 20),
    ('client_payable_document_review', 'review_status', 'Trạng thái kiểm tra hồ sơ', 'select', true, '[{"value":"complete","label":"Đủ hồ sơ"},{"value":"missing","label":"Thiếu hồ sơ"},{"value":"mismatch","label":"Lệch số liệu"},{"value":"approved","label":"Đã duyệt"}]'::jsonb, '{}'::jsonb, NULL, 10),
    ('client_subcontractor_payment_execution', 'payment_period', 'Đợt thanh toán', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":120}'::jsonb, NULL, 10),
    ('client_subcontractor_payment_execution', 'paid_amount', 'Số tiền thanh toán', 'number', false, '[]'::jsonb, '{"min":0}'::jsonb, NULL, 20),

    ('client_output_settlement_package', 'settlement_amount', 'Giá trị thanh toán / quyết toán', 'number', false, '[]'::jsonb, '{"min":0}'::jsonb, NULL, 10),
    ('client_output_settlement_package', 'signing_parties', 'Các bên ký duyệt', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 20),
    ('client_output_invoice_issue', 'invoice_no', 'Số hóa đơn', 'text', false, '[]'::jsonb, '{"max_length":120}'::jsonb, NULL, 10),
    ('client_output_invoice_issue', 'invoice_amount', 'Giá trị hóa đơn', 'number', false, '[]'::jsonb, '{"min":0}'::jsonb, NULL, 20),
    ('client_output_payment_collection', 'collection_period', 'Kỳ thu tiền', 'text', true, '[]'::jsonb, '{"min_length":1,"max_length":120}'::jsonb, NULL, 10),
    ('client_output_payment_collection', 'received_amount', 'Số tiền đã thu', 'number', false, '[]'::jsonb, '{"min":0}'::jsonb, NULL, 20),
    ('client_input_invoice_closure', 'input_invoice_no', 'Số hóa đơn đầu vào', 'text', false, '[]'::jsonb, '{"max_length":120}'::jsonb, NULL, 10),
    ('client_input_invoice_closure', 'invoice_amount', 'Giá trị hóa đơn', 'number', false, '[]'::jsonb, '{"min":0}'::jsonb, NULL, 20),
    ('client_output_invoice_closure', 'output_invoice_no', 'Số hóa đơn đầu ra', 'text', false, '[]'::jsonb, '{"max_length":120}'::jsonb, NULL, 10),
    ('client_output_invoice_closure', 'closure_checks', 'Kiểm tra đóng tài chính', 'structured_list', true, '[]'::jsonb, '{"min_items":1,"max_items":80,"item_fields":[{"key":"item","label":"Nội dung","type":"text","required":true,"min_length":2,"max_length":180},{"key":"status","label":"Trạng thái","type":"select","required":true,"options":[{"value":"done","label":"Hoàn tất"},{"value":"not_applicable","label":"N/A"},{"value":"pending","label":"Đang chờ"},{"value":"blocked","label":"Blocked"}]},{"key":"note","label":"Ghi chú","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 20)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT vr.version_id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM field_seed fs
JOIN version_rows vr ON vr.code=fs.code
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;

WITH decision_version AS (
  SELECT v.id AS version_id
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id
  WHERE t.code='client_design_branch_decision' AND v.version_no=2
),
field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('meeting_title', 'Tiêu đề cuộc họp', 'text', true, '[]'::jsonb, '{"min_length":3,"max_length":160}'::jsonb, NULL, 10),
    ('meeting_date', 'Ngày họp', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 20),
    ('attendees', 'Thành phần tham dự', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":3000}'::jsonb, NULL, 30),
    ('agenda', 'Nội dung chính', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":5000}'::jsonb, NULL, 40),
    ('decisions', 'Quyết định cuộc họp', 'textarea', true, '[]'::jsonb, '{"min_length":3,"max_length":5000}'::jsonb, NULL, 50),
    ('needs_design', 'Nhánh xử lý sau khảo sát', 'select', true, '[{"value":"yes","label":"Có thiết kế"},{"value":"no","label":"Không thiết kế"}]'::jsonb, '{}'::jsonb, 'Trường này quyết định workflow đi nhánh Thiết kế hoặc BOQ không thiết kế.', 60),
    ('action_items', 'Việc cần làm sau họp', 'structured_list', false, '[]'::jsonb, '{"max_items":120,"item_fields":[{"key":"title","label":"Việc cần làm","type":"text","required":true,"min_length":3,"max_length":180},{"key":"owner_role","label":"Bộ phận phụ trách","type":"text","required":true,"min_length":2,"max_length":80},{"key":"due_date","label":"Hạn","type":"date","required":false},{"key":"priority","label":"Priority","type":"select","required":false,"options":[{"value":"low","label":"Low"},{"value":"normal","label":"Normal"},{"value":"high","label":"High"},{"value":"urgent","label":"Urgent"}]},{"key":"description","label":"Ghi chú","type":"textarea","required":false,"max_length":1000}]}'::jsonb, NULL, 70),
    ('confirmed', 'Xác nhận biên bản họp', 'checkbox', true, '[]'::jsonb, '{}'::jsonb, NULL, 80)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT dv.version_id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM decision_version dv
CROSS JOIN field_seed fs
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;

WITH form_versions AS (
  SELECT t.code, v.id AS version_id, v.version_no
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id AND v.status='published'
  WHERE t.code IN (
    'client_survey_requirement_intake',
    'client_design_style_brief',
    'client_site_measurement_record',
    'client_site_risk_assessment',
    'client_site_survey_minutes',
    'client_boq_takeoff_summary',
    'client_design_layout_submission',
    'client_design_3d_submission',
    'client_construction_drawing_submission',
    'client_design_branch_decision',
    'client_design_review_layout',
    'client_design_review_3d',
    'client_design_review_construction_drawing',
    'client_design_owner_approval',
    'client_design_to_qs_handoff',
    'client_boq_supplier_price_collection',
    'client_boq_price_check',
    'client_director_management_rate_review',
    'client_boq_owner_submission',
    'client_procurement_takeoff_review',
    'client_procurement_sourcing',
    'client_procurement_contract_advance_package',
    'client_procurement_input_approval',
    'client_price_negotiation_log',
    'client_final_rate_confirmation',
    'client_director_final_price_approval',
    'client_owner_contract_package',
    'client_contract_legal_finance_review',
    'client_owner_soft_file_submission',
    'client_internal_contract_approval',
    'client_hardcopy_dispatch',
    'client_construction_assignment_meeting',
    'client_qa_construction_plan_checklist',
    'client_material_incoming_acceptance_summary',
    'client_subcontractor_quantity_dossier',
    'client_director_subcontractor_quantity_approval',
    'client_subcontractor_payment_dossier',
    'client_payment_acceptance_approval',
    'client_payable_document_review',
    'client_subcontractor_payment_execution',
    'client_outgoing_acceptance_director_approval',
    'client_output_settlement_package',
    'client_output_invoice_issue',
    'client_output_payment_collection',
    'client_input_invoice_closure',
    'client_output_invoice_closure',
    'builtin_meeting_minutes',
    'builtin_construction_daily_report',
    'builtin_subcontractor_checklist',
    'builtin_owner_client_checklist'
  )
),
step_form_map(step_id, form_code, version_no, form_required) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000001'::uuid, 'client_survey_requirement_intake', 1, true),
    ('41500000-0000-0000-0000-000000000003'::uuid, 'client_design_style_brief', 1, true),
    ('41500000-0000-0000-0000-000000000004'::uuid, 'client_site_measurement_record', 1, true),
    ('41500000-0000-0000-0000-000000000005'::uuid, 'client_site_risk_assessment', 1, true),
    ('41500000-0000-0000-0000-000000000006'::uuid, 'client_site_survey_minutes', 1, true),
    ('41500000-0000-0000-0000-000000000007'::uuid, 'client_boq_takeoff_summary', 1, true),
    ('41500000-0000-0000-0000-000000000008'::uuid, 'client_design_layout_submission', 1, true),
    ('41500000-0000-0000-0000-000000000009'::uuid, 'client_design_3d_submission', 1, true),
    ('41500000-0000-0000-0000-000000000010'::uuid, 'client_construction_drawing_submission', 1, true),
    ('41500000-0000-0000-0000-000000000011'::uuid, 'client_design_branch_decision', 2, true),
    ('41500000-0000-0000-0000-000000000012'::uuid, 'client_design_review_layout', 1, true),
    ('41500000-0000-0000-0000-000000000013'::uuid, 'client_design_review_3d', 1, true),
    ('41500000-0000-0000-0000-000000000014'::uuid, 'client_design_review_construction_drawing', 1, true),
    ('41500000-0000-0000-0000-000000000015'::uuid, 'client_design_owner_approval', 1, true),
    ('41500000-0000-0000-0000-000000000016'::uuid, 'client_design_to_qs_handoff', 1, true),
    ('41500000-0000-0000-0000-000000000017'::uuid, 'client_boq_supplier_price_collection', 1, true),
    ('41500000-0000-0000-0000-000000000018'::uuid, 'client_boq_price_check', 1, true),
    ('41500000-0000-0000-0000-000000000019'::uuid, 'client_director_management_rate_review', 1, true),
    ('41500000-0000-0000-0000-000000000020'::uuid, 'client_boq_owner_submission', 1, true),
    ('41500000-0000-0000-0000-000000000021'::uuid, 'client_boq_owner_submission', 1, true),
    ('41500000-0000-0000-0000-000000000022'::uuid, 'client_procurement_takeoff_review', 1, true),
    ('41500000-0000-0000-0000-000000000023'::uuid, 'client_procurement_sourcing', 1, true),
    ('41500000-0000-0000-0000-000000000024'::uuid, 'client_procurement_contract_advance_package', 1, true),
    ('41500000-0000-0000-0000-000000000025'::uuid, 'client_procurement_input_approval', 1, true),
    ('41500000-0000-0000-0000-000000000026'::uuid, 'client_price_negotiation_log', 1, true),
    ('41500000-0000-0000-0000-000000000027'::uuid, 'client_final_rate_confirmation', 1, true),
    ('41500000-0000-0000-0000-000000000028'::uuid, 'client_director_final_price_approval', 1, true),
    ('41500000-0000-0000-0000-000000000029'::uuid, 'client_owner_contract_package', 1, true),
    ('41500000-0000-0000-0000-000000000030'::uuid, 'client_contract_legal_finance_review', 1, true),
    ('41500000-0000-0000-0000-000000000031'::uuid, 'client_owner_soft_file_submission', 1, true),
    ('41500000-0000-0000-0000-000000000032'::uuid, 'client_internal_contract_approval', 1, true),
    ('41500000-0000-0000-0000-000000000033'::uuid, 'client_hardcopy_dispatch', 1, true),
    ('41500000-0000-0000-0000-000000000034'::uuid, 'client_construction_assignment_meeting', 1, true),
    ('41500000-0000-0000-0000-000000000035'::uuid, 'builtin_meeting_minutes', 2, true),
    ('41500000-0000-0000-0000-000000000036'::uuid, 'client_qa_construction_plan_checklist', 1, true),
    ('41500000-0000-0000-0000-000000000037'::uuid, 'builtin_construction_daily_report', 1, true),
    ('41500000-0000-0000-0000-000000000038'::uuid, 'builtin_subcontractor_checklist', 1, true),
    ('41500000-0000-0000-0000-000000000039'::uuid, 'client_material_incoming_acceptance_summary', 1, true),
    ('41500000-0000-0000-0000-000000000040'::uuid, 'client_subcontractor_quantity_dossier', 1, true),
    ('41500000-0000-0000-0000-000000000041'::uuid, 'client_director_subcontractor_quantity_approval', 1, true),
    ('41500000-0000-0000-0000-000000000042'::uuid, 'client_subcontractor_payment_dossier', 1, true),
    ('41500000-0000-0000-0000-000000000043'::uuid, 'client_payment_acceptance_approval', 1, true),
    ('41500000-0000-0000-0000-000000000044'::uuid, 'client_payable_document_review', 1, true),
    ('41500000-0000-0000-0000-000000000045'::uuid, 'client_subcontractor_payment_execution', 1, true),
    ('41500000-0000-0000-0000-000000000046'::uuid, 'builtin_owner_client_checklist', 1, true),
    ('41500000-0000-0000-0000-000000000047'::uuid, 'client_outgoing_acceptance_director_approval', 1, true),
    ('41500000-0000-0000-0000-000000000048'::uuid, 'client_output_settlement_package', 1, true),
    ('41500000-0000-0000-0000-000000000049'::uuid, 'client_output_invoice_issue', 1, true),
    ('41500000-0000-0000-0000-000000000050'::uuid, 'client_output_payment_collection', 1, true),
    ('41500000-0000-0000-0000-000000000051'::uuid, 'client_input_invoice_closure', 1, true),
    ('41500000-0000-0000-0000-000000000052'::uuid, 'client_output_invoice_closure', 1, true)
)
UPDATE pipeline_template_steps step
SET form_version_id=fv.version_id,
    form_required=map.form_required
FROM step_form_map map
JOIN form_versions fv ON fv.code=map.form_code AND fv.version_no=map.version_no,
     pipeline_template_nodes node
WHERE step.id=map.step_id
  AND node.id=step.node_id
  AND node.template_id='40000000-0000-0000-0000-000000000020';

UPDATE pipeline_template_steps step
SET form_version_id=NULL,
    form_required=false
FROM pipeline_template_nodes node
WHERE step.node_id=node.id
  AND node.template_id='40000000-0000-0000-0000-000000000020'
  AND step.id='41500000-0000-0000-0000-000000000002';
