UPDATE workflow_form_templates
SET description='Biểu mẫu chốt quy trình đi nhánh Có thiết kế hoặc Không thiết kế sau họp báo cáo khảo sát.',
    updated_at=now()
WHERE code='client_design_branch_decision';

UPDATE workflow_form_fields field
SET help_text='Chọn đúng nhánh để quy trình chuyển tiếp.'
FROM workflow_form_versions version
JOIN workflow_form_templates template ON template.id=version.template_id
WHERE field.form_version_id=version.id
  AND template.code='client_design_branch_decision'
  AND field.field_key='needs_design';

UPDATE pipeline_templates
SET name='[Client] Quy trình hoàn thiện công trình chính thức',
    description='Quy trình chính thức từ vận hành thực tế của client: khảo sát, nhánh thiết kế/không thiết kế, BOQ, đầu vào bắt buộc, thi công, nghiệm thu, bàn giao và đóng tài chính.',
    updated_at=now()
WHERE id='40000000-0000-0000-0000-000000000020';

WITH node_updates(id, description, exit_conditions, approval_rule) AS (
  VALUES
    (
      '41400000-0000-0000-0000-000000000006'::uuid,
      'Bước bắt buộc để chốt khối lượng, mua hàng, hợp đồng/tạm ứng và rà soát đầu vào.',
      '{"mandatory_gate":true,"text":"Đầu vào tổ đội/thầu phụ/nhà cung cấp vật tư đã được rà soát và kiểm tra"}'::jsonb,
      'PM/GĐ rà soát đầu vào'
    ),
    (
      '41400000-0000-0000-0000-000000000009'::uuid,
      'Họp phân công, chốt biên bản, đảm bảo chất lượng, kế hoạch thi công và báo cáo liên tục.',
      '{"text":"QA/QC và báo cáo thi công đã hoàn tất"}'::jsonb,
      'PM/QA rà soát thi công'
    )
)
UPDATE pipeline_template_nodes node
SET description=updates.description,
    exit_conditions=updates.exit_conditions,
    approval_rule=updates.approval_rule,
    updated_at=now()
FROM node_updates updates
WHERE node.id=updates.id
  AND node.template_id='40000000-0000-0000-0000-000000000020';

WITH node_updates(id, description, exit_conditions, approval_rule) AS (
  VALUES
    (
      '41400000-0000-0000-0000-000000000006'::uuid,
      'Bước bắt buộc để chốt khối lượng, mua hàng, hợp đồng/tạm ứng và rà soát đầu vào.',
      '{"mandatory_gate":true,"text":"Đầu vào tổ đội/thầu phụ/nhà cung cấp vật tư đã được rà soát và kiểm tra"}'::jsonb,
      'PM/GĐ rà soát đầu vào'
    ),
    (
      '41400000-0000-0000-0000-000000000009'::uuid,
      'Họp phân công, chốt biên bản, đảm bảo chất lượng, kế hoạch thi công và báo cáo liên tục.',
      '{"text":"QA/QC và báo cáo thi công đã hoàn tất"}'::jsonb,
      'PM/QA rà soát thi công'
    )
)
UPDATE pipeline_run_nodes run_node
SET description=updates.description,
    exit_conditions=updates.exit_conditions,
    approval_rule=updates.approval_rule
FROM node_updates updates, pipeline_runs run
WHERE run_node.template_node_id=updates.id
  AND run_node.run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000020';

