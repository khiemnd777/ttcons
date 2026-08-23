ALTER TABLE project_excel_imports
  ADD COLUMN IF NOT EXISTS source_format TEXT NOT NULL DEFAULT 'xlsx';

ALTER TABLE project_excel_imports
  DROP CONSTRAINT IF EXISTS project_excel_imports_source_format_check;

ALTER TABLE project_excel_imports
  ADD CONSTRAINT project_excel_imports_source_format_check
  CHECK (source_format IN ('xlsx','json'));

ALTER TABLE workflow_definition_imports
  ADD COLUMN IF NOT EXISTS source_format TEXT NOT NULL DEFAULT 'xlsx';

ALTER TABLE workflow_definition_imports
  DROP CONSTRAINT IF EXISTS workflow_definition_imports_source_format_check;

ALTER TABLE workflow_definition_imports
  ADD CONSTRAINT workflow_definition_imports_source_format_check
  CHECK (source_format IN ('xlsx','json'));
