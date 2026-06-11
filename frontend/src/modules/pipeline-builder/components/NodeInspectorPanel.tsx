import { Pencil, Plus, Save, Trash2, X } from 'lucide-react';
import { Dialog } from '../../../components/Dialog';
import { EnumChip } from '../../../components/EnumChip';
import type { PipelineNode } from '../../../lib/types/domain';
import { enumOptions } from '../../../lib/utils/status';

const roleOptions = ['PM', 'QS', 'Giám sát', 'Kế toán', 'Thủ kho', 'QA/QC', 'Giám đốc', 'Chủ đầu tư', 'Mua hàng', 'Đội thi công'];
const nodeTypes = ['survey', 'design', 'boq', 'budget_approval', 'procurement', 'warehouse_issue', 'construction', 'acceptance', 'rework', 'replanning', 'handover', 'financial_closure', 'custom'];
const approvalRules = ['PM duyệt', 'PM + QS duyệt', 'PM + Kế toán duyệt', 'PM + QS + Kế toán duyệt', 'PM + Kế toán + Giám đốc duyệt', 'Chủ đầu tư xác nhận', 'QA/QC + Chủ đầu tư xác nhận'];
const failPathOptions = ['none', 'previous', 'rework', 'replanning'];
const stepTypes = ['task', 'checklist', 'approval', 'document_upload', 'photo_upload', 'budget_check', 'material_check', 'acceptance_check', 'accounting_check'];

type SelectOption = string | { label: string; value: string };
type StepDialogMode = 'add' | 'edit' | null;

type NodeInspectorPanelProps = {
  node: PipelineNode;
  nodeForm: Record<string, any>;
  stepDialogMode: StepDialogMode;
  stepForm: { name: string; step_type: string };
  stepNameInvalid: boolean;
  savePending: boolean;
  addStepPending: boolean;
  updateStepPending: boolean;
  deleteStepPending: boolean;
  onChangeField: (field: string, value: string | boolean) => void;
  onChangeStepField: (field: 'name' | 'step_type', value: string) => void;
  onSave: () => void;
  onClose: () => void;
  onOpenAddStep: () => void;
  onOpenEditStep: (step: Record<string, any>) => void;
  onCloseStepDialog: () => void;
  onSubmitStep: () => void;
  onDeleteStep: (stepId: string) => void;
};