WITH step_updates(id, name, description) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000001'::uuid, 'Khảo sát yêu cầu của chủ đầu tư (Chính: PM, GĐ. Phối hợp: QS, Sale, Đội kỹ thuật)', 'Ghi nhận yêu cầu chính của chủ đầu tư và phạm vi khảo sát.'),
    ('41500000-0000-0000-0000-000000000002'::uuid, 'Khảo sát / Chụp / Tải ảnh hiện trạng', 'Chụp và tải ảnh hiện trạng.'),
    ('41500000-0000-0000-0000-000000000012'::uuid, 'Trao đổi / Lên bản vẽ bố trí mặt bằng', 'Trao đổi và lên bản vẽ bố trí mặt bằng.'),
    ('41500000-0000-0000-0000-000000000013'::uuid, 'Trao đổi / Lên bản vẽ 3D', 'Trao đổi và lên bản vẽ 3D.'),
    ('41500000-0000-0000-0000-000000000014'::uuid, 'Trao đổi / Lên bản vẽ thi công', 'Trao đổi và tải bản vẽ thi công.'),
    ('41500000-0000-0000-0000-000000000019'::uuid, 'GĐ Rà soát định mức quản lý', 'Giám đốc rà soát định mức quản lý.'),
    ('41500000-0000-0000-0000-000000000023'::uuid, 'Mua hàng / QS Mua hàng với Tổ Đội / Thầu Phụ / Cung cấp vật tư', 'Làm việc mua hàng với tổ đội, thầu phụ, nhà cung cấp vật tư.'),
    ('41500000-0000-0000-0000-000000000024'::uuid, 'Mua hàng / QS Ký hợp đồng / Hồ sơ tạm ứng với Tổ Đội / Thầu Phụ / Nhà cung cấp vật tư', 'Ký hợp đồng và chuẩn bị hồ sơ tạm ứng đầu vào.'),
    ('41500000-0000-0000-0000-000000000025'::uuid, 'PM / GĐ Rà soát & Kiểm tra Đầu Vào', 'PM/GĐ rà soát và kiểm tra đầu vào.'),
    ('41500000-0000-0000-0000-000000000028'::uuid, 'GĐ Rà soát & Chốt báo cáo đơn giá', 'Giám đốc rà soát và chốt báo cáo đơn giá.'),
    ('41500000-0000-0000-0000-000000000030'::uuid, 'Kế toán / Pháp lý kiểm tra lại hợp đồng', 'Kế toán/pháp lý kiểm tra hợp đồng.'),
    ('41500000-0000-0000-0000-000000000032'::uuid, 'PM / GĐ rà soát và ký duyệt', 'PM/GĐ rà soát và ký duyệt.'),
    ('41500000-0000-0000-0000-000000000041'::uuid, '[GĐ] Rà soát & Kiểm tra hồ sơ kiểm tra khối lượng thầu phụ', 'Giám đốc rà soát/kiểm tra hồ sơ khối lượng thầu phụ.'),
    ('41500000-0000-0000-0000-000000000049'::uuid, 'Xuất hóa đơn', 'Xuất hóa đơn cho chủ đầu tư.'),
    ('41500000-0000-0000-0000-000000000050'::uuid, 'Thanh toán (thanh toán nhiều kỳ)', 'Ghi nhận thanh toán nhiều kỳ từ chủ đầu tư.'),
    ('41500000-0000-0000-0000-000000000051'::uuid, 'Kế toán thầu phụ xuất hóa đơn về công ty', 'Ghi nhận hóa đơn thầu phụ xuất về công ty.'),
    ('41500000-0000-0000-0000-000000000052'::uuid, 'Kế toán xuất hóa đơn cho chủ đầu tư', 'Ghi nhận hóa đơn xuất cho chủ đầu tư và đóng tài chính.')
)
UPDATE pipeline_template_steps step
SET name=updates.name,
    description=updates.description
FROM step_updates updates, pipeline_template_nodes node
WHERE step.id=updates.id
  AND node.id=step.node_id
  AND node.template_id='40000000-0000-0000-0000-000000000020';

