-- Add the approved operational roles without broadening them beyond the
-- permissions required by their official workflow steps.
INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,module
FROM (VALUES
  ('project.view','Xem công trình','Xem công trình được phân quyền','project'),
  ('pipeline.view','Xem pipeline','Xem quy trình của công trình được phân quyền','pipeline'),
  ('planning.view','Xem kế hoạch','Xem kế hoạch công trình','planning'),
  ('planning.edit','Cập nhật kế hoạch','Cập nhật nội dung kế hoạch được phân công','planning'),
  ('operation.view','Xem thi công','Xem hoạt động thi công','operation'),
  ('operation.report','Báo cáo thi công','Lập báo cáo thi công','operation'),
  ('boq.view','Xem BOQ','Xem BOQ công trình','boq'),
  ('boq.update','Cập nhật BOQ','Cập nhật BOQ công trình','boq'),
  ('boq.approve','Duyệt BOQ','Duyệt BOQ công trình','boq'),
  ('contract.view','Xem hợp đồng','Xem hợp đồng công trình','contract'),
  ('contract.review','Review hợp đồng','Rà soát hợp đồng công trình','contract')
) permission_seed(key,name,description,module)
ON CONFLICT (key) DO NOTHING;

INSERT INTO roles (id,name,display_name,description,system_role)
VALUES
  ('20000000-0000-0000-0000-000000000014','design','Thiết kế','Phụ trách phương án và hồ sơ thiết kế',true),
  ('20000000-0000-0000-0000-000000000015','sales','Sale','Phụ trách phối hợp khách hàng, BOQ và hồ sơ thương mại',true),
  ('20000000-0000-0000-0000-000000000016','legal','Pháp lý','Phụ trách rà soát pháp lý hợp đồng',true)
ON CONFLICT (name) DO UPDATE
SET display_name=EXCLUDED.display_name,
    description=EXCLUDED.description,
    system_role=EXCLUDED.system_role;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id
FROM roles r
JOIN permissions p ON
  (r.name='design' AND p.key IN (
    'project.view','pipeline.view','planning.view','planning.edit','operation.view','operation.report'
  )) OR
  (r.name='sales' AND p.key IN (
    'project.view','pipeline.view','planning.view','planning.edit',
    'boq.view','boq.update','boq.approve','contract.view','contract.review'
  )) OR
  (r.name='legal' AND p.key IN (
    'project.view','pipeline.view','contract.view','contract.review'
  )) OR
  (r.name='construction_team' AND p.key IN (
    'pipeline.view','planning.view','planning.edit'
  ))
ON CONFLICT DO NOTHING;

