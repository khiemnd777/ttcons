import { useMemo, useState, type DragEvent } from 'react';
import clsx from 'clsx';
import { ChevronDown, GripVertical, Pencil, Plus, Save, Trash2, X } from 'lucide-react';
import { Dialog } from '../../../components/Dialog';
import { EnumChip } from '../../../components/EnumChip';
import type { PipelineNode, WorkflowFormCatalogItem } from '../../../lib/types/domain';
import { enumOptions } from '../../../lib/utils/status';

const roleOptions = ['PM', 'QS', 'Thiết kế', 'Giám sát', 'Kế toán', 'Thủ kho', 'QA/QC', 'Giám đốc', 'Chủ đầu tư', 'Mua hàng', 'Đội thi công'];
const nodeTypes = ['survey', 'design', 'boq', 'budget_approval', 'procurement', 'warehouse_issue', 'construction', 'acceptance', 'rework', 'replanning', 'handover', 'financial_closure', 'custom'];
const stepTypes = ['task', 'checklist', 'approval', 'document_upload', 'photo_upload', 'budget_check', 'material_check', 'daily_report', 'meeting_minutes', 'acceptance_check', 'handover_check', 'accounting_check'];

type SelectOption = string | { label: string; value: string };
type StepDialogMode = 'add' | 'edit' | null;
type StepDropPosition = 'before' | 'after';
type DropdownMode = 'overlay' | 'inline';
type StepForm = {
  name: string;
  step_type: string;
  required_role_ids: string[];
  form_required: boolean;
  form_version_id: string;
};

type NodeInspectorPanelProps = {
  node: PipelineNode;
  nodeForm: Record<string, any>;
  permissionOptions: SelectOption[];
  permissionsLoading: boolean;
  stepDialogMode: StepDialogMode;
  stepForm: StepForm;
  stepNameInvalid: boolean;
  stepRolesInvalid: boolean;
  stepFormInvalid: boolean;
  savePending: boolean;
  addStepPending: boolean;
  updateStepPending: boolean;
  deleteStepPending: boolean;
  reorderStepPending: boolean;
  workflowForms: WorkflowFormCatalogItem[];
  workflowFormsLoading: boolean;
  workflowFormsError: boolean;
  onChangeField: (field: string, value: unknown) => void;
  onChangeStepField: (field: keyof StepForm, value: string | string[] | boolean) => void;
  onSave: () => void;
  onClose: () => void;
  onOpenAddStep: () => void;
  onOpenEditStep: (step: Record<string, any>) => void;
  onCloseStepDialog: () => void;
  onSubmitStep: () => void;
  onDeleteStep: (stepId: string) => void;
  onReorderSteps: (orderedStepIds: string[]) => void;
};

