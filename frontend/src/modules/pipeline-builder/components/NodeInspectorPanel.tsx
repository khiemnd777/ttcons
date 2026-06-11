import { useMemo, useState } from 'react';
import { ChevronDown, Pencil, Plus, Save, Trash2, X } from 'lucide-react';
import { Dialog } from '../../../components/Dialog';
import { EnumChip } from '../../../components/EnumChip';
import type { PipelineNode } from '../../../lib/types/domain';
import { enumOptions } from '../../../lib/utils/status';

const roleOptions = ['PM', 'QS', 'Giám sát', 'Kế toán', 'Thủ kho', 'QA/QC', 'Giám đốc', 'Chủ đầu tư', 'Mua hàng', 'Đội thi công'];
const nodeTypes = ['survey', 'design', 'boq', 'budget_approval', 'procurement', 'warehouse_issue', 'construction', 'acceptance', 'rework', 'replanning', 'handover', 'financial_closure', 'custom'];
const stepTypes = ['task', 'checklist', 'approval', 'document_upload', 'photo_upload', 'budget_check', 'material_check', 'acceptance_check', 'accounting_check'];

type SelectOption = string | { label: string; value: string };
type StepDialogMode = 'add' | 'edit' | null;

type NodeInspectorPanelProps = {
  node: PipelineNode;
  nodeForm: Record<string, any>;
  permissionOptions: SelectOption[];
  permissionsLoading: boolean;
  stepDialogMode: StepDialogMode;
  stepForm: { name: string; step_type: string; responsible_role_id: string };
  stepNameInvalid: boolean;
  savePending: boolean;
  addStepPending: boolean;
  updateStepPending: boolean;
  deleteStepPending: boolean;
  onChangeField: (field: string, value: unknown) => void;
  onChangeStepField: (field: 'name' | 'step_type' | 'responsible_role_id', value: string) => void;
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
  permissionOptions,
  permissionsLoading,
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
          <PermissionSelect
            label="Permissions"
            value={Array.isArray(nodeForm.permissions) ? nodeForm.permissions : []}
            options={permissionOptions}
            loading={permissionsLoading}
            onChange={(value) => onChangeField('permissions', value)}
          />

          <div className="border-t border-line pt-4">
            <h3 className="text-sm font-semibold text-ink">Checklist / Step</h3>
            <div className="mt-3 space-y-2">
              {(node.steps ?? []).map((step: Record<string, any>, index: number) => {
                const stepRoles = Array.isArray(step.required_role_ids) ? step.required_role_ids.join(', ') : '';
                return (
                  <div key={step.id ?? `${node.id}-${index}`} className="flex items-start justify-between gap-2 rounded-md border border-line px-3 py-2">
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-ink">{index + 1}. {step.name}</p>
                      <div className="mt-1 flex flex-wrap gap-1">
                        <EnumChip group="step_type" value={step.step_type} />
                        <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{stepRoles || 'Chưa gán role'}</span>
                      </div>
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
                );
              })}
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
          <Select label="Role phụ trách" value={stepForm.responsible_role_id} options={roleOptions} onChange={(value) => onChangeStepField('responsible_role_id', value)} />
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

function PermissionSelect({
  label,
  value,
  options,
  loading,
  onChange,
}: {
  label: string;
  value: string[];
  options: SelectOption[];
  loading: boolean;
  onChange: (value: string[]) => void;
}) {
  const [query, setQuery] = useState('');
  const [open, setOpen] = useState(false);
  const normalizedOptions = useMemo(() => options.map((option) => (typeof option === 'string' ? { label: option, value: option } : option)), [options]);
  const selected = useMemo(() => new Set(value), [value]);
  const availableOptions = useMemo(() => normalizedOptions.filter((option) => !selected.has(option.value)), [normalizedOptions, selected]);
  const labelByValue = useMemo(() => new Map(normalizedOptions.map((option) => [option.value, option.label])), [normalizedOptions]);
  const filteredOptions = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase('vi-VN');
    if (!normalizedQuery) return availableOptions;
    return availableOptions.filter((option) => `${option.value} ${option.label}`.toLocaleLowerCase('vi-VN').includes(normalizedQuery));
  }, [availableOptions, query]);
  const canSearch = !loading && availableOptions.length > 0;
  const choosePermission = (permission: string) => {
    if (!availableOptions.some((option) => option.value === permission)) return;
    onChange([...value, permission]);
    setQuery('');
    setOpen(false);
  };

  return (
    <div className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <div className="mt-1 rounded-md border border-line px-2 py-2">
        <div className="flex min-h-8 flex-wrap gap-1">
          {value.length ? value.map((permission) => (
            <span key={permission} className="inline-flex max-w-full items-center gap-1 rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">
              <span className="truncate">{labelByValue.get(permission) ?? permission}</span>
              <button type="button" title={`Bỏ ${permission}`} className="rounded p-0.5 hover:bg-blue-100" onClick={() => onChange(value.filter((item) => item !== permission))}>
                <X className="h-3 w-3" />
              </button>
            </span>
          )) : <span className="px-1 py-1 text-xs font-medium text-slate-500">Không giới hạn permission riêng</span>}
        </div>
        <div
          className="relative mt-2"
          onBlur={(event) => {
            const nextTarget = event.relatedTarget as Node | null;
            if (!nextTarget || !event.currentTarget.contains(nextTarget)) setOpen(false);
          }}
        >
          <div className="flex overflow-hidden rounded-md border border-line bg-white focus-within:ring-2 focus-within:ring-blue-100">
            <input
              className="min-w-0 flex-1 px-3 py-2 text-sm outline-none disabled:bg-slate-100"
              value={query}
              disabled={!canSearch}
              placeholder={loading ? 'Đang tải permissions...' : availableOptions.length ? 'Gõ để tìm permission' : 'Đã chọn hết permissions'}
              onFocus={() => {
                if (canSearch) setOpen(true);
              }}
              onChange={(event) => {
                setQuery(event.target.value);
                setOpen(true);
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  event.preventDefault();
                  if (filteredOptions[0]) choosePermission(filteredOptions[0].value);
                }
                if (event.key === 'Escape') setOpen(false);
              }}
            />
            <button
              type="button"
              title="Mở danh sách permissions"
              className="grid w-10 place-items-center border-l border-line text-slate-500 hover:bg-slate-50 disabled:bg-slate-100 disabled:text-slate-300"
              disabled={!canSearch}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => setOpen((current) => !current)}
            >
              <ChevronDown className={`h-4 w-4 transition ${open ? 'rotate-180' : ''}`} />
            </button>
          </div>
          {open && canSearch ? (
            <div className="absolute left-0 right-0 top-full z-50 mt-1 max-h-56 overflow-auto rounded-md border border-line bg-white py-1 shadow-panel">
              {filteredOptions.length ? filteredOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  className="block w-full px-3 py-2 text-left text-sm hover:bg-blue-50"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => choosePermission(option.value)}
                >
                  <span className="block font-semibold text-ink">{option.value}</span>
                  <span className="block text-xs text-slate-500">{option.label.replace(`${option.value} - `, '')}</span>
                </button>
              )) : (
                <div className="px-3 py-2 text-sm font-medium text-slate-500">Không tìm thấy permission phù hợp</div>
              )}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