export function NodeInspectorPanel({
  node,
  nodeForm,
  stepDialogMode,
  stepForm,
  stepNameInvalid,
  savePending,
  addStepPending,
  updateStepPending,
  deleteStepPending,
  onChangeField,
  onChangeStepField,
  onSave,
  onClose,
  onOpenAddStep,
  onOpenEditStep,
  onCloseStepDialog,
  onSubmitStep,
  onDeleteStep,
}: NodeInspectorPanelProps) {
  const stepSavePending = addStepPending || updateStepPending;

  return (
    <>
      <aside className="pipeline-details-panel absolute bottom-4 right-4 top-20 z-30 w-[380px] overflow-auto rounded-lg border border-line bg-white p-4 shadow-panel">
        <div className="mb-4 flex items-center justify-between gap-3">
          <div>
            <h2 className="text-base font-semibold text-ink">Cấu hình Node</h2>
            <p className="mt-1 text-xs text-slate-500">{node.name}</p>
          </div>
          <div className="flex gap-2">
            <button title="Lưu node" className="rounded-md bg-blue-600 p-2 text-white hover:bg-blue-700 disabled:opacity-50" disabled={savePending} onClick={onSave}>
              <Save className="h-4 w-4" />
            </button>
            <button title="Đóng cấu hình" className="rounded-md border border-line p-2 text-slate-500 hover:bg-slate-100" onClick={onClose}>
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>
        <div className="space-y-3">
          <Field label="Tên node" value={nodeForm.name} onChange={(value) => onChangeField('name', value)} />
          <Field label="Mô tả" value={nodeForm.description} onChange={(value) => onChangeField('description', value)} textarea />
          <Select label="Node type" value={nodeForm.type} options={enumOptions('node_type', nodeTypes)} onChange={(value) => onChangeField('type', value)} />
          <Select label="Role phụ trách" value={nodeForm.responsible_role_ids} options={roleOptions} onChange={(value) => onChangeField('responsible_role_ids', value)} />
          <Select label="Approval rule" value={nodeForm.approval_rule} options={approvalRules} onChange={(value) => onChangeField('approval_rule', value)} />
          <Select label="Fail path" value={nodeForm.fail_path} options={enumOptions('fail_path', failPathOptions)} onChange={(value) => onChangeField('fail_path', value)} />
          <Field label="Required permission" value={nodeForm.required_permission} onChange={(value) => onChangeField('required_permission', value)} />
          <Field label="Điều kiện hoàn tất" value={nodeForm.exit_conditions_text} onChange={(value) => onChangeField('exit_conditions_text', value)} textarea />
          <div className="grid gap-2 sm:grid-cols-2">
            <Checkbox label="Rework" checked={Boolean(nodeForm.rework_enabled)} onChange={(value) => onChangeField('rework_enabled', value)} />
            <Checkbox label="Replanning" checked={Boolean(nodeForm.replanning_enabled)} onChange={(value) => onChangeField('replanning_enabled', value)} />
          </div>

          <div className="border-t border-line pt-4">
            <h3 className="text-sm font-semibold text-ink">Checklist / Step</h3>
            <div className="mt-3 space-y-2">
              {(node.steps ?? []).map((step: Record<string, any>, index: number) => (
                <div key={step.id ?? `${node.id}-${index}`} className="flex items-start justify-between gap-2 rounded-md border border-line px-3 py-2">
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-ink">{index + 1}. {step.name}</p>
                    <div className="mt-1"><EnumChip group="step_type" value={step.step_type} /></div>
                  </div>
                  {step.id ? (
                    <div className="flex shrink-0 gap-1">
                      <button title="Sửa checklist" className="rounded-md p-1 text-slate-400 hover:bg-blue-50 hover:text-blue-600" onClick={() => onOpenEditStep(step)} disabled={stepSavePending || deleteStepPending}>
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button title="Xóa checklist" className="rounded-md p-1 text-slate-300 hover:bg-rose-50 hover:text-rose-600" onClick={() => onDeleteStep(String(step.id))} disabled={deleteStepPending || stepSavePending}>
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
            <button className="mt-3 flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:opacity-50" onClick={onOpenAddStep} disabled={stepSavePending}>
              <Plus className="h-4 w-4" />
              Thêm
            </button>
          </div>
        </div>
      </aside>
      <Dialog
        open={Boolean(stepDialogMode)}
        title={stepDialogMode === 'edit' ? 'Sửa Checklist / Step' : 'Thêm Checklist / Step'}
        onClose={onCloseStepDialog}
        size="sm"
        footer={
          <div className="flex justify-end gap-2">
            <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-white disabled:opacity-50" onClick={onCloseStepDialog} disabled={stepSavePending}>
              Hủy
            </button>
            <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={onSubmitStep} disabled={stepSavePending}>
              {stepDialogMode === 'edit' ? 'Lưu' : 'Thêm'}
            </button>
          </div>
        }
      >
        <div className="space-y-4">
          <Field label="Tên" value={stepForm.name} onChange={(value) => onChangeStepField('name', value)} invalid={stepNameInvalid} />
          <Select label="Loại" value={stepForm.step_type} options={enumOptions('step_type', stepTypes)} onChange={(value) => onChangeStepField('step_type', value)} />
        </div>
      </Dialog>
    </>
  );
}

function Field({ label, value, onChange, textarea, invalid }: { label: string; value?: string; onChange: (value: string) => void; textarea?: boolean; invalid?: boolean }) {
  const fieldClass = `mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring ${invalid ? 'border-rose-300 bg-rose-50' : 'border-line'}`;
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      {textarea ? (
        <textarea className={`${fieldClass} min-h-20`} value={value ?? ''} onChange={(event) => onChange(event.target.value)} />
      ) : (
        <input className={fieldClass} value={value ?? ''} onChange={(event) => onChange(event.target.value)} />
      )}
      {invalid ? <span className="mt-1 block text-xs font-medium text-rose-600">Tên checklist là bắt buộc</span> : null}
    </label>
  );
}

function Select({ label, value, options, onChange }: { label: string; value?: string; options: SelectOption[]; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" value={value ?? ''} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => {
          const normalized = typeof option === 'string' ? { label: option, value: option } : option;
          return <option key={normalized.value} value={normalized.value}>{normalized.label}</option>;
        })}
      </select>
    </label>
  );
}

function Checkbox({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700">
      <input type="checkbox" className="h-4 w-4 rounded border-line text-blue-600 focus:ring-blue-500" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      {label}
    </label>
  );
}
