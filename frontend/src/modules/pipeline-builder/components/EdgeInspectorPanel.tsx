import { Save, X } from 'lucide-react';
import type { PipelineEdgeConditionDefinition, PipelineNode, PipelineTemplateEdge } from '../../../lib/types/domain';

type EdgeForm = {
  label: string;
  condition_type: string;
  priority: string;
};

type EdgeInspectorPanelProps = {
  edge: PipelineTemplateEdge;
  fromNode?: PipelineNode;
  toNode?: PipelineNode;
  conditions: PipelineEdgeConditionDefinition[];
  conditionsLoading: boolean;
  conditionLoadError: boolean;
  edgeForm: EdgeForm;
  conditionInvalid: boolean;
  priorityInvalid: boolean;
  savePending: boolean;
  onChangeField: (field: keyof EdgeForm, value: string) => void;
  onSave: () => void;
  onClose: () => void;
};

export function EdgeInspectorPanel({
  edge,
  fromNode,
  toNode,
  conditions,
  conditionsLoading,
  conditionLoadError,
  edgeForm,
  conditionInvalid,
  priorityInvalid,
  savePending,
  onChangeField,
  onSave,
  onClose,
}: EdgeInspectorPanelProps) {
  const selectedCondition = conditions.find((condition) => condition.type === edgeForm.condition_type);
  const sourceName = fromNode?.name ?? edge.from_node_id;
  const targetName = toNode?.name ?? edge.to_node_id;

  return (
    <aside className="pipeline-details-panel absolute bottom-4 right-4 top-20 z-30 w-[380px] overflow-auto rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-ink">Cấu hình Connector</h2>
          <p className="mt-1 truncate text-xs text-slate-500">{sourceName} → {targetName}</p>
        </div>
        <div className="flex gap-2">
          <button title="Lưu connector" className="rounded-md bg-blue-600 p-2 text-white hover:bg-blue-700 disabled:opacity-50" disabled={savePending || conditionsLoading || conditionLoadError} onClick={onSave}>
            <Save className="h-4 w-4" />
          </button>
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
          onChange={(value) => onChangeField('label', value)}
        />
        <Select
          label="Điều kiện"
          value={edgeForm.condition_type}
          options={conditions.map((condition) => ({ label: condition.label, value: condition.type }))}
          disabled={conditionsLoading || conditionLoadError}
          invalid={conditionInvalid}
          invalidText={conditionLoadError ? 'Không tải được danh sách điều kiện' : 'Điều kiện không hợp lệ'}
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
          onChange={(value) => onChangeField('priority', value)}
        />

        <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-semibold uppercase text-slate-500">Đánh giá</span>
            <span className="rounded bg-white px-2 py-1 text-xs font-semibold text-slate-600">{selectedCondition?.category ?? 'condition'}</span>
          </div>
          <p className="mt-2 text-sm leading-5 text-slate-700">
            {selectedCondition?.description ?? (conditionsLoading ? 'Đang tải danh sách điều kiện...' : 'Chọn điều kiện hợp lệ cho connector.')}
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
}: {
  label: string;
  value?: string;
  onChange: (value: string) => void;
  invalid?: boolean;
  invalidText: string;
  type?: 'text' | 'number';
  min?: number;
  max?: number;
}) {
  const fieldClass = `mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring ${invalid ? 'border-rose-300 bg-rose-50' : 'border-line'}`;
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <input className={fieldClass} type={type} min={min} max={max} value={value ?? ''} onChange={(event) => onChange(event.target.value)} />
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
        <option value="">Chọn condition</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>{option.label}</option>
        ))}
      </select>
      {invalid ? <span className="mt-1 block text-xs font-medium text-rose-600">{invalidText}</span> : null}
    </label>
  );
}