WITH step_updates(id, name, description) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000001'::uuid, 'Khảo sát yêu cầu của chủ đầu tư (Chính: PM, GĐ. Phối hợp: QS, Sale, Đội kỹ thuật)', 'Ghi nhận yêu cầu chính của chủ đầu tư và phạm vi khảo sát.'),
    ('41500000-0000-0000-0000-000000000002'::uuid, 'Khảo sát / Chụp / Tải ảnh hiện trạng', 'Chụp và tải ảnh hiện trạng.'),
    ('41500000-0000-0000-0000-000000000012'::uuid, 'Trao đổi / Lên bản vẽ bố trí mặt bằng', 'Trao đổi và lên bản vẽ bố trí mặt bằng.'),
    ('41500000-0000-0000-0000-000000000013'::uuid, 'Trao đổi / Lên bản vẽ 3D', 'Trao đổi và lên bản vẽ 3D.'),
    ('41500000-0000-0000-0000-000000000014'::uuid, 'Trao đổi / Lên bản vẽ thi công', 'Trao đổi và tải bản vẽ thi công.'),
    ('41500000-0000-0000-0000-000000000019'::uuid, 'GĐ Rà soát định mức quản lý', 'Giám đốc rà soát định mức quản lý.'),
    ('41500000-0000-0000-0000-000000000023'::uuid, 'Mua hàng / QS Mua hàng với Tổ Đội / Thầu Phụ / Cung cấp vật tư', 'Làm việc mua hàng với tổ đội, thầu phụ, nhà cung cấp vật tư.'),
    ('41500000-0000-0000-0000-000000000024'::uuid, 'Mua hàng / QS Ký hợp đồng / Hồ sơ tạm ứng với Tổ Đội / Thầu Phụ / Nhà cung cấp vật tư', 'Ký hợp đồng và chuẩn bị hồ sơ tạm ứng đầu vào.'),
    ('41500000-0000-0000-0000-000000000025'::uuid, 'PM / GĐ Rà soát & Kiểm tra Đầu Vào', 'PM/GĐ rà soát và kiểm tra đầu vào.'),
    ('41500000-0000-0000-0000-000000000028'::uuid, 'GĐ Rà soát & Chốt báo cáo đơn giá', 'Giám đốc rà soát và chốt báo cáo đơn giá.'),
    ('41500000-0000-0000-0000-000000000030'::uuid, 'Kế toán / Pháp lý kiểm tra lại hợp đồng', 'Kế toán/pháp lý kiểm tra hợp đồng.'),
    ('41500000-0000-0000-0000-000000000032'::uuid, 'PM / GĐ rà soát và ký duyệt', 'PM/GĐ rà soát và ký duyệt.'),
    ('41500000-0000-0000-0000-000000000041'::uuid, '[GĐ] Rà soát & Kiểm tra hồ sơ kiểm tra khối lượng thầu phụ', 'Giám đốc rà soát/kiểm tra hồ sơ khối lượng thầu phụ.'),
    ('41500000-0000-0000-0000-000000000049'::uuid, 'Xuất hóa đơn', 'Xuất hóa đơn cho chủ đầu tư.'),
    ('41500000-0000-0000-0000-000000000050'::uuid, 'Thanh toán (thanh toán nhiều kỳ)', 'Ghi nhận thanh toán nhiều kỳ từ chủ đầu tư.'),
    ('41500000-0000-0000-0000-000000000051'::uuid, 'Kế toán thầu phụ xuất hóa đơn về công ty', 'Ghi nhận hóa đơn thầu phụ xuất về công ty.'),
    ('41500000-0000-0000-0000-000000000052'::uuid, 'Kế toán xuất hóa đơn cho chủ đầu tư', 'Ghi nhận hóa đơn xuất cho chủ đầu tư và đóng tài chính.')
)
UPDATE pipeline_run_steps run_step
SET name=updates.name,
    description=updates.description
FROM step_updates updates, pipeline_run_nodes run_node, pipeline_runs run
WHERE run_step.template_step_id=updates.id
  AND run_step.run_node_id=run_node.id
  AND run_node.run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000020';

UPDATE pipeline_template_edges
SET label='Đầu vào bắt buộc đã được duyệt'
WHERE id='41600000-0000-0000-0000-000000000007'
  AND template_id='40000000-0000-0000-0000-000000000020';

UPDATE pipeline_run_edges run_edge
SET label='Đầu vào bắt buộc đã được duyệt'
FROM pipeline_runs run
WHERE run_edge.run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000020'
  AND run_edge.template_edge_id='41600000-0000-0000-0000-000000000007';
