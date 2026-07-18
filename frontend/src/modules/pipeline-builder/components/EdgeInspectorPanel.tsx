import { Save, X } from 'lucide-react';
import type { PipelineEdgeConditionDefinition, PipelineNode, PipelineTemplateEdge } from '../../../lib/types/domain';

type EdgeForm = {
  label: string;
  condition_type: string;
  priority: string;
  condition_config: Record<string, any>;
};

type EdgeInspectorPanelProps = {
  edge: PipelineTemplateEdge;
  fromNode?: PipelineNode;
  toNode?: PipelineNode;
  readOnly: boolean;
  readOnlyLabel?: string;
  conditions: PipelineEdgeConditionDefinition[];
  conditionsLoading: boolean;
  conditionLoadError: boolean;
  edgeForm: EdgeForm;
  conditionInvalid: boolean;
  priorityInvalid: boolean;
  answerConfigInvalid: boolean;
  answerFieldOptions: Array<{ label: string; value: string }>;
  savePending: boolean;
  onChangeField: (field: 'label' | 'condition_type' | 'priority', value: string) => void;
  onChangeConditionConfig: (field: string, value: any) => void;
  onSave: () => void;
  onClose: () => void;
};

export function EdgeInspectorPanel({
  edge,
  fromNode,
  toNode,
  readOnly,
  readOnlyLabel = 'Đã phát hành · Chỉ xem',
  conditions,
  conditionsLoading,
  conditionLoadError,
  edgeForm,
  conditionInvalid,
  priorityInvalid,
  answerConfigInvalid,
  answerFieldOptions,
  savePending,
  onChangeField,
  onChangeConditionConfig,
  onSave,
  onClose,
}: EdgeInspectorPanelProps) {
  const selectedCondition = conditions.find((condition) => condition.type === edgeForm.condition_type);
  const sourceName = fromNode?.name ?? edge.from_node_id;
  const targetName = toNode?.name ?? edge.to_node_id;
  const conditionOptions = conditions.map((condition) => ({ label: condition.label, value: condition.type }));
  if (edgeForm.condition_type && !conditionOptions.some((option) => option.value === edgeForm.condition_type)) {
    conditionOptions.push({ label: edgeForm.condition_type, value: edgeForm.condition_type });
  }
  const sourceFieldKey = String(edgeForm.condition_config.field_key ?? '');
  const sourceFieldOptions = [...answerFieldOptions];
  if (sourceFieldKey && !sourceFieldOptions.some((option) => option.value === sourceFieldKey)) {
    sourceFieldOptions.push({ label: sourceFieldKey, value: sourceFieldKey });
  }

  return (
    <aside className="pipeline-details-panel absolute bottom-4 right-4 top-20 z-30 w-[380px] overflow-auto rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-base font-semibold text-ink">Cấu hình Connector</h2>
            {readOnly ? <span className="rounded-md bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-800">{readOnlyLabel}</span> : null}
          </div>
          <p className="mt-1 truncate text-xs text-slate-500">{sourceName} → {targetName}</p>
        </div>
        <div className="flex gap-2">
          {!readOnly ? (
            <button title="Lưu connector" className="rounded-md bg-blue-600 p-2 text-white hover:bg-blue-700 disabled:opacity-50" disabled={savePending || conditionsLoading || conditionLoadError} onClick={onSave}>
              <Save className="h-4 w-4" />
            </button>
          ) : null}
          <button title="Đóng cấu hình" className="rounded-md border border-line p-2 text-slate-500 hover:bg-slate-100" onClick={onClose}>
            <X className="h-4 w-4" />
          </button>
        </div>
      </div>

      <div className="space-y-3">
        <Field
          label="Nhãn"
          value={edgeForm.label}
          invalidText="Nhãn tối đa 80 ký tự"
          disabled={readOnly}
          onChange={(value) => onChangeField('label', value)}
        />

        {edgeForm.condition_type === 'workflow_form_answer_equals' ? (
          <div className="space-y-3 rounded-md border border-blue-200 bg-blue-50/50 p-3">
            <Select
              label="Trường biểu mẫu nguồn"
              value={sourceFieldKey}
              options={sourceFieldOptions}
              invalid={answerConfigInvalid && !edgeForm.condition_config.field_key}
              invalidText={answerFieldOptions.length ? 'Chọn trường dùng để rẽ nhánh' : 'Node nguồn chưa có trường biểu mẫu khả dụng'}
              disabled={readOnly}
              onChange={(value) => onChangeConditionConfig('field_key', value)}
            />
            <Field
              label="Giá trị cần khớp"
              value={String(edgeForm.condition_config.value ?? '')}
              invalid={answerConfigInvalid && String(edgeForm.condition_config.value ?? '').trim() === ''}
              invalidText="Nhập giá trị dùng để rẽ nhánh"
              disabled={readOnly}
              onChange={(value) => onChangeConditionConfig('value', value)}
            />
          </div>
        ) : null}
        <Select
          label="Điều kiện chuyển tiếp"
          value={edgeForm.condition_type}
          options={conditionOptions}
          disabled={readOnly || conditionsLoading || conditionLoadError}
          invalid={conditionInvalid}
          invalidText={conditionLoadError ? 'Không tải được danh sách điều kiện chuyển tiếp' : 'Điều kiện chuyển tiếp không hợp lệ'}
          onChange={(value) => onChangeField('condition_type', value)}
        />
        <Field
          label="Ưu tiên"
          value={edgeForm.priority}
          type="number"
          min={1}
          max={999}
          invalid={priorityInvalid}
          invalidText="Ưu tiên phải từ 1 đến 999"
          disabled={readOnly}
          onChange={(value) => onChangeField('priority', value)}
        />

        {conditionLoadError ? (
          <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
            Không tải được danh mục điều kiện. Mã điều kiện đã lưu vẫn được hiển thị để kiểm tra.
          </div>
        ) : null}

        <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-semibold uppercase text-slate-500">Đánh giá</span>
            <span className="rounded bg-white px-2 py-1 text-xs font-semibold text-slate-600">{selectedCondition?.category ?? 'condition'}</span>
          </div>
          <p className="mt-2 text-sm leading-5 text-slate-700">
            {selectedCondition?.description ?? (conditionsLoading
              ? 'Đang tải danh sách điều kiện chuyển tiếp...'
              : conditionLoadError
                ? `Không tải được mô tả điều kiện. Mã đang lưu: ${edgeForm.condition_type || 'không có'}.`
                : 'Chọn điều kiện chuyển tiếp hợp lệ cho connector.')}
          </p>
        </div>

        <div className="rounded-md border border-line px-3 py-2">
          <dl className="grid grid-cols-[88px_1fr] gap-x-3 gap-y-2 text-sm">
            <dt className="font-semibold text-slate-500">Nguồn</dt>
            <dd className="min-w-0 truncate text-ink">{sourceName}</dd>
            <dt className="font-semibold text-slate-500">Đích</dt>
            <dd className="min-w-0 truncate text-ink">{targetName}</dd>
            <dt className="font-semibold text-slate-500">Edge ID</dt>
            <dd className="min-w-0 truncate text-slate-600">{edge.id}</dd>
          </dl>
        </div>
      </div>
    </aside>
  );
}