export function NodeInspectorPanel({
  node,
  nodeForm,
  permissionOptions,
  permissionsLoading,
  stepDialogMode,
  stepForm,
  stepNameInvalid,
  stepRolesInvalid,
  stepFormInvalid,
  savePending,
  addStepPending,
  updateStepPending,
  deleteStepPending,
  reorderStepPending,
  workflowForms,
  workflowFormsLoading,
  workflowFormsError,
  onChangeField,
  onChangeStepField,
  onSave,
  onClose,
  onOpenAddStep,
  onOpenEditStep,
  onCloseStepDialog,
  onSubmitStep,
  onDeleteStep,
  onReorderSteps,
}: NodeInspectorPanelProps) {
  const stepSavePending = addStepPending || updateStepPending;
  const stepActionPending = stepSavePending || deleteStepPending || reorderStepPending;
  const [draggedStepId, setDraggedStepId] = useState('');
  const [dropTargetStepId, setDropTargetStepId] = useState('');
  const [dropPosition, setDropPosition] = useState<StepDropPosition>('before');
  const sortedSteps = useMemo(() => {
    return [...(node.steps ?? [])].sort((a: Record<string, any>, b: Record<string, any>) => {
      const aOrder = Number(a.order_index ?? 0);
      const bOrder = Number(b.order_index ?? 0);
      if (aOrder !== bOrder) return aOrder - bOrder;
      return String(a.created_at ?? a.id ?? '').localeCompare(String(b.created_at ?? b.id ?? ''), 'vi');
    });
  }, [node.steps]);
  const stepIds = useMemo(() => sortedSteps.map((step: Record<string, any>) => (step.id ? String(step.id) : '')).filter(Boolean), [sortedSteps]);
  const canReorderSteps = stepIds.length > 1 && !stepActionPending;
  const formByVersionId = useMemo(() => new Map(workflowForms.map((form) => [String(form.version_id), form])), [workflowForms]);
  const selectedStepForm = stepForm.form_version_id ? formByVersionId.get(stepForm.form_version_id) : undefined;

  const resetStepDragState = () => {
    setDraggedStepId('');
    setDropTargetStepId('');
    setDropPosition('before');
  };

  const stepOrderAfterDrop = (sourceStepId: string, targetStepId: string, position: StepDropPosition) => {
    if (!sourceStepId || sourceStepId === targetStepId) return [];
    if (!stepIds.includes(sourceStepId) || !stepIds.includes(targetStepId)) return [];

    const nextStepIds = stepIds.filter((stepId) => stepId !== sourceStepId);
    const targetIndex = nextStepIds.indexOf(targetStepId);
    if (targetIndex < 0) return [];

    nextStepIds.splice(position === 'after' ? targetIndex + 1 : targetIndex, 0, sourceStepId);
    return sameStringArray(stepIds, nextStepIds) ? [] : nextStepIds;
  };

  const handleStepDragStart = (event: DragEvent<HTMLButtonElement>, stepId: string) => {
    if (!canReorderSteps) {
      event.preventDefault();
      return;
    }
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', stepId);
    setDraggedStepId(stepId);
  };

  const handleStepDragOver = (event: DragEvent<HTMLDivElement>, targetStepId: string) => {
    if (!canReorderSteps || !draggedStepId || draggedStepId === targetStepId) return;

    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
    const targetRect = event.currentTarget.getBoundingClientRect();
    const nextPosition = event.clientY - targetRect.top > targetRect.height / 2 ? 'after' : 'before';
    setDropTargetStepId(targetStepId);
    setDropPosition(nextPosition);
  };

  const handleStepDragLeave = (event: DragEvent<HTMLDivElement>, targetStepId: string) => {
    const nextTarget = event.relatedTarget as Node | null;
    if (nextTarget && event.currentTarget.contains(nextTarget)) return;
    if (dropTargetStepId === targetStepId) setDropTargetStepId('');
  };

  const handleStepDrop = (event: DragEvent<HTMLDivElement>, targetStepId: string) => {
    event.preventDefault();
    const sourceStepId = draggedStepId || event.dataTransfer.getData('text/plain');
    const nextStepIds = stepOrderAfterDrop(sourceStepId, targetStepId, dropPosition);
    resetStepDragState();
    if (nextStepIds.length) onReorderSteps(nextStepIds);
  };

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
            <div className="flex items-center justify-between gap-2">
              <h3 className="text-sm font-semibold text-ink">Checklist / Step</h3>
              {reorderStepPending ? <span className="text-xs font-medium text-blue-600">Đang lưu...</span> : null}
            </div>
            <div className="mt-3 space-y-2">
              {sortedSteps.length ? sortedSteps.map((step: Record<string, any>, index: number) => {
                const stepId = step.id ? String(step.id) : '';
                const stepRoles = Array.isArray(step.required_role_ids) ? step.required_role_ids.join(', ') : '';
                const stepWorkflowForm = step.form_version_id ? formByVersionId.get(String(step.form_version_id)) : undefined;
                const canDragStep = Boolean(stepId) && canReorderSteps;
                const isDropTarget = Boolean(stepId) && dropTargetStepId === stepId && draggedStepId !== stepId;
                return (
                  <div key={step.id ?? `${node.id}-${index}`} className="relative">
                    {isDropTarget && dropPosition === 'before' ? <div className="absolute -top-1 left-2 right-2 z-10 h-0.5 rounded-full bg-blue-500" /> : null}
                    <div
                      className={clsx(
                        'flex items-start justify-between gap-2 rounded-md border border-line px-2 py-2 transition',
                        draggedStepId === stepId && 'border-blue-200 bg-blue-50/50 opacity-60',
                        isDropTarget && 'border-blue-300 bg-blue-50/30',
                      )}
                      onDragOver={(event) => {
                        if (stepId) handleStepDragOver(event, stepId);
                      }}
                      onDragLeave={(event) => {
                        if (stepId) handleStepDragLeave(event, stepId);
                      }}
                      onDrop={(event) => {
                        if (stepId) handleStepDrop(event, stepId);
                      }}
                    >
                      <button
                        type="button"
                        title="Kéo để sắp xếp checklist"
                        aria-label={`Kéo để sắp xếp ${step.name}`}
                        className="mt-0.5 rounded-md p-1 text-slate-300 hover:bg-slate-100 hover:text-slate-600 disabled:cursor-not-allowed disabled:opacity-30 enabled:cursor-grab enabled:active:cursor-grabbing"
                        draggable={canDragStep}
                        disabled={!canDragStep}
                        onClick={(event) => event.preventDefault()}
                        onDragStart={(event) => handleStepDragStart(event, stepId)}
                        onDragEnd={resetStepDragState}
                      >
                        <GripVertical className="h-4 w-4" />
                      </button>
                      <div className="min-w-0 flex-1">
                        <p className="text-sm font-medium text-ink">{index + 1}. {step.name}</p>
                        <div className="mt-1 flex flex-wrap gap-1">
                          <EnumChip group="step_type" value={step.step_type} />
                          <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{stepRoles || 'Chưa gán role'}</span>
                          {stepWorkflowForm ? (
                            <span className="rounded-md bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700">
                              {stepWorkflowForm.template_name} v{stepWorkflowForm.version_no}{step.form_required ? ' · Required' : ' · Optional'}
                            </span>
                          ) : step.form_version_id ? (
                            <span className={clsx('rounded-md px-2 py-0.5 text-xs font-semibold', workflowFormsLoading ? 'bg-slate-100 text-slate-600' : 'bg-amber-50 text-amber-700')}>
                              {workflowFormsLoading ? 'Đang tải workflow form' : workflowFormsError ? 'Không tải được workflow form' : 'Workflow form không còn active'}
                            </span>
                          ) : step.form_required ? (
                            <span className="rounded-md bg-rose-50 px-2 py-0.5 text-xs font-semibold text-rose-700">Thiếu workflow form</span>
                          ) : null}
                        </div>
                      </div>
                      {stepId ? (
                        <div className="flex shrink-0 gap-1">
                          <button title="Sửa checklist" className="rounded-md p-1 text-slate-400 hover:bg-blue-50 hover:text-blue-600" onClick={() => onOpenEditStep(step)} disabled={stepActionPending}>
                            <Pencil className="h-4 w-4" />
                          </button>
                          <button title="Xóa checklist" className="rounded-md p-1 text-slate-300 hover:bg-rose-50 hover:text-rose-600" onClick={() => onDeleteStep(stepId)} disabled={stepActionPending}>
                            <Trash2 className="h-4 w-4" />
                          </button>
                        </div>
                      ) : null}
                    </div>
                    {isDropTarget && dropPosition === 'after' ? <div className="absolute -bottom-1 left-2 right-2 z-10 h-0.5 rounded-full bg-blue-500" /> : null}
                  </div>
                );
              }) : (
                <div className="rounded-md border border-dashed border-line px-3 py-4 text-sm font-medium text-slate-500">
                  Chưa có checklist/step
                </div>
              )}
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
          <MultiSelect
            label="Role phụ trách"
            value={stepForm.required_role_ids}
            options={roleOptions}
            invalid={stepRolesInvalid}
            emptyText="Chưa gán role phụ trách"
            loadingPlaceholder="Đang tải roles..."
            searchPlaceholder="Gõ để tìm role"
            fullPlaceholder="Đã chọn hết roles"
            openButtonTitle="Mở danh sách roles"
            noResultsText="Không tìm thấy role phù hợp"
            invalidText="Chọn ít nhất một role phụ trách"
            removeTitle={(role) => `Bỏ ${role}`}
            dropdownMode="inline"
            onChange={(value) => onChangeStepField('required_role_ids', value)}
          />
          <WorkflowFormSelect
            value={stepForm.form_version_id}
            required={stepForm.form_required}
            invalid={stepFormInvalid}
            forms={workflowForms}
            loading={workflowFormsLoading}
            error={workflowFormsError}
            selectedForm={selectedStepForm}
            onRequiredChange={(value) => onChangeStepField('form_required', value)}
            onChange={(value) => onChangeStepField('form_version_id', value)}
          />
        </div>
      </Dialog>
    </>
  );
}

