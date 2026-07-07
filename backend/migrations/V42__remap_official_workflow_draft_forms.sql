WITH remap_candidates AS (
  SELECT
    wfi.id AS instance_id,
    pts.form_version_id AS target_form_version_id,
    target_version.template_id AS target_form_template_id,
    target_template.form_type AS target_form_type,
    pts.name AS target_title
  FROM workflow_form_instances wfi
  JOIN workflow_form_templates current_template ON current_template.id=wfi.form_template_id
  JOIN pipeline_runs pr ON pr.id=wfi.pipeline_run_id
  JOIN pipeline_template_steps pts ON pts.id=wfi.template_step_id
  JOIN workflow_form_versions target_version ON target_version.id=pts.form_version_id
  JOIN workflow_form_templates target_template ON target_template.id=target_version.template_id
  WHERE pr.template_id='40000000-0000-0000-0000-000000000020'::uuid
    AND current_template.code='builtin_generic_submission'
    AND target_template.code<>'builtin_generic_submission'
    AND wfi.status='draft'
    AND wfi.submitted_at IS NULL
    AND wfi.current_revision=0
    AND NOT EXISTS (
      SELECT 1
      FROM workflow_form_answers answer
      WHERE answer.instance_id=wfi.id
    )
    AND NOT EXISTS (
      SELECT 1
      FROM workflow_form_signatures signature
      WHERE signature.form_instance_id=wfi.id
    )
    AND NOT EXISTS (
      SELECT 1
      FROM workflow_form_pdf_exports pdf
      WHERE pdf.form_instance_id=wfi.id
    )
)
UPDATE workflow_form_instances wfi
SET form_template_id=remap.target_form_template_id,
    form_version_id=remap.target_form_version_id,
    form_type=remap.target_form_type,
    title=remap.target_title,
    answers_snapshot_json='{}'::jsonb,
    updated_at=now()
FROM remap_candidates remap
WHERE wfi.id=remap.instance_id;
