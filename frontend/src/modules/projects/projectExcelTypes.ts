import type { DataPackageFormat } from '../_shared/dataPackage';

export type ProjectExcelIssue = {
  field: string;
  message: string;
};

export type ProjectExcelReadinessItem = {
  code: string;
  message: string;
  route: string;
  count?: number;
};

export type ProjectExcelReadiness = {
  ready: boolean;
  published_pipeline_count: number;
  ready_pipeline_count: number;
  invalid_pipeline_count: number;
  active_form_template_count: number;
  published_form_version_count: number;
  valid_form_version_count: number;
  field_count: number;
  active_client_partner_count: number;
  blockers: ProjectExcelReadinessItem[];
  warnings: ProjectExcelReadinessItem[];
  pipeline_builder_route: string;
  workflow_forms_route: string;
  reference_fingerprint: string;
};

export type ProjectExcelImportPreviewRow = {
  row_number: number;
  project_code: string;
  project_name: string;
  client_name: string;
  client_partner_code: string;
  pipeline_ref: string;
  pipeline_name: string;
  pipeline_revision: number;
  estimated_budget_vnd: number;
  valid: boolean;
  errors: ProjectExcelIssue[];
  warnings: ProjectExcelIssue[];
};

export type ProjectExcelImportPreview = {
  workbook_version: string;
  package_version: string;
  source_format: DataPackageFormat;
  source_filename: string;
  total_count: number;
  valid_count: number;
  invalid_count: number;
  warning_count: number;
  preview_hash: string;
  reference_fingerprint: string;
  readiness: ProjectExcelReadiness;
  rows: ProjectExcelImportPreviewRow[];
};

export type ProjectExcelImportResult = {
  import_id: string;
  source_format: DataPackageFormat;
  source_filename: string;
  created_count: number;
  pipeline_started_count: number;
  idempotent_replay: boolean;
  items: Array<{
    row_number: number;
    project_id: string;
    project_code: string;
    pipeline_ref: string;
  }>;
};