-- The role labels embedded in the approved wording become structured role
-- assignments only on the published template. Existing runtime role snapshots
-- remain untouched so active and historical work keeps its original access.
WITH step_updates(id,name,description,roles) AS (
  VALUES
    ('42500000-0000-0000-0000-000000000001'::uuid,'Khảo sát yêu cầu của chủ đầu tư','Ghi nhận yêu cầu chính của chủ đầu tư và phạm vi khảo sát. Phụ trách chính: PM, Giám đốc. Phối hợp: QS, Sale, Đội thi công.','["PM","Giám đốc","QS","Sale","Đội thi công"]'::jsonb),
    ('42500000-0000-0000-0000-000000000002'::uuid,'Chụp và tải ảnh hiện trạng','Chụp và tải ảnh hiện trạng.','["Giám sát"]'::jsonb),
    ('42500000-0000-0000-0000-000000000003'::uuid,'Nắm phong cách thiết kế','Ghi nhận phong cách thiết kế mong muốn.','["Thiết kế"]'::jsonb),
    ('42500000-0000-0000-0000-000000000004'::uuid,'Ghi nhận kích thước chính','Ghi nhận kích thước chính phục vụ BOQ và thiết kế.','["QS","Thiết kế"]'::jsonb),
    ('42500000-0000-0000-0000-000000000005'::uuid,'Xác định rủi ro mặt bằng','Xác định rủi ro mặt bằng và lưu ý kỹ thuật.','["QS","Đội thi công"]'::jsonb),
    ('42500000-0000-0000-0000-000000000006'::uuid,'Hoàn tất biên bản khảo sát','Hoàn tất biên bản khảo sát.','["PM","Giám sát"]'::jsonb),
    ('42500000-0000-0000-0000-000000000007'::uuid,'Bóc khối lượng cho nhánh không thiết kế','Bóc khối lượng phục vụ nhánh không thiết kế.','["QS"]'::jsonb),
    ('42500000-0000-0000-0000-000000000008'::uuid,'Lên bản vẽ bố trí mặt bằng','Lên bản vẽ bố trí mặt bằng sơ bộ nếu cần thiết kế.','["Thiết kế"]'::jsonb),
    ('42500000-0000-0000-0000-000000000009'::uuid,'Lên bản vẽ 3D','Lên bản vẽ 3D sơ bộ nếu cần thiết kế.','["Thiết kế"]'::jsonb),
    ('42500000-0000-0000-0000-000000000010'::uuid,'Lên bản vẽ thi công','Lên bản vẽ thi công sơ bộ nếu cần thiết kế.','["Thiết kế"]'::jsonb),
    ('42500000-0000-0000-0000-000000000011'::uuid,'Chốt biên bản cuộc họp và nhánh xử lý','Chốt biên bản cuộc họp và nhánh xử lý tiếp theo.','["PM","QS","Giám sát"]'::jsonb),
    ('42500000-0000-0000-0000-000000000012'::uuid,'Trao đổi và hoàn thiện bản vẽ bố trí mặt bằng','Trao đổi và hoàn thiện bản vẽ bố trí mặt bằng.','["PM"]'::jsonb),
    ('42500000-0000-0000-0000-000000000013'::uuid,'Trao đổi và hoàn thiện bản vẽ 3D','Trao đổi và hoàn thiện bản vẽ 3D.','["PM"]'::jsonb),
    ('42500000-0000-0000-0000-000000000014'::uuid,'Trao đổi và hoàn thiện bản vẽ thi công','Trao đổi và hoàn thiện bản vẽ thi công.','["PM"]'::jsonb),
    ('42500000-0000-0000-0000-000000000015'::uuid,'Chốt phương án với Sale và chủ đầu tư','Chốt phương án với Sale và chủ đầu tư.','["PM","Chủ đầu tư"]'::jsonb),
    ('42500000-0000-0000-0000-000000000016'::uuid,'Bàn giao bản vẽ cho QS','Bàn giao bản vẽ cho QS để làm BOQ.','["PM","QS"]'::jsonb),
    ('42500000-0000-0000-0000-000000000017'::uuid,'Lấy giá thầu phụ, tổ đội và vật tư','Lấy giá từ thầu phụ, tổ đội và vật tư.','["QS"]'::jsonb),
    ('42500000-0000-0000-0000-000000000018'::uuid,'Kiểm tra đơn giá và tính hợp lý của đầu vào','Kiểm tra đơn giá và tính hợp lý của đầu vào.','["QS","PM","Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000019'::uuid,'Review định mức quản lý','Giám đốc review định mức quản lý.','["Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000020'::uuid,'Gửi BOQ cho chủ đầu tư','Gửi BOQ cho chủ đầu tư.','["QS","PM","Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000021'::uuid,'Gửi BOQ cho chủ đầu tư','Gửi BOQ cho chủ đầu tư sau thiết kế.','["QS","Sale","PM"]'::jsonb),
    ('42500000-0000-0000-0000-000000000022'::uuid,'Bóc khối lượng đầu vào','Bóc khối lượng đầu vào.','["QS"]'::jsonb),
    ('42500000-0000-0000-0000-000000000023'::uuid,'Mua hàng với tổ đội, thầu phụ và nhà cung cấp vật tư','Làm việc mua hàng với tổ đội, thầu phụ và nhà cung cấp vật tư.','["Mua hàng","QS"]'::jsonb),
    ('42500000-0000-0000-0000-000000000024'::uuid,'Ký hợp đồng và lập hồ sơ tạm ứng với tổ đội, thầu phụ và nhà cung cấp vật tư','Ký hợp đồng và chuẩn bị hồ sơ tạm ứng đầu vào.','["Mua hàng","QS","Kế toán"]'::jsonb),
    ('42500000-0000-0000-0000-000000000025'::uuid,'Rà soát và kiểm tra đầu vào','PM và Giám đốc rà soát, kiểm tra đầu vào.','["PM","Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000026'::uuid,'Đàm phán với thầu phụ để chốt giá hợp lý','Đàm phán với thầu phụ để chốt giá hợp lý.','["Sale","QS","PM","Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000027'::uuid,'Chốt định mức sau đàm phán','Chốt định mức sau đàm phán.','["QS","PM"]'::jsonb),
    ('42500000-0000-0000-0000-000000000028'::uuid,'Review và chốt báo cáo đơn giá','Giám đốc review và chốt báo cáo đơn giá.','["Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000029'::uuid,'Lập báo giá và hợp đồng','Lập báo giá và hợp đồng với chủ đầu tư.','["QS","PM"]'::jsonb),
    ('42500000-0000-0000-0000-000000000030'::uuid,'Kiểm tra hợp đồng','Kế toán và Pháp lý kiểm tra hợp đồng.','["Kế toán","Pháp lý"]'::jsonb),
    ('42500000-0000-0000-0000-000000000031'::uuid,'Gửi hồ sơ điện tử cho chủ đầu tư','Gửi hồ sơ điện tử gồm hợp đồng, bảng tiến độ, đề nghị tạm ứng và nội dung ký duyệt hai bên cho chủ đầu tư.','["Sale","PM","QS"]'::jsonb),
    ('42500000-0000-0000-0000-000000000032'::uuid,'Rà soát và ký duyệt hợp đồng','PM và Giám đốc rà soát, ký duyệt hợp đồng.','["PM","Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000033'::uuid,'In và gửi hồ sơ bản cứng cho chủ đầu tư','In và gửi bản cứng hợp đồng, hồ sơ tạm ứng cho chủ đầu tư.','["PM","Kế toán"]'::jsonb),
    ('42500000-0000-0000-0000-000000000034'::uuid,'Họp và phân công thi công','Họp và phân công thi công cho QS, PM, Thiết kế và QA/QC.','["QS","PM","Thiết kế","QA/QC"]'::jsonb),
    ('42500000-0000-0000-0000-000000000035'::uuid,'Chốt biên bản cuộc họp','Chốt biên bản cuộc họp thi công và QA/QC.','["PM","QA/QC"]'::jsonb),
    ('42500000-0000-0000-0000-000000000036'::uuid,'Lập kế hoạch thi công và đảm bảo chất lượng','Lập kế hoạch thi công và đảm bảo chất lượng.','["QA/QC","Giám sát"]'::jsonb),
    ('42500000-0000-0000-0000-000000000060'::uuid,'Chốt hạng mục giám sát và bàn giao thi công','Lập một lần danh sách kiểm tra sẽ được sinh tự động cho đội hiện trường.','["PM","QA/QC","Giám sát"]'::jsonb),
    ('42500000-0000-0000-0000-000000000037'::uuid,'Báo cáo liên tục trong quá trình thi công','QA/QC và Giám sát báo cáo liên tục trong quá trình thi công.','["QA/QC","Giám sát"]'::jsonb),
    ('42500000-0000-0000-0000-000000000038'::uuid,'Nghiệm thu thầu phụ và tổ đội','Nghiệm thu thầu phụ, tổ đội và đưa số liệu cho QS kiểm tra.','["QA/QC","QS"]'::jsonb),
    ('42500000-0000-0000-0000-000000000039'::uuid,'Nghiệm thu vật tư đầu vào và đầu ra','Nghiệm thu vật tư đầu vào, đầu ra và đưa số liệu cho QS kiểm tra.','["QA/QC","QS"]'::jsonb),
    ('42500000-0000-0000-0000-000000000040'::uuid,'Lập hồ sơ kiểm tra khối lượng thầu phụ','Lập hồ sơ kiểm tra khối lượng thầu phụ và áp giá.','["QS"]'::jsonb),
    ('42500000-0000-0000-0000-000000000041'::uuid,'Rà soát và kiểm tra hồ sơ khối lượng thầu phụ','Giám đốc rà soát, kiểm tra hồ sơ khối lượng thầu phụ.','["Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000042'::uuid,'Lập hồ sơ thanh toán cho thầu phụ','Lập hồ sơ thanh toán cho thầu phụ.','["QS","Kế toán"]'::jsonb),
    ('42500000-0000-0000-0000-000000000043'::uuid,'Ký nghiệm thu hồ sơ thanh toán','Giám đốc ký nghiệm thu hồ sơ thanh toán.','["Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000044'::uuid,'Kiểm tra hồ sơ thanh toán','Kế toán kiểm tra hồ sơ thanh toán.','["Kế toán"]'::jsonb),
    ('42500000-0000-0000-0000-000000000045'::uuid,'Thanh toán cho thầu phụ theo đợt','Thanh toán cho thầu phụ theo đợt.','["Kế toán"]'::jsonb),
    ('42500000-0000-0000-0000-000000000046'::uuid,'Nghiệm thu và ký duyệt công trình','Nghiệm thu và ký duyệt công trình.','["QA/QC","QS","PM","Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000047'::uuid,'Ký duyệt hồ sơ nghiệm thu','Giám đốc ký duyệt hồ sơ nghiệm thu.','["Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000048'::uuid,'Lập hồ sơ thanh toán, quyết toán và ký duyệt','Lập hồ sơ thanh toán, quyết toán và ký duyệt giữa các bên.','["Kế toán","QS","PM","Giám đốc"]'::jsonb),
    ('42500000-0000-0000-0000-000000000049'::uuid,'Xuất hóa đơn cho chủ đầu tư','Xuất hóa đơn cho chủ đầu tư.','["Kế toán"]'::jsonb),
    ('42500000-0000-0000-0000-000000000050'::uuid,'Ghi nhận thanh toán từ chủ đầu tư theo từng kỳ','Ghi nhận thanh toán từ chủ đầu tư theo từng kỳ.','["Kế toán"]'::jsonb),
    ('42500000-0000-0000-0000-000000000051'::uuid,'Ghi nhận hóa đơn thầu phụ xuất về công ty','Ghi nhận hóa đơn thầu phụ xuất về công ty.','["Kế toán"]'::jsonb),
    ('42500000-0000-0000-0000-000000000052'::uuid,'Ghi nhận hóa đơn xuất cho chủ đầu tư và đóng tài chính','Ghi nhận hóa đơn xuất cho chủ đầu tư và đóng tài chính.','["Kế toán","Giám đốc"]'::jsonb)
)
UPDATE pipeline_template_steps step
SET name=updates.name,
    description=updates.description,
    required_role_ids=updates.roles
