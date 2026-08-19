import type { WorkflowFormField } from '../../lib/types/domain';

const workflowOtherOptionValue = 'other';

export const workflowSelectOtherTextMaxLength = 300;

export type WorkflowSelectOtherTextConfig = {
  inputLabel: string;
  placeholder: string;
  maxLength: number;
};

const workflowSelectOtherTextConfigs = new Map<string, WorkflowSelectOtherTextConfig>([
  ['client_design_style_brief.style_direction', {
    inputLabel: 'Nhập phong cách khác',
    placeholder: 'Ví dụ: Japandi, Đông Dương đương đại…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['client_survey_work_package.style_direction', {
    inputLabel: 'Nhập phong cách khác',
    placeholder: 'Ví dụ: Japandi, Đông Dương đương đại…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['client_boq_owner_submission.sent_channel', {
    inputLabel: 'Kênh gửi khác',
    placeholder: 'Ví dụ: Cổng hồ sơ của chủ đầu tư…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['client_hardcopy_dispatch.delivery_method', {
    inputLabel: 'Cách gửi khác',
    placeholder: 'Ví dụ: Đơn vị vận chuyển nội bộ…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['builtin_construction_daily_report.construction_scope', {
    inputLabel: 'Phạm vi thi công khác',
    placeholder: 'Ví dụ: Cảnh quan ngoài nhà…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['builtin_construction_daily_report.weather', {
    inputLabel: 'Điều kiện thi công khác',
    placeholder: 'Ví dụ: Hạn chế tiếng ồn theo khung giờ…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['builtin_subcontractor_checklist.work_package', {
    inputLabel: 'Gói / phạm vi thi công khác',
    placeholder: 'Ví dụ: Chống thấm sân thượng…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['client_subcontractor_acceptance_package.work_package', {
    inputLabel: 'Gói / phạm vi thi công khác',
    placeholder: 'Ví dụ: Chống thấm sân thượng…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['builtin_daily_report.planning_scope', {
    inputLabel: 'Phạm vi kế hoạch khác',
    placeholder: 'Ví dụ: Pháp lý hiện trường…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['builtin_checklist.planning_scope', {
    inputLabel: 'Phạm vi kế hoạch khác',
    placeholder: 'Ví dụ: Pháp lý hiện trường…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['client_owner_soft_file_submission.delivery_channel', {
    inputLabel: 'Kênh gửi khác',
    placeholder: 'Ví dụ: Cổng hồ sơ của chủ đầu tư…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
  ['client_procurement_takeoff_review.takeoff_source', {
    inputLabel: 'Nguồn khối lượng khác',
    placeholder: 'Ví dụ: Biên bản xác nhận tại hiện trường…',
    maxLength: workflowSelectOtherTextMaxLength,
  }],
]);

export function workflowSelectOtherTextConfig(templateCode: string | undefined, field: WorkflowFormField) {
  if (field.field_type !== 'select' || !workflowSelectOptionValues(field).has(workflowOtherOptionValue)) return undefined;
  return workflowSelectOtherTextConfigs.get(workflowSelectOtherTextConfigKey(templateCode, field.field_key));
}

export function workflowSelectSupportsOtherText(templateCode: string | undefined, field: WorkflowFormField) {
  return Boolean(workflowSelectOtherTextConfig(templateCode, field));
}

export function workflowSelectOptionValue(templateCode: string | undefined, field: WorkflowFormField, value: unknown) {
  const rawValue = workflowTextValue(value);
  if (!workflowSelectSupportsOtherText(templateCode, field) || rawValue === '' || workflowSelectOptionValues(field).has(rawValue)) {
    return rawValue;
  }
  return workflowOtherOptionValue;
}

export function workflowSelectOtherTextValue(templateCode: string | undefined, field: WorkflowFormField, value: unknown) {
  if (!workflowSelectSupportsOtherText(templateCode, field)) return '';
  const rawValue = workflowTextValue(value);
  if (rawValue === '' || workflowSelectOptionValues(field).has(rawValue)) return '';
  return rawValue;
}

export function workflowSelectOptionChangeValue(
  templateCode: string | undefined,
  field: WorkflowFormField,
  currentValue: unknown,
  nextOptionValue: string,
) {
  if (!workflowSelectSupportsOtherText(templateCode, field) || nextOptionValue !== workflowOtherOptionValue) return nextOptionValue;
  return workflowSelectOptionValue(templateCode, field, currentValue) === workflowOtherOptionValue
    ? workflowSelectOtherTextValue(templateCode, field, currentValue) || workflowOtherOptionValue
    : workflowOtherOptionValue;
}

export function workflowSelectOtherTextChangeValue(templateCode: string | undefined, field: WorkflowFormField, nextText: string) {
  if (!workflowSelectSupportsOtherText(templateCode, field)) return nextText;
  return nextText === '' ? workflowOtherOptionValue : nextText;
}

export function workflowSelectOtherTextIssue(templateCode: string | undefined, field: WorkflowFormField, value: unknown) {
  const config = workflowSelectOtherTextConfig(templateCode, field);
  if (!config || workflowSelectOptionValue(templateCode, field, value) !== workflowOtherOptionValue) return '';
  const customText = workflowSelectOtherTextValue(templateCode, field, value).trim();
  const errorLabel = config.inputLabel.replace(/^Nhập\s+/u, '');
  if (!customText) return `${errorLabel} là bắt buộc`;
  if (Array.from(customText).length > config.maxLength) {
    return `${errorLabel} không được vượt quá ${config.maxLength} ký tự`;
  }
  return '';
}

function workflowSelectOtherTextConfigKey(templateCode: string | undefined, fieldKey: string) {
  return `${String(templateCode ?? '').trim()}.${fieldKey}`;
}

function workflowSelectOptionValues(field: WorkflowFormField) {
  return new Set((field.options_json ?? []).map((option) => (
    typeof option === 'string' ? option : String(option.value ?? '')
  )).filter(Boolean));
}

function workflowTextValue(value: unknown) {
  if (value === null || value === undefined) return '';
  return String(value);
}
