import type { DataPackageFormat } from '../_shared/dataPackage';

export type WorkflowDefinitionScope = 'form' | 'pipeline' | 'bundle';

export type WorkflowDefinitionImportPayload = {
  scope: WorkflowDefinitionScope;
  workbook_version: string;
  package_version: string;
  source_format: DataPackageFormat;
  source_filename: string;
  source_size_bytes: number;
  pipelines: Array<Record<string, unknown>>;
  pipeline_nodes: Array<Record<string, unknown>>;
  pipeline_steps: Array<Record<string, unknown>>;
  pipeline_edges: Array<Record<string, unknown>>;
  form_templates: Array<Record<string, unknown>>;
  form_versions: Array<Record<string, unknown>>;
  form_fields: Array<Record<string, unknown>>;
};

export type WorkflowDefinitionIssue = {
  code: string;
  message: string;
  sheet: string;
  entity_key: string;
  row_number?: number;
};

export type WorkflowDefinitionImportPreview = {
  scope: WorkflowDefinitionScope;
  workbook_version: string;
  package_version: string;
  source_format: DataPackageFormat;
  source_filename: string;
  pipeline_count: number;
  form_count: number;
  blocker_count: number;
  warning_count: number;
  blockers: WorkflowDefinitionIssue[];
  warnings: WorkflowDefinitionIssue[];
  entities: Array<{ kind: 'form' | 'pipeline'; key: string; name: string; action: 'create_template' | 'create_version' | 'create_family' | 'create_revision'; target?: string }>;
  preview_hash: string;
  reference_fingerprint: string;
  confirmation: string;
};

export type WorkflowDefinitionImportResult = {
  import_id: string;
  source_format: DataPackageFormat;
  scope: WorkflowDefinitionScope;
  pipeline_count: number;
  form_count: number;
  status: 'draft';
  idempotent_replay: boolean;
  pipelines: Array<{ pipeline_key: string; template_id: string; template_family_id: string; revision_no: number; status: 'draft'; engine_version: number }>;
  forms: Array<{ form_key: string; template_id: string; version_id: string; version_no: number; status: 'draft'; definition_hash: string }>;
};