FROM step_updates updates
JOIN pipeline_template_nodes node ON node.template_id='40000000-0000-0000-0000-000000000021'
WHERE step.id=updates.id
  AND step.node_id=node.id;

UPDATE pipeline_template_nodes
SET name='Đầu Vào',updated_at=now()
WHERE id='42400000-0000-0000-0000-000000000006'
  AND template_id='40000000-0000-0000-0000-000000000021';

UPDATE pipeline_run_nodes run_node
SET name='Đầu Vào'
FROM pipeline_runs run
WHERE run_node.run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000021'
  AND run_node.template_node_id='42400000-0000-0000-0000-000000000006';

WITH step_updates(id,name,description) AS (
  SELECT id,name,description
  FROM pipeline_template_steps
  WHERE id::text LIKE '42500000-0000-0000-0000-0000000000%'
    AND node_id IN (
      SELECT id FROM pipeline_template_nodes
      WHERE template_id='40000000-0000-0000-0000-000000000021'
    )
)
UPDATE pipeline_run_steps run_step
SET name=updates.name,
    description=updates.description
FROM step_updates updates,pipeline_run_nodes run_node,pipeline_runs run
WHERE run_step.template_step_id=updates.id
  AND run_step.run_node_id=run_node.id
  AND run_node.run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000021';

