import type { WorkBook, WorkSheet } from 'xlsx';
import xlsxMini from 'xlsx/dist/xlsx.mini.min.js';
import {
  WORKFLOW_DEFINITION_SHEETS,
  WORKFLOW_DEFINITION_PACKAGE_VERSION,
  WORKFLOW_DEFINITION_WORKBOOK_VERSION,
} from './workflowDefinitionExcel';
import type { WorkflowDefinitionScope } from './workflowDefinitionExcelTypes';
import type { DataPackageFormat } from '../_shared/dataPackage';
import { assertExactKeys, isJsonRecord, parseStrictJson } from '../_shared/strictJson';

type SheetJs = typeof import('xlsx');
type WorkerRequest = { type: 'parse'; sourceFormat: DataPackageFormat; buffer: ArrayBuffer };
type WorkerScope = { onmessage: ((event: MessageEvent<WorkerRequest>) => void) | null; postMessage: (message: unknown) => void };

const scope = self as unknown as WorkerScope;
const xlsx = xlsxMini as unknown as SheetJs;

scope.onmessage = (event) => {
  try {
	if (event.data.sourceFormat === 'json') {
		parseJSONPackage(event.data.buffer);
		return;
	}
    const workbook = xlsx.read(new Uint8Array(event.data.buffer), {
      type: 'array', raw: true, cellDates: false, cellFormula: true, cellHTML: false,
      cellNF: false, cellStyles: false, bookVBA: false, sheetRows: 10002,
    });
    const manifestSheet = requiredSheet(workbook, 'Manifest');
    rejectFormulas(manifestSheet, 'Manifest');
    const manifest = readKeyValues(manifestSheet);
    const workbookVersion = manifest.workbook_version ?? '';
    if (workbookVersion !== WORKFLOW_DEFINITION_WORKBOOK_VERSION) {
      throw new Error(`Phiên bản file không được hỗ trợ. Hệ thống yêu cầu workbook_version=${WORKFLOW_DEFINITION_WORKBOOK_VERSION}.`);
    }
    const definitionScope = manifest.scope as WorkflowDefinitionScope;
    if (!['form', 'pipeline', 'bundle'].includes(definitionScope)) {
      throw new Error('Manifest.scope phải là form, pipeline hoặc bundle.');
    }
    for (const sheetName of ['Huong_dan', 'Ref_Permissions', 'Ref_Roles', 'Ref_Edge_Conditions']) {
      rejectFormulas(requiredSheet(workbook, sheetName), sheetName);
    }
    const parsedSheets: Record<string, Array<Record<string, string>>> = {};
    for (const [sheetName, headers] of Object.entries(WORKFLOW_DEFINITION_SHEETS)) {
      const worksheet = requiredSheet(workbook, sheetName);
      rejectFormulas(worksheet, sheetName);
      parsedSheets[sheetName] = readRows(worksheet, headers, sheetName);
    }
    if (!parsedSheets.Pipelines.length && !parsedSheets.Form_Templates.length) {
      throw new Error('Workbook cần ít nhất một dòng Pipeline hoặc Form Template.');
    }
    if (parsedSheets.Pipelines.length > 25 || parsedSheets.Form_Templates.length > 100) {
      throw new Error('Workbook vượt giới hạn 25 Pipeline hoặc 100 Form.');
    }
    scope.postMessage({
      type: 'parsed', scope: definitionScope, workbook_version: workbookVersion, package_version: workbookVersion, source_format: 'xlsx',
      pipelines: parsedSheets.Pipelines,
      pipeline_nodes: parsedSheets.Pipeline_Nodes,
      pipeline_steps: parsedSheets.Pipeline_Steps,
      pipeline_edges: parsedSheets.Pipeline_Edges,
      form_templates: parsedSheets.Form_Templates,
      form_versions: parsedSheets.Form_Versions,
      form_fields: parsedSheets.Form_Fields,
    });
  } catch (error) {
    scope.postMessage({ type: 'error', message: error instanceof Error ? error.message : 'Không thể đọc Workflow Definition Workbook.' });
  }
};

