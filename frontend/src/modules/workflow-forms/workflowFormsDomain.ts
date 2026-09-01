import { resolveCurrencyFractionDigits } from '../../lib/utils/currencyInput';
import type {
  AnyRecord,
  WorkflowFormField,
  WorkflowFormManagedTemplate,
  WorkflowFormManagedVersion,
  WorkflowFormTemplateLibraryItem,
} from '../../lib/types/domain';

export type TemplateForm = {
  code: string;
  name: string;
  description: string;
  form_type: string;
  status: string;
};

export type DraftField = {
  field_key: string;
  label: string;
  field_type: string;
  required: boolean;
  help_text: string;
  order_index: number;
  optionsText: string;
  validationText: string;
  visibilityText: string;
};

export type FieldErrors = Record<string, string>;
export type ConfirmAction = 'publish' | 'archive-template' | 'archive-version' | null;

export const formTypeOptions = [
  ['generic_submission', 'Biểu mẫu gửi'],
  ['checklist', 'Checklist'],
  ['daily_report', 'Báo cáo ngày'],
  ['meeting_minutes', 'Biên bản họp'],
];

export const fieldTypeOptions = [
  ['text', 'Văn bản'],
  ['textarea', 'Văn bản dài'],
  ['number', 'Số'],
  ['currency', 'Tiền tệ'],
  ['date', 'Ngày'],
  ['select', 'Danh sách chọn'],
  ['multi_select', 'Nhiều lựa chọn'],
  ['checkbox', 'Có / Không'],
  ['evidence_reference', 'Minh chứng'],
  ['structured_list', 'Danh sách cấu trúc'],
];

export const blankTemplateForm: TemplateForm = {
  code: '',
  name: '',
  description: '',
  form_type: 'generic_submission',
  status: 'active',
};

export function templateMatches(template: WorkflowFormTemplateLibraryItem, search: string, type: string, status: string) {
  const haystack = `${template.template_code} ${template.template_name} ${template.template_description ?? ''}`.toLowerCase();
  const needle = search.trim().toLowerCase();
  if (needle && !haystack.includes(needle)) return false;
  if (type !== 'all' && template.form_type !== type) return false;
  if (status !== 'all' && template.template_status !== status) return false;
  return true;
}

export function templateMissingFields(form: TemplateForm, requireCode: boolean) {
  const missing: string[] = [];
  if (requireCode && !form.code.trim()) missing.push('Mã biểu mẫu');
  if (!form.name.trim()) missing.push('Tên biểu mẫu');
  if (!form.form_type.trim()) missing.push('Loại biểu mẫu');
  return missing;
}

export function fieldToDraft(field: WorkflowFormField): DraftField {
  return {
    field_key: field.field_key,
    label: field.label,
    field_type: field.field_type,
    required: Boolean(field.required),
    help_text: field.help_text ?? '',
    order_index: Number(field.order_index ?? 0),
    optionsText: stableJSONString(field.options_json ?? []),
    validationText: stableJSONString(field.validation_json ?? {}),
    visibilityText: stableJSONString(field.visibility_condition_json ?? {}),
  };
}

export function blankDraftField(index: number): DraftField {
  return {
    field_key: `field_${index + 1}`,
    label: 'Trường mới',
    field_type: 'text',
    required: false,
    help_text: '',
    order_index: (index + 1) * 10,
    optionsText: '[]',
    validationText: '{}',
    visibilityText: '{}',
  };
}

export function draftFieldsPayload(fields: DraftField[]) {
  return fields.map((field, index) => ({
    field_key: field.field_key.trim(),
    label: field.label.trim(),
    field_type: field.field_type,
    required: field.required,
    help_text: field.help_text.trim(),
    order_index: field.order_index || (index + 1) * 10,
    options_json: parseJSONOrDefault(field.optionsText, []),
    validation_json: parseJSONOrDefault(field.validationText, {}),
    visibility_condition_json: parseJSONOrDefault(field.visibilityText, {}),
  }));
}

