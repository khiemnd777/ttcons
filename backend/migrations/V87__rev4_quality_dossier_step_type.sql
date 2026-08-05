-- Preserve the published Rev3 snapshot and correct only the future Rev4 report/runtime definition.
-- The client quality dossier is a document-backed QA/QC step, not a construction daily report.

UPDATE pipeline_template_steps step
SET step_type='document_upload',
    description='QA/QC lập và quản lý hồ sơ chất lượng làm việc với chủ đầu tư; checklist nghiệp vụ và tài liệu hồ sơ đều bắt buộc. Giám sát phụ trách báo cáo hiện trường ở bước riêng.'
FROM pipeline_template_nodes node
WHERE node.id=step.node_id
  AND node.template_id='40000000-0000-0000-0000-000000000023'
  AND step.business_code='planning.client_quality_file'
  AND step.required_role_ids='["QA/QC"]'::jsonb
  AND step.checklist_required
  AND step.document_required
  AND NOT step.photo_required;
