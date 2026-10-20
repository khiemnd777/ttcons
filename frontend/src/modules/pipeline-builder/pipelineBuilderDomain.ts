import type { PipelineEdgeConditionDefinition, PipelineNode, PipelineTemplate, PipelineTemplateEdge, WorkflowFormCatalogItem } from '../../lib/types/domain';

export function isRecord(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

export function cleanString(value: unknown) {
  const text = String(value ?? '').trim();
  return text && text !== '<nil>' ? text : '';
}

export function boolValue(value: unknown, fallback = false) {
  if (typeof value === 'boolean') return value;
  if (value === undefined || value === null || value === '') return fallback;
  return ['true', '1', 'yes'].includes(String(value).toLocaleLowerCase('vi-VN'));
}

export function normalizePermissions(node: PipelineNode) {
  if (Array.isArray(node.permissions)) return node.permissions.map(String);
  if (node.required_permission) return [String(node.required_permission)];
  return [];
}

export function normalizeRoleIds(value: unknown) {
  const roles = Array.isArray(value) ? value.map((role) => String(role).trim()).filter(Boolean) : [];
  return roles.filter((role, index) => roles.indexOf(role) === index);
}

export function sortedStepsForNode(node?: PipelineNode) {
  return [...(node?.steps ?? [])].sort((a: Record<string, any>, b: Record<string, any>) => {
    const aOrder = Number(a.order_index ?? 0);
    const bOrder = Number(b.order_index ?? 0);
    if (aOrder !== bOrder) return aOrder - bOrder;
    return String(a.created_at ?? a.id ?? '').localeCompare(String(b.created_at ?? b.id ?? ''), 'vi');
  });
}

export function validEdgePriority(value: string) {
  const priority = Number(value);
  return Number.isInteger(priority) && priority >= 1 && priority <= 999;
}

export function defaultStepsForType(type: string) {
  const stepsByType: Record<string, string[]> = {
    survey: ['Upload ảnh hiện trạng', 'Ghi nhận kích thước chính', 'Xác định rủi ro mặt bằng'],
    design: ['Upload bản vẽ', 'Review phương án thiết kế', 'Khách hàng xác nhận'],
    boq: ['Bóc tách khối lượng', 'Nhập đơn giá dự kiến', 'Kiểm tra ngân sách'],
    budget_approval: ['Kiểm tra ngân sách', 'Đối chiếu hạn mức', 'Duyệt ngân sách'],
    procurement: ['Sinh nhu cầu vật tư từ BOQ', 'Kiểm tra tồn kho', 'Tạo đề xuất mua vật tư'],
    warehouse_issue: ['Kiểm tra tồn kho', 'Lập phiếu xuất kho', 'Xác nhận vật tư ra công trình'],
    construction: ['Daily report thi công', 'Checklist thầu phụ', 'Ghi nhận vật tư sử dụng', 'Kiểm tra chất lượng'],
    acceptance: ['Checklist nghiệm thu', 'Checklist chủ đầu tư', 'Upload ảnh minh chứng', 'Pass / Fail nghiệm thu'],
  };
  return stepsByType[type] ?? ['Checklist', 'Review', 'Approval'];
}

export function defaultStepTypeForNode(type?: string) {
  if (type === 'procurement' || type === 'warehouse_issue') return 'material_check';
  if (type === 'acceptance') return 'acceptance_check';
  if (type === 'handover') return 'handover_check';
  if (type === 'construction') return 'daily_report';
  if (type === 'boq' || type === 'budget_approval') return 'budget_check';
  return 'task';
}

export function defaultStepTypeForStep(nodeType: string, stepName: string) {
  const normalizedName = normalizedSearchText(stepName);
  if (normalizedName.includes('daily report') || normalizedName.includes('nhat ky')) return 'daily_report';
  if (normalizedName.includes('vat tu')) return 'material_check';
  if (
    normalizedName.includes('chu dau tu')
    || normalizedName.includes('client')
    || normalizedName.includes('owner')
    || normalizedName.includes('thau phu')
    || normalizedName.includes('subcontractor')
    || normalizedName.includes('checklist')
    || normalizedName.includes('kiem tra chat luong')
  ) return 'checklist';
  if (normalizedName.includes('duyet') || normalizedName.includes('xac nhan')) return 'approval';
  return defaultStepTypeForNode(nodeType);
}

export function defaultRoleForNode(type?: string) {
  if (type === 'survey' || type === 'construction') return 'Giám sát';
  if (type === 'boq') return 'QS';
  if (type === 'budget_approval' || type === 'financial_closure') return 'Kế toán';
  if (type === 'procurement') return 'Mua hàng';
  if (type === 'warehouse_issue') return 'Thủ kho';
  if (type === 'acceptance') return 'QA/QC';
  if (type === 'rework') return 'Đội thi công';
  return 'PM';
}

export function workflowFormRequiredForStepType(stepType: string) {
  return ['daily_report', 'meeting_minutes', 'checklist', 'acceptance_check', 'handover_check'].includes(stepType);
}

function workflowFormTypeForStep(stepType: string) {
  if (stepType === 'daily_report') return 'daily_report';
  if (stepType === 'meeting_minutes') return 'meeting_minutes';
  if (['checklist', 'acceptance_check', 'handover_check'].includes(stepType)) return 'checklist';
  return 'generic_submission';
}

export function defaultWorkflowFormVersionForStep(stepType: string, catalog: WorkflowFormCatalogItem[], nodeType?: string, stepName = '') {
  const defaults = catalog.filter((form) => form.default_eligible !== false);
  const preferredCode = preferredWorkflowFormCodeForStep(stepType, nodeType, stepName);
  if (preferredCode) {
    const preferred = defaults.find((form) => form.template_code === preferredCode);
    if (preferred) return preferred.version_id;
  }
  const formType = workflowFormTypeForStep(stepType);
  return defaults.find((form) => form.form_type === formType)?.version_id ?? defaults.find((form) => form.form_type === 'generic_submission')?.version_id ?? defaults[0]?.version_id ?? '';
}

export function workflowFormOptions(forms: WorkflowFormCatalogItem[], selectedVersionId: string) {
  return forms.filter((form) => form.selectable !== false || form.version_id === selectedVersionId);
}

export function workflowFormsForTemplate(catalog: WorkflowFormCatalogItem[], pinned: WorkflowFormCatalogItem[]) {
  const catalogIds = new Set(catalog.map((form) => String(form.version_id)));
  return [...catalog, ...pinned
    .filter((form) => !catalogIds.has(String(form.version_id)))
    .map((form) => ({ ...form, selectable: false }))];
}

function preferredWorkflowFormCodeForStep(stepType: string, nodeType?: string, stepName = '') {
  const normalizedName = normalizedSearchText(stepName);
  if (stepType === 'daily_report') {
    if (
      nodeType === 'construction'
      || normalizedName.includes('thi cong')
      || normalizedName.includes('cong truong')
      || normalizedName.includes('construction')
      || normalizedName.includes('site report')
    ) {
      return 'builtin_construction_daily_report';
    }
  }
  if (stepType === 'checklist' && (nodeType === 'construction' || nodeType === 'acceptance')) {
    if (normalizedName.includes('chu dau tu') || normalizedName.includes('client') || normalizedName.includes('owner')) {
      return 'builtin_owner_client_checklist';
    }
    if (normalizedName.includes('thau phu') || normalizedName.includes('subcontractor')) {
      return 'builtin_subcontractor_checklist';
    }
  }
  return '';
}

function normalizedSearchText(value: string) {
  return value
    .toLocaleLowerCase('vi-VN')
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/đ/g, 'd');
}

