-- Repair only the invalid lifecycle pair emitted by Developer Sample Data
-- blueprint versions before v4. Real projects are deliberately excluded.
UPDATE pipeline_run_steps AS run_step
SET status = 'completed'
FROM work_items AS work_item
JOIN projects AS project ON project.id = work_item.project_id
WHERE work_item.run_step_id = run_step.id
  AND work_item.project_id = run_step.project_id
  AND project.is_sample = true
  AND project.sample_blueprint_version < 4
  AND work_item.status = 'completed'
  AND run_step.status = 'submitted';