export function validateDraftFields(fields: DraftField[]) {
  const errors: FieldErrors = {};
  const seen = new Set<string>();
  fields.forEach((field, index) => {
    const key = field.field_key.trim();
    if (!/^[a-z][a-z0-9_]*$/.test(key)) errors[`${index}.field_key`] = 'Khóa kỹ thuật phải có dạng snake_case';
    if (seen.has(key)) errors[`${index}.field_key`] = 'Khóa kỹ thuật bị trùng';
    seen.add(key);
    if (!field.label.trim()) errors[`${index}.label`] = 'Tên trường là bắt buộc';
    if (!validJSON(field.optionsText)) errors[`${index}.options_json`] = 'JSON không hợp lệ';
    if (!validJSON(field.validationText)) errors[`${index}.validation_json`] = 'JSON không hợp lệ';
    if (field.field_type === 'currency' && validJSON(field.validationText)) {
      const validation = parseJSONObject(field.validationText) ?? {};
      const currencyCode = String(validation.currency_code ?? 'VND');
      const fractionDigits = Number(validation.maximum_fraction_digits ?? resolveCurrencyFractionDigits(currencyCode));
      if (!/^[A-Z]{3}$/.test(currencyCode)) errors[`${index}.validation_json`] = 'Mã tiền tệ phải gồm 3 chữ cái viết hoa';
      else if (!Number.isInteger(fractionDigits) || fractionDigits < 0 || fractionDigits > 6) errors[`${index}.validation_json`] = 'Số lẻ tối đa phải từ 0 đến 6';
      else if (currencyCode === 'VND' && fractionDigits !== 0) errors[`${index}.validation_json`] = 'VND phải dùng 0 chữ số thập phân';
      else if (validation.min !== undefined && validation.max !== undefined && Number(validation.min) > Number(validation.max)) errors[`${index}.validation_json`] = 'Giá trị nhỏ nhất không được lớn hơn giá trị lớn nhất';
    }
    if (!validJSON(field.visibilityText)) errors[`${index}.visibility_json`] = 'JSON không hợp lệ';
  });
  if (!fields.length) errors.form = 'Cần ít nhất một trường dữ liệu';
  return errors;
}

export function stableJSONString(value: unknown) {
  try {
    return JSON.stringify(value ?? {}, null, 2);
  } catch {
    return '{}';
  }
}

export function parseJSONOrDefault(value: string, fallback: AnyRecord | unknown[]) {
  try {
    return JSON.parse(value || JSON.stringify(fallback));
  } catch {
    return fallback;
  }
}

export function parseJSONObject(value: string): AnyRecord | null {
  try {
    const parsed = JSON.parse(value || '{}');
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

export function validJSON(value: string) {
  try {
    JSON.parse(value || '{}');
    return true;
  } catch {
    return false;
  }
}

export function formTypeLabel(value?: string) {
  return formTypeOptions.find(([type]) => type === value)?.[1] ?? value ?? 'Biểu mẫu workflow';
}

export function fieldTypeLabel(value?: string) {
  return fieldTypeOptions.find(([type]) => type === value)?.[1] ?? value ?? 'Chưa xác định';
}

export function versionStatusLabel(value?: string) {
  if (value === 'draft') return 'Bản nháp';
  if (value === 'published') return 'Đã phát hành';
  if (value === 'archived') return 'Đã lưu trữ';
  return value || 'Chưa xác định';
}

export function draftFieldsFingerprint(fields: DraftField[]) {
  return JSON.stringify(fields.map((field) => ({
    field_key: field.field_key,
    label: field.label,
    field_type: field.field_type,
    required: field.required,
    help_text: field.help_text,
    order_index: field.order_index,
    optionsText: field.optionsText,
    validationText: field.validationText,
    visibilityText: field.visibilityText,
  })));
}

export function confirmationContent(
  action: ConfirmAction,
  template?: WorkflowFormManagedTemplate,
  version?: WorkflowFormManagedVersion,
  fieldCount = 0,
) {
  if (action === 'publish') {
    return {
      title: `Phát hành phiên bản v${version?.version_no ?? ''}?`,
      description: `${fieldCount} trường dữ liệu sẽ được lưu và phát hành.`,
      notice: 'Sau khi phát hành, nội dung phiên bản này trở thành bất biến. Muốn chỉnh sửa tiếp, anh cần tạo một bản nháp mới.',
      confirmLabel: 'Phát hành phiên bản',
    };
  }
  if (action === 'archive-template') {
    return {
      title: 'Lưu trữ biểu mẫu?',
      description: template?.template_name ?? '',
      notice: 'Biểu mẫu sẽ không còn xuất hiện trong bộ lọc “Đang dùng”. Các phiên bản và dữ liệu lịch sử vẫn được giữ nguyên.',
      confirmLabel: 'Lưu trữ biểu mẫu',
    };
  }
  if (action === 'archive-version') {
    return {
      title: `Lưu trữ phiên bản v${version?.version_no ?? ''}?`,
      description: template?.template_name ?? '',
      notice: 'Phiên bản này sẽ được chuyển sang trạng thái lưu trữ. Dữ liệu lịch sử liên quan vẫn được giữ nguyên.',
      confirmLabel: 'Lưu trữ phiên bản',
    };
  }
  return { title: '', description: '', notice: '', confirmLabel: '' };
}