export function validBusinessCode(value: unknown) {
  return /^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$/.test(cleanString(value)) && cleanString(value).length <= 120;
}

export function nextStepBusinessCode(node: PipelineNode | undefined, steps: Record<string, any>[]) {
  const nodeCode = cleanString(node?.business_code) || 'custom_node';
  const used = new Set(steps.map((step) => cleanString(step.business_code)).filter(Boolean));
  let index = steps.length + 1;
  while (used.has(`${nodeCode}.step_${index}`)) index += 1;
  return `${nodeCode}.step_${index}`;
}

export function pipelineTemplatePDFDownloadName(template: PipelineTemplate) {
  const slug = normalizedSearchText(template.name ?? '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '') || 'template';
  const revision = Math.max(1, Number(template.revision_no ?? template.version ?? 1) || 1);
  return `workflow-${slug}-rev-${revision}.pdf`;
}

export function buildValidationIssues(
  nodes: PipelineNode[],
  edges: PipelineTemplateEdge[],
  conditions: PipelineEdgeConditionDefinition[],
  workflowFormVersionIds: Set<string>,
  allowUnknownConditions: boolean,
) {
  const issues: string[] = [];
  const conditionByType = new Map(conditions.map((condition) => [condition.type, condition]));
  if (!nodes.length) issues.push('Template chưa có node');
  nodes.forEach((node) => {
    if (!node.name?.trim()) issues.push('Có node thiếu tên');
    (node.steps ?? []).forEach((step: Record<string, any>) => {
      if (boolValue(step.form_required) && !cleanString(step.form_version_id)) {
        issues.push(`Step ${step.name || step.id} bắt buộc workflow form nhưng chưa chọn form`);
      }
      if (cleanString(step.form_version_id) && workflowFormVersionIds.size > 0 && !workflowFormVersionIds.has(cleanString(step.form_version_id))) {
        issues.push(`Step ${step.name || step.id} đang dùng workflow form chưa published hoặc không còn active`);
      }
    });
  });
  if (nodes.length > 1 && !edges.length) issues.push('Chưa có cạnh được lưu; hiện chỉ có thứ tự node mặc định');

  const nodeIds = new Set(nodes.map((node) => node.id));
  const indegree = new Map(nodes.map((node) => [node.id, 0]));
  const outdegree = new Map(nodes.map((node) => [node.id, 0]));
  const adjacency = new Map(nodes.map((node) => [node.id, [] as string[]]));
  edges.forEach((edge) => {
    if (!nodeIds.has(String(edge.from_node_id))) issues.push('Có cạnh trỏ từ node đã xóa');
    if (!nodeIds.has(String(edge.to_node_id))) issues.push('Có cạnh trỏ tới node đã xóa');
    if (String(edge.label ?? '').trim().length > 80) issues.push(`Connector ${edge.id} có nhãn quá dài`);
    if (!String(edge.condition_type ?? '').trim()) issues.push('Có connector thiếu điều kiện chuyển tiếp');
    if (!allowUnknownConditions && conditionByType.size && !conditionByType.has(String(edge.condition_type ?? ''))) issues.push(`Connector ${edge.label || edge.id} dùng điều kiện chuyển tiếp không được hỗ trợ`);
    if (!validEdgePriority(String(edge.priority ?? 100))) issues.push(`Connector ${edge.label || edge.id} có priority không hợp lệ`);
    if (nodeIds.has(String(edge.from_node_id)) && nodeIds.has(String(edge.to_node_id))) {
      const from = String(edge.from_node_id);
      const to = String(edge.to_node_id);
      outdegree.set(from, (outdegree.get(from) ?? 0) + 1);
      indegree.set(to, (indegree.get(to) ?? 0) + 1);
      adjacency.get(from)?.push(to);
    }
    const config = isRecord(edge.condition_config) ? edge.condition_config : {};
    const missingConfigLabels = (conditionByType.get(String(edge.condition_type ?? ''))?.config_schema ?? [])
      .filter((field) => field.required && cleanString(config[field.key]) === '')
      .map((field) => field.label);
    if (missingConfigLabels.length) issues.push(`Connector ${edge.label || edge.id} thiếu ${missingConfigLabels.join(', ')}`);
  });
  if (nodes.length) {
    const roots = nodes.filter((node) => (indegree.get(node.id) ?? 0) === 0);
    const sinks = nodes.filter((node) => (outdegree.get(node.id) ?? 0) === 0);
    if (roots.length !== 1) issues.push(`Template phải có đúng một node bắt đầu; hiện có ${roots.length}`);
    if (sinks.length !== 1) issues.push(`Template phải có đúng một node kết thúc; hiện có ${sinks.length}`);
    if (roots.length === 1) {
      const visited = new Set<string>();
      const queue = [roots[0].id];
      while (queue.length) {
        const nodeId = queue.shift()!;
        if (visited.has(nodeId)) continue;
        visited.add(nodeId);
        queue.push(...(adjacency.get(nodeId) ?? []));
      }
      if (visited.size !== nodes.length) issues.push(`Có ${nodes.length - visited.size} node không thể đi tới từ node bắt đầu`);
    }
    const remainingIndegree = new Map(indegree);
    const queue = nodes.filter((node) => (remainingIndegree.get(node.id) ?? 0) === 0).map((node) => node.id);
    let processed = 0;
    while (queue.length) {
      const nodeId = queue.shift()!;
      processed += 1;
      (adjacency.get(nodeId) ?? []).forEach((nextId) => {
        const nextDegree = (remainingIndegree.get(nextId) ?? 0) - 1;
        remainingIndegree.set(nextId, nextDegree);
        if (nextDegree === 0) queue.push(nextId);
      });
    }
    if (processed !== nodes.length) issues.push('Template có connector tạo chu trình không được kiểm soát');
  }
  return issues;
}