function sameStringArray(left: string[], right: string[]) {
  return left.length === right.length && left.every((value, index) => value === right[index]);
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
  return (
    <MultiSelect
      label={label}
      value={value}
      options={options}
      loading={loading}
      emptyText="Không giới hạn permission riêng"
      loadingPlaceholder="Đang tải permissions..."
      searchPlaceholder="Gõ để tìm permission"
      fullPlaceholder="Đã chọn hết permissions"
      openButtonTitle="Mở danh sách permissions"
      noResultsText="Không tìm thấy permission phù hợp"
      removeTitle={(permission) => `Bỏ ${permission}`}
      onChange={onChange}
    />
  );
}

function WorkflowFormSelect({
  value,
  required,
  invalid,
  forms,
  loading,
  error,
  selectedForm,
  onRequiredChange,
  onChange,
}: {
  value: string;
  required: boolean;
  invalid: boolean;
  forms: WorkflowFormCatalogItem[];
  loading: boolean;
  error: boolean;
  selectedForm?: WorkflowFormCatalogItem;
  onRequiredChange: (value: boolean) => void;
  onChange: (value: string) => void;
}) {
  const disabled = loading || error || !forms.length;
  const requiredFields = requiredFieldLabels(selectedForm);
  return (
    <section className="rounded-md border border-line bg-slate-50 p-3">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-ink">Workflow Form</h3>
        </div>
        <label className="inline-flex shrink-0 items-center gap-2 text-xs font-semibold text-slate-700">
          <input type="checkbox" className="h-4 w-4 rounded border-line" checked={required} onChange={(event) => onRequiredChange(event.target.checked)} />
          Bắt buộc
        </label>
      </div>
      <label className="mt-3 block">
        <span className="text-xs font-semibold text-slate-600">Form template/version</span>
        <select
          className={clsx(
            'mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring disabled:bg-slate-100',
            invalid ? 'border-rose-300 bg-rose-50' : 'border-line bg-white',
          )}
          value={value ?? ''}
          disabled={disabled}
          onChange={(event) => onChange(event.target.value)}
        >
          <option value="">{workflowFormEmptyLabel({ required, loading, error, hasForms: forms.length > 0 })}</option>
          {forms.map((form) => (
            <option key={form.version_id} value={form.version_id}>
              {formOptionLabel(form)}
            </option>
          ))}
        </select>
      </label>
      {selectedForm ? (
        <div className="mt-2 rounded-md border border-line bg-white px-3 py-2 text-xs text-slate-600">
          <p className="font-semibold text-slate-700">{selectedForm.field_count ?? 0} fields · {requiredFields.length ? `Required: ${requiredFields.join(', ')}` : 'Không có field bắt buộc'}</p>
          <p className="mt-1">{selectedForm.template_description || selectedForm.form_type}</p>
        </div>
      ) : null}
      {invalid ? <p className="mt-2 text-xs font-medium text-rose-600">Chọn workflow form cho step bắt buộc.</p> : null}
      {error ? <p className="mt-2 text-xs font-medium text-rose-600">Không tải được workflow form catalog.</p> : null}
      {!loading && !error && !forms.length ? <p className="mt-2 text-xs font-medium text-amber-700">Chưa có published workflow form.</p> : null}
    </section>
  );
}