WITH step_updates(id,name,description) AS (
  SELECT id,name,description
  FROM pipeline_template_steps
  WHERE id::text LIKE '42500000-0000-0000-0000-0000000000%'
    AND node_id IN (
      SELECT id FROM pipeline_template_nodes
      WHERE template_id='40000000-0000-0000-0000-000000000021'
    )
)
UPDATE work_items work_item
SET title=updates.name,
    description=updates.description,
    updated_at=now()
FROM step_updates updates,pipeline_run_steps run_step,pipeline_runs run
WHERE work_item.source_type='pipeline_run_step'
  AND work_item.source_id=run_step.id
  AND run_step.template_step_id=updates.id
  AND work_item.pipeline_run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000021';

WITH step_updates(id,name) AS (
  SELECT id,name
  FROM pipeline_template_steps
  WHERE id::text LIKE '42500000-0000-0000-0000-0000000000%'
    AND node_id IN (
      SELECT id FROM pipeline_template_nodes
      WHERE template_id='40000000-0000-0000-0000-000000000021'
    )
)
UPDATE workflow_form_instances form_instance
SET title=updates.name,updated_at=now()
FROM step_updates updates,pipeline_runs run
WHERE form_instance.template_step_id=updates.id
  AND form_instance.pipeline_run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000021';