function Field({
  label,
  value,
  onChange,
  invalid,
  invalidText,
  type = 'text',
  min,
  max,
  disabled,
}: {
  label: string;
  value?: string;
  onChange: (value: string) => void;
  invalid?: boolean;
  invalidText: string;
  type?: 'text' | 'number';
  min?: number;
  max?: number;
  disabled?: boolean;
}) {
  const fieldClass = `mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring disabled:bg-slate-100 disabled:text-slate-600 ${invalid ? 'border-rose-300 bg-rose-50' : 'border-line'}`;
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <input className={fieldClass} type={type} min={min} max={max} value={value ?? ''} disabled={disabled} onChange={(event) => onChange(event.target.value)} />
      {invalid ? <span className="mt-1 block text-xs font-medium text-rose-600">{invalidText}</span> : null}
    </label>
  );
}

function Select({
  label,
  value,
  options,
  onChange,
  disabled,
  invalid,
  invalidText,
}: {
  label: string;
  value?: string;
  options: { label: string; value: string }[];
  onChange: (value: string) => void;
  disabled?: boolean;
  invalid?: boolean;
  invalidText: string;
}) {
  const fieldClass = `mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring disabled:bg-slate-100 ${invalid ? 'border-rose-300 bg-rose-50' : 'border-line'}`;
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <select className={fieldClass} value={value ?? ''} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
        <option value="">Chọn điều kiện chuyển tiếp</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>{option.label}</option>
        ))}
      </select>
      {invalid ? <span className="mt-1 block text-xs font-medium text-rose-600">{invalidText}</span> : null}
    </label>
  );
}