const PIPELINE_KEYS = ['pipeline_key', 'target_family_id', 'name', 'description', 'project_type', 'engine_version', 'report_config_json', 'source', 'nodes', 'edges'] as const;
const NODE_KEYS = ['node_key', 'business_code', 'name', 'description', 'type', 'position_x', 'position_y', 'order_index', 'responsible_role_ids', 'required_permission', 'permissions', 'entry_conditions', 'exit_conditions', 'approval_rule', 'approval_policy_json', 'rework_enabled', 'replanning_enabled', 'fail_path', 'status', 'steps'] as const;
const STEP_KEYS = ['step_key', 'business_code', 'name', 'description', 'step_type', 'required_role_ids', 'required_permission', 'checklist_required', 'document_required', 'photo_required', 'form_ref', 'form_required', 'surface_kind', 'domain_action_key', 'order_index'] as const;
const EDGE_KEYS = ['edge_key', 'from_node_key', 'to_node_key', 'condition_type', 'condition_config', 'label', 'priority'] as const;
const FIELD_KEYS = ['field_key', 'label', 'field_type', 'required', 'options_json', 'validation_json', 'help_text', 'order_index', 'visibility_condition_json'] as const;

function parseJSONPackage(buffer: ArrayBuffer) {
  const text = new TextDecoder('utf-8', { fatal: true }).decode(buffer);
  const value = parseStrictJson(text);
  if (!isJsonRecord(value)) throw new Error('Workflow Definition JSON phải là một object.');
  assertExactKeys(value, ['schema', 'schema_version', 'scope', 'mode', 'generated_at', 'reference_fingerprint', 'instructions', 'pipelines', 'forms', 'references'], 'Workflow Definition package');
  if (value.schema !== 'consflow.workflow-definition-package' || value.schema_version !== Number(WORKFLOW_DEFINITION_PACKAGE_VERSION)) {
    throw new Error(`JSON không đúng Workflow Definition schema v${WORKFLOW_DEFINITION_PACKAGE_VERSION}.`);
  }
  const definitionScope = value.scope as WorkflowDefinitionScope;
  if (!['form', 'pipeline', 'bundle'].includes(definitionScope)) throw new Error('scope phải là form, pipeline hoặc bundle.');
  if (!Array.isArray(value.pipelines) || !Array.isArray(value.forms)) throw new Error('Gói JSON cần mảng pipelines và forms.');
  if (!value.pipelines.length && !value.forms.length) throw new Error('Gói import cần ít nhất một Pipeline hoặc Form.');
  if (value.pipelines.length > 25 || value.forms.length > 100) throw new Error('Gói import vượt giới hạn 25 Pipeline hoặc 100 Form.');

  const pipelines: Array<Record<string, unknown>> = [];
  const pipelineNodes: Array<Record<string, unknown>> = [];
  const pipelineSteps: Array<Record<string, unknown>> = [];
  const pipelineEdges: Array<Record<string, unknown>> = [];
  value.pipelines.forEach((item, pipelineIndex) => {
    if (!isJsonRecord(item)) throw new Error(`pipelines[${pipelineIndex}] phải là object.`);
    assertExactKeys(item, PIPELINE_KEYS, `pipelines[${pipelineIndex}]`);
    if (!isJsonRecord(item.source) || !Array.isArray(item.nodes) || !Array.isArray(item.edges)) throw new Error(`pipelines[${pipelineIndex}] thiếu source, nodes hoặc edges hợp lệ.`);
    assertExactKeys(item.source, ['family_id', 'revision_no', 'definition_hash'], `pipelines[${pipelineIndex}].source`);
    pipelines.push({
      pipeline_key: item.pipeline_key, target_family_id: item.target_family_id, source_family_id: item.source.family_id,
      name: item.name, description: item.description, project_type: item.project_type, engine_version: item.engine_version,
      report_config_json: item.report_config_json, source_revision_no: item.source.revision_no, source_definition_hash: item.source.definition_hash,
    });
    item.nodes.forEach((nodeValue, nodeIndex) => {
      if (!isJsonRecord(nodeValue)) throw new Error(`pipelines[${pipelineIndex}].nodes[${nodeIndex}] phải là object.`);
      assertExactKeys(nodeValue, NODE_KEYS, `pipelines[${pipelineIndex}].nodes[${nodeIndex}]`);
      if (!Array.isArray(nodeValue.steps)) throw new Error(`pipelines[${pipelineIndex}].nodes[${nodeIndex}].steps phải là mảng.`);
      pipelineNodes.push({
        pipeline_key: item.pipeline_key, node_key: nodeValue.node_key, business_code: nodeValue.business_code, name: nodeValue.name,
        description: nodeValue.description, type: nodeValue.type, position_x: nodeValue.position_x, position_y: nodeValue.position_y,
        order_index: nodeValue.order_index, responsible_roles_json: nodeValue.responsible_role_ids, required_permission: nodeValue.required_permission,
        permissions_json: nodeValue.permissions, entry_conditions_json: nodeValue.entry_conditions, exit_conditions_json: nodeValue.exit_conditions,
        approval_rule: nodeValue.approval_rule, approval_policy_json: nodeValue.approval_policy_json, rework_enabled: nodeValue.rework_enabled,
        replanning_enabled: nodeValue.replanning_enabled, fail_path: nodeValue.fail_path, status: nodeValue.status,
      });
      nodeValue.steps.forEach((stepValue, stepIndex) => {
        if (!isJsonRecord(stepValue)) throw new Error(`pipelines[${pipelineIndex}].nodes[${nodeIndex}].steps[${stepIndex}] phải là object.`);
        assertExactKeys(stepValue, STEP_KEYS, `pipelines[${pipelineIndex}].nodes[${nodeIndex}].steps[${stepIndex}]`);
        pipelineSteps.push({
          pipeline_key: item.pipeline_key, node_key: nodeValue.node_key, step_key: stepValue.step_key, business_code: stepValue.business_code,
          name: stepValue.name, description: stepValue.description, step_type: stepValue.step_type, required_roles_json: stepValue.required_role_ids,
          required_permission: stepValue.required_permission, checklist_required: stepValue.checklist_required, document_required: stepValue.document_required,
          photo_required: stepValue.photo_required, form_ref: stepValue.form_ref, form_required: stepValue.form_required,
          surface_kind: stepValue.surface_kind, domain_action_key: stepValue.domain_action_key, order_index: stepValue.order_index,
        });
      });
    });
    item.edges.forEach((edgeValue, edgeIndex) => {
      if (!isJsonRecord(edgeValue)) throw new Error(`pipelines[${pipelineIndex}].edges[${edgeIndex}] phải là object.`);
      assertExactKeys(edgeValue, EDGE_KEYS, `pipelines[${pipelineIndex}].edges[${edgeIndex}]`);
      pipelineEdges.push({
        pipeline_key: item.pipeline_key, edge_key: edgeValue.edge_key, from_node_key: edgeValue.from_node_key, to_node_key: edgeValue.to_node_key,
        condition_type: edgeValue.condition_type, condition_config_json: edgeValue.condition_config, label: edgeValue.label, priority: edgeValue.priority,
      });
    });
  });

  const formTemplates: Array<Record<string, unknown>> = [];
  const formVersions: Array<Record<string, unknown>> = [];
  const formFields: Array<Record<string, unknown>> = [];
  value.forms.forEach((item, formIndex) => {
    if (!isJsonRecord(item)) throw new Error(`forms[${formIndex}] phải là object.`);
    assertExactKeys(item, ['form_key', 'template', 'source', 'version'], `forms[${formIndex}]`);
    if (!isJsonRecord(item.template) || !isJsonRecord(item.source) || !isJsonRecord(item.version)) throw new Error(`forms[${formIndex}] thiếu template, source hoặc version hợp lệ.`);
    assertExactKeys(item.template, ['template_code', 'name', 'description', 'form_type'], `forms[${formIndex}].template`);
    assertExactKeys(item.source, ['template_id', 'version_id', 'version_no', 'definition_hash'], `forms[${formIndex}].source`);
    assertExactKeys(item.version, ['schema_json', 'validation_json', 'fields'], `forms[${formIndex}].version`);
    if (!Array.isArray(item.version.fields)) throw new Error(`forms[${formIndex}].version.fields phải là mảng.`);
    formTemplates.push({ form_key: item.form_key, template_code: item.template.template_code, name: item.template.name, description: item.template.description, form_type: item.template.form_type, source_template_id: item.source.template_id });
    formVersions.push({ form_key: item.form_key, source_version_no: item.source.version_no, schema_json: item.version.schema_json, validation_json: item.version.validation_json, source_definition_hash: item.source.definition_hash });
    item.version.fields.forEach((fieldValue, fieldIndex) => {
      if (!isJsonRecord(fieldValue)) throw new Error(`forms[${formIndex}].version.fields[${fieldIndex}] phải là object.`);
      assertExactKeys(fieldValue, FIELD_KEYS, `forms[${formIndex}].version.fields[${fieldIndex}]`);
      formFields.push({ form_key: item.form_key, ...fieldValue });
    });
  });
  scope.postMessage({
    type: 'parsed', scope: definitionScope, workbook_version: WORKFLOW_DEFINITION_PACKAGE_VERSION,
    package_version: WORKFLOW_DEFINITION_PACKAGE_VERSION, source_format: 'json', pipelines,
    pipeline_nodes: pipelineNodes, pipeline_steps: pipelineSteps, pipeline_edges: pipelineEdges,
    form_templates: formTemplates, form_versions: formVersions, form_fields: formFields,
  });
}

