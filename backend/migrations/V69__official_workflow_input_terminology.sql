-- Use “đầu vào” as the collective workflow term for tổ đội, thầu phụ and
-- material suppliers. Keep entity-specific wording in negotiation,
-- acceptance, settlement and invoice steps where the distinction matters.
WITH step_updates(id,name,description) AS (
  VALUES
    ('42500000-0000-0000-0000-000000000017'::uuid,'Lấy giá đầu vào','Thu thập giá đầu vào.'),
    ('42500000-0000-0000-0000-000000000023'::uuid,'Mua hàng đầu vào','Thực hiện mua hàng đầu vào.'),
    ('42500000-0000-0000-0000-000000000024'::uuid,'Ký hợp đồng và lập hồ sơ tạm ứng đầu vào','Ký hợp đồng và chuẩn bị hồ sơ tạm ứng đầu vào.')
)
UPDATE pipeline_template_steps step
SET name=updates.name,
    description=updates.description
FROM step_updates updates
JOIN pipeline_template_nodes node
  ON node.template_id='40000000-0000-0000-0000-000000000021'
WHERE step.id=updates.id
  AND step.node_id=node.id;

UPDATE pipeline_template_nodes
SET description='Bước bắt buộc để chốt khối lượng, mua hàng, hợp đồng/tạm ứng và rà soát đầu vào.',
    exit_conditions=jsonb_set(
      exit_conditions,
      '{text}',
      to_jsonb('Đầu vào đã được rà soát và kiểm tra'::text),
      true
    ),
    updated_at=now()
WHERE id='42400000-0000-0000-0000-000000000006'
  AND template_id='40000000-0000-0000-0000-000000000021';

UPDATE pipeline_template_edges
SET label='Đầu vào đã được kiểm tra'
WHERE id='42600000-0000-0000-0000-000000000007'
  AND template_id='40000000-0000-0000-0000-000000000021';

UPDATE workflow_form_templates
SET name=CASE code
      WHEN 'client_boq_supplier_price_collection' THEN 'Thu thập giá đầu vào'
      WHEN 'client_procurement_sourcing' THEN 'Mua hàng đầu vào'
      ELSE name
    END,
    description=CASE code
      WHEN 'client_procurement_sourcing'
        THEN 'Ghi nhận nguồn đầu vào, phạm vi mua hàng, so sánh báo giá, lead time và phương án lựa chọn.'
      ELSE description
    END,
    updated_at=now()
WHERE code IN (
  'client_boq_supplier_price_collection',
  'client_procurement_sourcing'
);

-- Existing revision-2 runs keep their workflow state, role assignments,
-- answers and audit history. Only user-facing wording snapshots are aligned.
UPDATE pipeline_run_nodes run_node
SET description='Bước bắt buộc để chốt khối lượng, mua hàng, hợp đồng/tạm ứng và rà soát đầu vào.',
    exit_conditions=jsonb_set(
      run_node.exit_conditions,
      '{text}',
      to_jsonb('Đầu vào đã được rà soát và kiểm tra'::text),
      true
    )
FROM pipeline_runs run
WHERE run_node.run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000021'
  AND run_node.template_node_id='42400000-0000-0000-0000-000000000006';

UPDATE pipeline_run_edges run_edge
SET label='Đầu vào đã được kiểm tra'
FROM pipeline_runs run
WHERE run_edge.run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000021'
  AND run_edge.template_edge_id='42600000-0000-0000-0000-000000000007';

WITH step_updates(id,name,description) AS (
  VALUES
    ('42500000-0000-0000-0000-000000000017'::uuid,'Lấy giá đầu vào','Thu thập giá đầu vào.'),
    ('42500000-0000-0000-0000-000000000023'::uuid,'Mua hàng đầu vào','Thực hiện mua hàng đầu vào.'),
    ('42500000-0000-0000-0000-000000000024'::uuid,'Ký hợp đồng và lập hồ sơ tạm ứng đầu vào','Ký hợp đồng và chuẩn bị hồ sơ tạm ứng đầu vào.')
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
  VALUES
    ('42500000-0000-0000-0000-000000000017'::uuid,'Lấy giá đầu vào','Thu thập giá đầu vào.'),
    ('42500000-0000-0000-0000-000000000023'::uuid,'Mua hàng đầu vào','Thực hiện mua hàng đầu vào.'),
    ('42500000-0000-0000-0000-000000000024'::uuid,'Ký hợp đồng và lập hồ sơ tạm ứng đầu vào','Ký hợp đồng và chuẩn bị hồ sơ tạm ứng đầu vào.')
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
  VALUES
    ('42500000-0000-0000-0000-000000000017'::uuid,'Lấy giá đầu vào'),
    ('42500000-0000-0000-0000-000000000023'::uuid,'Mua hàng đầu vào'),
    ('42500000-0000-0000-0000-000000000024'::uuid,'Ký hợp đồng và lập hồ sơ tạm ứng đầu vào')
)
UPDATE workflow_form_instances form_instance
SET title=updates.name,
    updated_at=now()
FROM step_updates updates,pipeline_runs run
WHERE form_instance.template_step_id=updates.id
  AND form_instance.pipeline_run_id=run.id
  AND run.template_id='40000000-0000-0000-0000-000000000021';
