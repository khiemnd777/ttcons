import type { WorkflowDefinitionImportPayload, WorkflowDefinitionScope } from './workflowDefinitionExcelTypes';
import type { DataPackageFormat } from '../_shared/dataPackage';

export const WORKFLOW_DEFINITION_WORKBOOK_VERSION = '1';
export const WORKFLOW_DEFINITION_PACKAGE_VERSION = '1';
export const WORKFLOW_DEFINITION_MAXIMUM_FILE_BYTES = 10 * 1024 * 1024;

export const WORKFLOW_DEFINITION_SHEETS = {
  Pipelines: ['pipeline_key', 'target_family_id', 'source_family_id', 'name', 'description', 'project_type', 'engine_version', 'report_config_json', 'source_revision_no', 'source_definition_hash'],
  Pipeline_Nodes: ['pipeline_key', 'node_key', 'business_code', 'name', 'description', 'type', 'position_x', 'position_y', 'order_index', 'responsible_roles_json', 'required_permission', 'permissions_json', 'entry_conditions_json', 'exit_conditions_json', 'approval_rule', 'approval_policy_json', 'rework_enabled', 'replanning_enabled', 'fail_path', 'status'],
  Pipeline_Steps: ['pipeline_key', 'node_key', 'step_key', 'business_code', 'name', 'description', 'step_type', 'required_roles_json', 'required_permission', 'checklist_required', 'document_required', 'photo_required', 'form_ref', 'form_required', 'surface_kind', 'domain_action_key', 'order_index'],
  Pipeline_Edges: ['pipeline_key', 'edge_key', 'from_node_key', 'to_node_key', 'condition_type', 'condition_config_json', 'label', 'priority'],
  Form_Templates: ['form_key', 'template_code', 'name', 'description', 'form_type', 'source_template_id'],
  Form_Versions: ['form_key', 'source_version_no', 'schema_json', 'validation_json', 'source_definition_hash'],
  Form_Fields: ['form_key', 'field_key', 'label', 'field_type', 'required', 'options_json', 'validation_json', 'help_text', 'order_index', 'visibility_condition_json'],
} as const;

export type ParsedWorkflowDefinitionFile = Omit<WorkflowDefinitionImportPayload, 'source_filename' | 'source_size_bytes'>;

type WorkerResponse =
  | ({ type: 'parsed' } & ParsedWorkflowDefinitionFile)
  | { type: 'error'; message: string };

export async function parseWorkflowDefinitionExcelFile(file: File): Promise<ParsedWorkflowDefinitionFile> {
  return parseWorkflowDefinitionImportFile(file, 'xlsx');
}

export async function parseWorkflowDefinitionImportFile(file: File, sourceFormat: DataPackageFormat): Promise<ParsedWorkflowDefinitionFile> {
  const expectedExtension = `.${sourceFormat}`;
  if (!file.name.toLocaleLowerCase('vi-VN').endsWith(expectedExtension)) {
	throw new Error(`Định dạng đã chọn chỉ chấp nhận tệp ${expectedExtension}.`);
  }
  if (file.size <= 0 || file.size > WORKFLOW_DEFINITION_MAXIMUM_FILE_BYTES) {
    throw new Error('Tệp phải có dung lượng không quá 10 MB.');
  }
  const buffer = await file.arrayBuffer();
  return new Promise<ParsedWorkflowDefinitionFile>((resolve, reject) => {
    const worker = new Worker(new URL('./workflowDefinitionExcel.worker.ts', import.meta.url), { type: 'module' });
    const finish = () => worker.terminate();
    worker.onmessage = (event: MessageEvent<WorkerResponse>) => {
      finish();
      if (event.data.type === 'parsed') {
        const { type: _messageType, ...payload } = event.data;
        resolve(payload);
      } else reject(new Error(event.data.message));
    };
    worker.onerror = () => {
      finish();
      reject(new Error('Không thể đọc gói dữ liệu Workflow Definition.'));
    };
    worker.postMessage({ type: 'parse', sourceFormat, buffer }, [buffer]);
  });
}

export function saveWorkflowDefinitionBlob(blob: Blob, scope: WorkflowDefinitionScope, mode: 'template' | 'export', format: DataPackageFormat = 'xlsx') {
  const now = new Date();
  const stamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = `workflow-definition-${scope}-${mode}-${stamp}.${format}`;
  anchor.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}

function pad(value: number) {
  return String(value).padStart(2, '0');
}