function workflowFormEmptyLabel({ required, loading, error, hasForms }: { required: boolean; loading: boolean; error: boolean; hasForms: boolean }) {
  if (loading) return 'Đang tải workflow form...';
  if (error) return 'Không tải được workflow form';
  if (!hasForms) return 'Chưa có published workflow form';
  return required ? 'Chọn workflow form' : 'Không gắn workflow form';
}

function formOptionLabel(form: WorkflowFormCatalogItem) {
  return `${form.template_name} · v${form.version_no} · ${form.form_type}`;
}

function requiredFieldLabels(form?: WorkflowFormCatalogItem) {
  if (!form) return [];
  const labelByKey = new Map((form.fields ?? []).map((field) => [field.field_key, field.label || field.field_key]));
  return (Array.isArray(form.required_field_keys) ? form.required_field_keys : []).map((fieldKey) => labelByKey.get(fieldKey) ?? fieldKey);
}

function MultiSelect({
  label,
  value,
  options,
  loading = false,
  invalid = false,
  emptyText,
  loadingPlaceholder,
  searchPlaceholder,
  fullPlaceholder,
  openButtonTitle,
  noResultsText,
  invalidText,
  removeTitle,
  dropdownMode = 'overlay',
  onChange,
}: {
  label: string;
  value: string[];
  options: SelectOption[];
  loading?: boolean;
  invalid?: boolean;
  emptyText: string;
  loadingPlaceholder: string;
  searchPlaceholder: string;
  fullPlaceholder: string;
  openButtonTitle: string;
  noResultsText: string;
  invalidText?: string;
  removeTitle: (value: string) => string;
  dropdownMode?: DropdownMode;
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
  const chooseValue = (selectedValue: string) => {
    if (!availableOptions.some((option) => option.value === selectedValue)) return;
    onChange([...value, selectedValue]);
    setQuery('');
    setOpen(false);
  };

  return (
    <div className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <div className={clsx('mt-1 rounded-md border px-2 py-2', invalid ? 'border-rose-300 bg-rose-50' : 'border-line')}>
        <div className="flex min-h-8 flex-wrap gap-1">
          {value.length ? value.map((selectedValue) => (
            <span key={selectedValue} className="inline-flex max-w-full items-center gap-1 rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">
              <span className="truncate">{labelByValue.get(selectedValue) ?? selectedValue}</span>
              <button type="button" title={removeTitle(selectedValue)} className="rounded p-0.5 hover:bg-blue-100" onClick={() => onChange(value.filter((item) => item !== selectedValue))}>
                <X className="h-3 w-3" />
              </button>
            </span>
          )) : <span className={clsx('px-1 py-1 text-xs font-medium', invalid ? 'text-rose-700' : 'text-slate-500')}>{emptyText}</span>}
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
              placeholder={loading ? loadingPlaceholder : availableOptions.length ? searchPlaceholder : fullPlaceholder}
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
                  if (filteredOptions[0]) chooseValue(filteredOptions[0].value);
                }
                if (event.key === 'Escape') setOpen(false);
              }}
            />
            <button
              type="button"
              title={openButtonTitle}
              className="grid w-10 place-items-center border-l border-line text-slate-500 hover:bg-slate-50 disabled:bg-slate-100 disabled:text-slate-300"
              disabled={!canSearch}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => setOpen((current) => !current)}
            >
              <ChevronDown className={`h-4 w-4 transition ${open ? 'rotate-180' : ''}`} />
            </button>
          </div>
          {open && canSearch ? (
            <div
              className={clsx(
                'mt-1 overflow-auto rounded-md border border-line bg-white py-1 shadow-panel',
                dropdownMode === 'overlay' ? 'absolute left-0 right-0 top-full z-50 max-h-56' : 'max-h-40',
              )}
            >
              {filteredOptions.length ? filteredOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  className="block w-full px-3 py-2 text-left text-sm hover:bg-blue-50"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => chooseValue(option.value)}
                >
                  <span className="block font-semibold text-ink">{option.value}</span>
                  {optionLabelDescription(option) ? <span className="block text-xs text-slate-500">{optionLabelDescription(option)}</span> : null}
                </button>
              )) : (
                <div className="px-3 py-2 text-sm font-medium text-slate-500">{noResultsText}</div>
              )}
            </div>
          ) : null}
        </div>
      </div>
      {invalid && invalidText ? <span className="mt-1 block text-xs font-medium text-rose-600">{invalidText}</span> : null}
    </div>
  );
}

function optionLabelDescription(option: { label: string; value: string }) {
  if (option.label === option.value) return '';
  return option.label.startsWith(`${option.value} - `) ? option.label.replace(`${option.value} - `, '') : option.label;
}