function requiredSheet(workbook: WorkBook, expected: string) {
  const normalizedExpected = expected.toLocaleLowerCase('vi-VN');
  const name = workbook.SheetNames.find((item) => item.trim().toLocaleLowerCase('vi-VN') === normalizedExpected);
  const worksheet = name ? workbook.Sheets[name] : undefined;
  if (!worksheet) throw new Error(`Workbook thiếu sheet ${expected}. Hãy tải lại file mẫu mới.`);
  return worksheet;
}

function readKeyValues(worksheet: WorkSheet) {
  const rows = xlsx.utils.sheet_to_json<unknown[]>(worksheet, { header: 1, raw: false, defval: '', blankrows: false });
  const result: Record<string, string> = {};
  rows.slice(1).forEach((row) => {
    const key = String(row[0] ?? '').trim();
    if (key) result[key] = String(row[1] ?? '').trim();
  });
  return result;
}

function readRows(worksheet: WorkSheet, expectedHeaders: readonly string[], sheetName: string) {
  const values = xlsx.utils.sheet_to_json<unknown[]>(worksheet, { header: 1, raw: false, defval: '', blankrows: false });
  if (!values.length) throw new Error(`Sheet ${sheetName} chưa có dòng tiêu đề.`);
  const headers = values[0].map((value) => String(value ?? '').trim());
  if (headers.length !== expectedHeaders.length || expectedHeaders.some((header, index) => headers[index] !== header)) {
    throw new Error(`Dòng tiêu đề sheet ${sheetName} phải đúng thứ tự: ${expectedHeaders.join(', ')}.`);
  }
  return values.slice(1)
    .filter((row) => row.some((value) => String(value ?? '').trim() !== ''))
    .map((row) => Object.fromEntries(expectedHeaders.map((header, index) => [header, String(row[index] ?? '').trim()])));
}

function rejectFormulas(worksheet: WorkSheet, sheetName: string) {
  for (const [address, cell] of Object.entries(worksheet)) {
    if (address.startsWith('!')) continue;
    if (cell && typeof cell === 'object' && 'f' in cell && cell.f) {
      throw new Error(`Sheet ${sheetName} không được chứa công thức. Hãy chuyển các ô thành giá trị trước khi tải lên.`);
    }
  }
}
