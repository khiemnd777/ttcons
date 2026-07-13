WITH corrected_steps(id, incorrect_name, corrected_name) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000012'::uuid, 'Trảo đổi / Lên bảng vẽ bố trí mặt bằng', 'Trao đổi / Lên bảng vẽ bố trí mặt bằng'),
    ('41500000-0000-0000-0000-000000000013'::uuid, 'Trảo đổi / Lên bảng vẽ 3D', 'Trao đổi / Lên bảng vẽ 3D'),
    ('41500000-0000-0000-0000-000000000014'::uuid, 'Trảo đổi / Lên bảng vẽ thi công', 'Trao đổi / Lên bảng vẽ thi công')
)
UPDATE pipeline_template_steps step
SET name=corrected.corrected_name
FROM corrected_steps corrected
WHERE step.id=corrected.id
  AND step.name=corrected.incorrect_name;

WITH corrected_steps(id, incorrect_name, corrected_name) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000012'::uuid, 'Trảo đổi / Lên bảng vẽ bố trí mặt bằng', 'Trao đổi / Lên bảng vẽ bố trí mặt bằng'),
    ('41500000-0000-0000-0000-000000000013'::uuid, 'Trảo đổi / Lên bảng vẽ 3D', 'Trao đổi / Lên bảng vẽ 3D'),
    ('41500000-0000-0000-0000-000000000014'::uuid, 'Trảo đổi / Lên bảng vẽ thi công', 'Trao đổi / Lên bảng vẽ thi công')
)
UPDATE pipeline_run_steps run_step
SET name=corrected.corrected_name
FROM corrected_steps corrected
WHERE run_step.template_step_id=corrected.id
  AND run_step.name=corrected.incorrect_name;

WITH corrected_steps(id, incorrect_name, corrected_name) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000012'::uuid, 'Trảo đổi / Lên bảng vẽ bố trí mặt bằng', 'Trao đổi / Lên bảng vẽ bố trí mặt bằng'),
    ('41500000-0000-0000-0000-000000000013'::uuid, 'Trảo đổi / Lên bảng vẽ 3D', 'Trao đổi / Lên bảng vẽ 3D'),
    ('41500000-0000-0000-0000-000000000014'::uuid, 'Trảo đổi / Lên bảng vẽ thi công', 'Trao đổi / Lên bảng vẽ thi công')
)
UPDATE work_items work_item
SET title=corrected.corrected_name
FROM pipeline_run_steps run_step
JOIN corrected_steps corrected ON corrected.id=run_step.template_step_id
WHERE work_item.run_step_id=run_step.id
  AND work_item.title=corrected.incorrect_name;

WITH corrected_steps(id, incorrect_name, corrected_name) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000012'::uuid, 'Trảo đổi / Lên bảng vẽ bố trí mặt bằng', 'Trao đổi / Lên bảng vẽ bố trí mặt bằng'),
    ('41500000-0000-0000-0000-000000000013'::uuid, 'Trảo đổi / Lên bảng vẽ 3D', 'Trao đổi / Lên bảng vẽ 3D'),
    ('41500000-0000-0000-0000-000000000014'::uuid, 'Trảo đổi / Lên bảng vẽ thi công', 'Trao đổi / Lên bảng vẽ thi công')
)
UPDATE workflow_form_instances form_instance
SET title=corrected.corrected_name
FROM work_items work_item
JOIN pipeline_run_steps run_step ON run_step.id=work_item.run_step_id
JOIN corrected_steps corrected ON corrected.id=run_step.template_step_id
WHERE form_instance.work_item_id=work_item.id
  AND form_instance.title=corrected.incorrect_name;
