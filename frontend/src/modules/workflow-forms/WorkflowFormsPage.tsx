import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useBlocker } from 'react-router-dom';
import {
  AlertTriangle,
  Archive,
  ChevronDown,
  ChevronRight,
  ClipboardList,
  FilePlus2,
  FileText,
  GitBranch,
  Layers3,
  MoreHorizontal,
  Plus,
  RotateCcw,
  Save,
  Search,
  Settings2,
  Trash2,
  UploadCloud,
  type LucideIcon,
} from 'lucide-react';
import clsx from 'clsx';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { Dialog } from '../../components/Dialog';
import { CurrencyInput } from '../../components/CurrencyInput';
import { useToast } from '../../components/ToastProvider';
import { resolveCurrencyFractionDigits } from '../../lib/utils/currencyInput';
import type {
  AnyRecord,
  WorkflowFormField,
  WorkflowFormManagedTemplate,
  WorkflowFormManagedVersion,
  WorkflowFormTemplateLibraryItem,
} from '../../lib/types/domain';

type TemplateForm = {
  code: string;
  name: string;
  description: string;
  form_type: string;
  status: string;
};

type DraftField = {
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

type FieldErrors = Record<string, string>;

type ConfirmAction = 'publish' | 'archive-template' | 'archive-version' | null;

type PendingNavigation =
  | { kind: 'template'; id: string }
  | { kind: 'version'; id: string }
  | { kind: 'create-template' }
  | null;

const formTypeOptions = [
  ['generic_submission', 'Biểu mẫu gửi'],
  ['checklist', 'Checklist'],
  ['daily_report', 'Báo cáo ngày'],
  ['meeting_minutes', 'Biên bản họp'],
];

const fieldTypeOptions = [
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

const blankTemplateForm: TemplateForm = {
  code: '',
  name: '',
  description: '',
  form_type: 'generic_submission',
  status: 'active',
};

export function WorkflowFormsPage() {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [selectedTemplateId, setSelectedTemplateId] = useState('');
  const [selectedVersionId, setSelectedVersionId] = useState('');
  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState('all');
  const [statusFilter, setStatusFilter] = useState('active');
  const [templateDialog, setTemplateDialog] = useState<'create' | 'edit' | null>(null);
  const [templateForm, setTemplateForm] = useState<TemplateForm>(blankTemplateForm);
  const [templateSubmitAttempted, setTemplateSubmitAttempted] = useState(false);
  const [draftFields, setDraftFields] = useState<DraftField[]>([]);
  const [draftBaseline, setDraftBaseline] = useState('[]');
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [confirmAction, setConfirmAction] = useState<ConfirmAction>(null);
  const [pendingNavigation, setPendingNavigation] = useState<PendingNavigation>(null);

  const library = useQuery({
    queryKey: ['workflow-form-template-library'],
    queryFn: () => get<WorkflowFormTemplateLibraryItem[]>('/workflow-form-template-library'),
  });
  const detail = useQuery({
    queryKey: ['workflow-form-template', selectedTemplateId],
    queryFn: () => get<WorkflowFormManagedTemplate>(`/workflow-form-templates/${selectedTemplateId}/manage`),
    enabled: Boolean(selectedTemplateId),
  });

  const templates = library.data ?? [];
  const filteredTemplates = useMemo(() => templates.filter((template) => templateMatches(template, search, typeFilter, statusFilter)), [search, statusFilter, templates, typeFilter]);
  const versions = useMemo(() => [...(detail.data?.versions ?? [])].sort((a, b) => Number(b.version_no ?? 0) - Number(a.version_no ?? 0)), [detail.data?.versions]);
  const selectedVersion = versions.find((version) => version.version_id === selectedVersionId) ?? versions[0];
  const draftSelected = selectedVersion?.version_status === 'draft';
  const selectedVersionFingerprint = useMemo(() => draftFieldsFingerprint((selectedVersion?.fields ?? []).map(fieldToDraft)), [selectedVersion?.fields]);
  const draftDirty = Boolean(draftSelected) && draftFieldsFingerprint(draftFields) !== draftBaseline;
  const routeBlocker = useBlocker(draftDirty);

  useEffect(() => {
    if (!selectedTemplateId && filteredTemplates[0]) {
      setSelectedTemplateId(filteredTemplates[0].template_id);
    }
  }, [filteredTemplates, selectedTemplateId]);

  useEffect(() => {
    if (!versions.length) {
      if (selectedVersionId) setSelectedVersionId('');
      return;
    }
    if (!versions.some((version) => version.version_id === selectedVersionId)) {
      setSelectedVersionId(versions[0].version_id);
    }
  }, [selectedVersionId, versions]);

  useEffect(() => {
    const nextFields = (selectedVersion?.fields ?? []).map(fieldToDraft);
    setDraftFields(nextFields);
    setDraftBaseline(draftFieldsFingerprint(nextFields));
    setFieldErrors({});
  }, [selectedVersion?.version_id, selectedVersionFingerprint]);

  useEffect(() => {
    if (!draftDirty) return undefined;
    const warnBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', warnBeforeUnload);
    return () => window.removeEventListener('beforeunload', warnBeforeUnload);
  }, [draftDirty]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['workflow-form-template-library'] });
    if (selectedTemplateId) queryClient.invalidateQueries({ queryKey: ['workflow-form-template', selectedTemplateId] });
    queryClient.invalidateQueries({ queryKey: ['workflow-form-catalog'] });
  };

  const createTemplate = useMutation({
    mutationFn: (payload: TemplateForm) => post<WorkflowFormManagedTemplate>('/workflow-form-templates', payload),
    onSuccess: (created) => {
      closeTemplateDialog();
      setSelectedTemplateId(created.template_id);
      invalidate();
      toast.success('Đã tạo biểu mẫu workflow');
    },
    onError: (error) => toast.error('Không tạo được biểu mẫu workflow', error),
  });

  const updateTemplate = useMutation({
    mutationFn: (payload: TemplateForm) => put<WorkflowFormManagedTemplate>(`/workflow-form-templates/${selectedTemplateId}`, payload),
    onSuccess: () => {
      closeTemplateDialog();
      invalidate();
      toast.success('Đã cập nhật thông tin biểu mẫu');
    },
    onError: (error) => toast.error('Không cập nhật được thông tin biểu mẫu', error),
  });

  const archiveTemplate = useMutation({
    mutationFn: () => post<WorkflowFormManagedTemplate>(`/workflow-form-templates/${selectedTemplateId}/archive`, {}),
    onSuccess: () => {
      setConfirmAction(null);
      invalidate();
      toast.success('Đã lưu trữ biểu mẫu workflow');
    },
    onError: (error) => toast.error('Không lưu trữ được biểu mẫu workflow', error),
  });

  const createVersion = useMutation({
    mutationFn: (sourceVersionId?: string) => post<WorkflowFormManagedVersion>(`/workflow-form-templates/${selectedTemplateId}/versions`, { source_version_id: sourceVersionId ?? '' }),
    onSuccess: (version) => {
      setSelectedVersionId(version.version_id);
      invalidate();
      toast.success('Đã tạo phiên bản nháp');
    },
    onError: (error) => toast.error('Không tạo được phiên bản nháp', error),
  });

  const saveVersion = useMutation({
    mutationFn: () => put<WorkflowFormManagedVersion>(`/workflow-form-versions/${selectedVersion?.version_id}`, { fields: draftFieldsPayload(draftFields) }),
    onSuccess: () => {
      setDraftBaseline(draftFieldsFingerprint(draftFields));
      invalidate();
      toast.success('Đã lưu thay đổi bản nháp');
    },
    onError: (error) => toast.error('Không lưu được bản nháp', error),
  });

  const publishVersion = useMutation({
    mutationFn: async () => {
      await put<WorkflowFormManagedVersion>(`/workflow-form-versions/${selectedVersion?.version_id}`, { fields: draftFieldsPayload(draftFields) });
      return post<WorkflowFormManagedVersion>(`/workflow-form-versions/${selectedVersion?.version_id}/publish`, {});
    },
    onSuccess: () => {
      setConfirmAction(null);
      setDraftBaseline(draftFieldsFingerprint(draftFields));
      invalidate();
      toast.success('Đã phát hành phiên bản biểu mẫu');
    },
    onError: (error) => toast.error('Không phát hành được phiên bản biểu mẫu', error),
  });

  const archiveVersion = useMutation({
    mutationFn: () => post<WorkflowFormManagedVersion>(`/workflow-form-versions/${selectedVersion?.version_id}/archive`, {}),
    onSuccess: () => {
      setConfirmAction(null);
      invalidate();
      toast.success('Đã lưu trữ phiên bản biểu mẫu');
    },
    onError: (error) => toast.error('Không lưu trữ được phiên bản biểu mẫu', error),
  });

  function openCreateTemplate() {
    if (draftDirty) {
      setPendingNavigation({ kind: 'create-template' });
      return;
    }
    showCreateTemplateDialog();
  }

  function showCreateTemplateDialog() {
    setTemplateSubmitAttempted(false);
    setTemplateForm(blankTemplateForm);
    setTemplateDialog('create');
  }

  function openEditTemplate() {
    if (!detail.data) return;
    setTemplateSubmitAttempted(false);
    setTemplateForm({
      code: detail.data.template_code,
      name: detail.data.template_name,
      description: detail.data.template_description ?? '',
      form_type: detail.data.form_type,
      status: detail.data.template_status ?? 'active',
    });
    setTemplateDialog('edit');
  }

  function closeTemplateDialog() {
    setTemplateDialog(null);
    setTemplateSubmitAttempted(false);
  }

  function submitTemplateForm() {
    setTemplateSubmitAttempted(true);
    const missing = templateMissingFields(templateForm, templateDialog === 'create');
    if (missing.length) {
      toast.warn({ title: 'Thiếu dữ liệu biểu mẫu', description: missing.join(', ') });
      return;
    }
    if (templateDialog === 'create') createTemplate.mutate(templateForm);
    else updateTemplate.mutate(templateForm);
  }

  function validateCurrentDraft() {
    const errors = validateDraftFields(draftFields);
    setFieldErrors(errors);
    if (Object.keys(errors).length) {
      toast.warn({ title: 'Cấu trúc biểu mẫu chưa hợp lệ', description: Object.values(errors).slice(0, 4).join(', ') });
      return false;
    }
    return true;
  }

  function saveDraft() {
    if (validateCurrentDraft()) saveVersion.mutate();
  }

  function requestPublish() {
    if (validateCurrentDraft()) setConfirmAction('publish');
  }

  function requestTemplateSelection(id: string) {
    if (!id || id === selectedTemplateId) return;
    if (draftDirty) {
      setPendingNavigation({ kind: 'template', id });
      return;
    }
    setSelectedTemplateId(id);
  }

  function requestVersionSelection(id: string) {
    if (!id || id === selectedVersion?.version_id) return;
    if (draftDirty) {
      setPendingNavigation({ kind: 'version', id });
      return;
    }
    setSelectedVersionId(id);
  }

  function discardDraftAndContinue() {
    const navigation = pendingNavigation;
    setPendingNavigation(null);
    setDraftFields((selectedVersion?.fields ?? []).map(fieldToDraft));
    setFieldErrors({});
    if (navigation?.kind === 'template') setSelectedTemplateId(navigation.id);
    if (navigation?.kind === 'version') setSelectedVersionId(navigation.id);
    if (navigation?.kind === 'create-template') showCreateTemplateDialog();
    if (!navigation && routeBlocker.state === 'blocked') routeBlocker.proceed();
  }

  function closeUnsavedChangesDialog() {
    setPendingNavigation(null);
    if (routeBlocker.state === 'blocked') routeBlocker.reset();
  }

  return (
    <>
      <PageHeader
        title="Workflow Forms"
        description="Tìm, cấu hình và phát hành biểu mẫu dùng trong các bước workflow."
        actions={
          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={openCreateTemplate}>
            <Plus className="h-4 w-4" />
            Tạo biểu mẫu
          </button>
        }
      />

      {library.isLoading ? <WorkflowFormsLoading /> : null}
      {library.error ? (
        <ErrorPanel title="Không tải được thư viện biểu mẫu" onRetry={() => library.refetch()} />
      ) : null}

      {!library.isLoading && !library.error ? (
        <div className="grid items-start gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
          <TemplateLibraryPanel
            templates={filteredTemplates}
            totalCount={templates.length}
            selectedTemplateId={selectedTemplateId}
            search={search}
            typeFilter={typeFilter}
            statusFilter={statusFilter}
            onSearchChange={setSearch}
            onTypeFilterChange={setTypeFilter}
            onStatusFilterChange={setStatusFilter}
            onSelect={requestTemplateSelection}
            onClearFilters={() => {
              setSearch('');
              setTypeFilter('all');
              setStatusFilter('active');
            }}
          />

          <main className="min-w-0">
            {!selectedTemplateId ? <EmptyState title="Chọn một biểu mẫu trong thư viện hoặc tạo biểu mẫu mới." /> : null}
            {detail.isLoading ? <WorkspaceLoading /> : null}
            {detail.error ? (
              <ErrorPanel title="Không tải được biểu mẫu đã chọn" onRetry={() => detail.refetch()} />
            ) : null}
            {detail.data ? (
              <TemplateWorkspace
                template={detail.data}
                versions={versions}
                selectedVersion={selectedVersion}
                draftFields={draftFields}
                fieldErrors={fieldErrors}
                draftDirty={draftDirty}
                saving={saveVersion.isPending || publishVersion.isPending}
                creatingVersion={createVersion.isPending}
                onEditTemplate={openEditTemplate}
                onArchiveTemplate={() => setConfirmAction('archive-template')}
                onCreateVersion={(sourceVersionId) => createVersion.mutate(sourceVersionId)}
                onSelectVersion={requestVersionSelection}
                onArchiveVersion={() => setConfirmAction('archive-version')}
                onDraftFieldsChange={(fields) => {
                  setDraftFields(fields);
                  setFieldErrors({});
                }}
                onSaveDraft={saveDraft}
                onPublishDraft={requestPublish}
              />
            ) : null}
          </main>
        </div>
      ) : null}

      <TemplateDialog
        mode={templateDialog}
        form={templateForm}
        originalStatus={templateDialog === 'edit' ? detail.data?.template_status : undefined}
        submitAttempted={templateSubmitAttempted}
        submitting={createTemplate.isPending || updateTemplate.isPending}
        onChange={setTemplateForm}
        onClose={closeTemplateDialog}
        onSubmit={submitTemplateForm}
      />
      <ConfirmationDialog
        action={confirmAction}
        template={detail.data}
        version={selectedVersion}
        fieldCount={draftFields.length}
        pending={archiveTemplate.isPending || archiveVersion.isPending || publishVersion.isPending}
        onClose={() => setConfirmAction(null)}
        onConfirm={() => {
          if (confirmAction === 'publish') publishVersion.mutate();
          if (confirmAction === 'archive-template') archiveTemplate.mutate();
          if (confirmAction === 'archive-version') archiveVersion.mutate();
        }}
      />
      <UnsavedChangesDialog
        open={Boolean(pendingNavigation) || routeBlocker.state === 'blocked'}
        onClose={closeUnsavedChangesDialog}
        onDiscard={discardDraftAndContinue}
      />
    </>
  );
}

function TemplateLibraryPanel({
  templates,
  totalCount,
  selectedTemplateId,
  search,
  typeFilter,
  statusFilter,
  onSearchChange,
  onTypeFilterChange,
  onStatusFilterChange,
  onSelect,
  onClearFilters,
}: {
  templates: WorkflowFormTemplateLibraryItem[];
  totalCount: number;
  selectedTemplateId: string;
  search: string;
  typeFilter: string;
  statusFilter: string;
  onSearchChange: (value: string) => void;
  onTypeFilterChange: (value: string) => void;
  onStatusFilterChange: (value: string) => void;
  onSelect: (id: string) => void;
  onClearFilters: () => void;
}) {
  const selectedVisible = templates.some((template) => template.template_id === selectedTemplateId);
  return (
    <aside className="overflow-hidden rounded-lg border border-line bg-white shadow-panel xl:sticky xl:top-4">
      <div className="border-b border-line p-4">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <ClipboardList className="h-5 w-5 text-blue-700" />
            <h2 className="font-semibold text-ink">Thư viện biểu mẫu</h2>
          </div>
          <span className="rounded-full bg-slate-100 px-2.5 py-1 text-xs font-semibold text-slate-600">{templates.length}/{totalCount}</span>
        </div>
        <div className="space-y-2">
          <label className="relative block">
            <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-slate-400" />
            <input
              className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="Tìm tên hoặc mã biểu mẫu"
              value={search}
              onChange={(event) => onSearchChange(event.target.value)}
            />
          </label>
          <div className="grid grid-cols-2 gap-2">
            <select className="min-w-0 rounded-md border border-line px-2.5 py-2 text-sm" value={typeFilter} onChange={(event) => onTypeFilterChange(event.target.value)}>
              <option value="all">Tất cả loại</option>
              {formTypeOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
            <select className="min-w-0 rounded-md border border-line px-2.5 py-2 text-sm" value={statusFilter} onChange={(event) => onStatusFilterChange(event.target.value)}>
              <option value="all">Mọi trạng thái</option>
              <option value="active">Đang dùng</option>
              <option value="archived">Đã lưu trữ</option>
            </select>
          </div>
          <select
            className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm xl:hidden"
            value={selectedVisible ? selectedTemplateId : ''}
            onChange={(event) => onSelect(event.target.value)}
          >
            <option value="">Chọn biểu mẫu…</option>
            {templates.map((template) => <option key={template.template_id} value={template.template_id}>{template.template_name}</option>)}
          </select>
        </div>
      </div>

      <div className="hidden max-h-[calc(100vh-14rem)] space-y-1 overflow-y-auto p-2 xl:block">
        {templates.length ? templates.map((template) => (
          <button
            key={template.template_id}
            className={clsx(
              'block w-full rounded-md border px-3 py-2.5 text-left transition-colors',
              selectedTemplateId === template.template_id
                ? 'border-blue-300 bg-blue-50 ring-1 ring-blue-100'
                : 'border-transparent hover:border-line hover:bg-slate-50',
            )}
            onClick={() => onSelect(template.template_id)}
          >
            <div className="flex items-start justify-between gap-2">
              <p className="min-w-0 truncate text-sm font-semibold text-ink">{template.template_name}</p>
              <StatusBadge status={template.template_status} className="shrink-0" />
            </div>
            <p className="mt-0.5 truncate font-mono text-[11px] text-slate-500">{template.template_code}</p>
            <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-slate-600">
              <span className="font-semibold text-blue-700">{formTypeLabel(template.form_type)}</span>
              <span>v{template.latest_version_no ?? '—'}</span>
              <span>{template.field_count ?? 0} trường</span>
              <span>{template.step_usage_count ?? 0} bước</span>
              {(template.draft_version_count ?? 0) > 0 ? <span className="rounded bg-amber-50 px-1.5 py-0.5 font-semibold text-amber-700">Có bản nháp</span> : null}
            </div>
          </button>
        )) : (
          <div className="p-2">
            <EmptyState
              title="Không có biểu mẫu khớp bộ lọc."
              action={<button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClearFilters}><RotateCcw className="h-4 w-4" />Xóa bộ lọc</button>}
            />
          </div>
        )}
      </div>
      {!templates.length ? (
        <div className="p-3 xl:hidden">
          <button className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700" onClick={onClearFilters}>
            <RotateCcw className="h-4 w-4" /> Xóa bộ lọc
          </button>
        </div>
      ) : null}
    </aside>
  );
}

function TemplateWorkspace({
  template,
  versions,
  selectedVersion,
  draftFields,
  fieldErrors,
  draftDirty,
  saving,
  creatingVersion,
  onEditTemplate,
  onArchiveTemplate,
  onCreateVersion,
  onSelectVersion,
  onArchiveVersion,
  onDraftFieldsChange,
  onSaveDraft,
  onPublishDraft,
}: {
  template: WorkflowFormManagedTemplate;
  versions: WorkflowFormManagedVersion[];
  selectedVersion?: WorkflowFormManagedVersion;
  draftFields: DraftField[];
  fieldErrors: FieldErrors;
  draftDirty: boolean;
  saving: boolean;
  creatingVersion: boolean;
  onEditTemplate: () => void;
  onArchiveTemplate: () => void;
  onCreateVersion: (sourceVersionId?: string) => void;
  onSelectVersion: (id: string) => void;
  onArchiveVersion: () => void;
  onDraftFieldsChange: (fields: DraftField[]) => void;
  onSaveDraft: () => void;
  onPublishDraft: () => void;
}) {
  const draftSelected = selectedVersion?.version_status === 'draft';
  const templateInUse = (template.step_usage_count ?? 0) > 0;
  const versionInUse = (selectedVersion?.step_usage_count ?? 0) > 0;
  const visibleFieldCount = draftSelected ? draftFields.length : (selectedVersion?.field_count ?? selectedVersion?.fields?.length ?? 0);
  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
      <div className="p-4 md:p-5">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-xl font-semibold text-ink">{template.template_name}</h1>
              <StatusBadge status={template.template_status} />
              <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{formTypeLabel(template.form_type)}</span>
            </div>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-600">{template.template_description || 'Chưa có mô tả cho biểu mẫu này.'}</p>
            <p className="mt-2 truncate font-mono text-xs text-slate-500">{template.template_code}</p>
          </div>
          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onEditTemplate}>Sửa thông tin</button>
            {!draftSelected ? (
              <button
                className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60"
                onClick={() => onCreateVersion(selectedVersion?.version_id)}
                disabled={creatingVersion}
              >
                <FilePlus2 className="h-4 w-4" />
                {selectedVersion ? `Tạo bản nháp từ v${selectedVersion.version_no}` : 'Tạo phiên bản đầu tiên'}
              </button>
            ) : null}
            <details className="relative">
              <summary className="grid h-9 w-9 cursor-pointer list-none place-items-center rounded-md border border-line text-slate-600 hover:bg-slate-50" aria-label="Thao tác khác">
                <MoreHorizontal className="h-4 w-4" />
              </summary>
              <div className="absolute right-0 z-20 mt-1 w-52 rounded-md border border-line bg-white p-1 shadow-lg">
                <button
                  className="flex w-full items-center gap-2 rounded px-3 py-2 text-left text-sm font-medium text-rose-700 hover:bg-rose-50 disabled:cursor-not-allowed disabled:text-slate-400"
                  onClick={onArchiveTemplate}
                  disabled={templateInUse}
                  title={templateInUse ? 'Biểu mẫu đang được pipeline step sử dụng' : 'Lưu trữ biểu mẫu'}
                >
                  <Archive className="h-4 w-4" /> Lưu trữ biểu mẫu
                </button>
              </div>
            </details>
          </div>
        </div>
      </div>

      <div className="border-t border-line bg-slate-50/70 p-4 md:p-5">
        <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
          <label className="block w-full max-w-sm">
            <span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Phiên bản</span>
            <select
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-ink outline-none focus:ring-2 focus:ring-blue-500"
              value={selectedVersion?.version_id ?? ''}
              onChange={(event) => onSelectVersion(event.target.value)}
              disabled={!versions.length}
            >
              {!versions.length ? <option value="">Chưa có phiên bản</option> : null}
              {versions.map((version) => (
                <option key={version.version_id} value={version.version_id}>
                  v{version.version_no} · {versionStatusLabel(version.version_status)}{version.latest_published ? ' · Mới nhất' : ''}
                </option>
              ))}
            </select>
          </label>
          {selectedVersion ? (
            <div className="flex items-center gap-2">
              <StatusBadge status={selectedVersion.version_status} />
              {selectedVersion.latest_published ? <span className="rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">Bản phát hành mới nhất</span> : null}
              <details className="relative">
                <summary className="grid h-9 w-9 cursor-pointer list-none place-items-center rounded-md border border-line bg-white text-slate-600 hover:bg-slate-50" aria-label="Thao tác phiên bản">
                  <MoreHorizontal className="h-4 w-4" />
                </summary>
                <div className="absolute right-0 z-20 mt-1 w-52 rounded-md border border-line bg-white p-1 shadow-lg">
                  <button
                    className="flex w-full items-center gap-2 rounded px-3 py-2 text-left text-sm font-medium text-rose-700 hover:bg-rose-50 disabled:cursor-not-allowed disabled:text-slate-400"
                    onClick={onArchiveVersion}
                    disabled={versionInUse}
                    title={versionInUse ? 'Phiên bản đang được pipeline step sử dụng' : 'Lưu trữ phiên bản'}
                  >
                    <Archive className="h-4 w-4" /> Lưu trữ phiên bản
                  </button>
                </div>
              </details>
            </div>
          ) : null}
        </div>

        <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-3">
          <MetricCard icon={Layers3} label="Trường dữ liệu" value={visibleFieldCount} />
          <MetricCard icon={GitBranch} label="Bước đang dùng" value={selectedVersion?.step_usage_count ?? 0} />
          <MetricCard icon={FileText} label="Biểu mẫu phát sinh" value={selectedVersion?.instance_count ?? 0} />
        </div>
      </div>

      <div className="p-4 md:p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 className="font-semibold text-ink">Cấu trúc biểu mẫu</h2>
            <p className="mt-1 text-xs text-slate-500">Tên nghiệp vụ được ưu tiên; khóa kỹ thuật vẫn được giữ để mapping pipeline.</p>
          </div>
          {draftDirty ? <span className="inline-flex items-center gap-1.5 rounded-md bg-amber-50 px-2.5 py-1.5 text-xs font-semibold text-amber-800"><AlertTriangle className="h-3.5 w-3.5" />Chưa lưu</span> : null}
        </div>
        {!selectedVersion ? (
          <EmptyState title="Biểu mẫu này chưa có phiên bản." action={<button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => onCreateVersion('')}>Tạo phiên bản đầu tiên</button>} />
        ) : draftSelected ? (
          <DraftFieldEditor
            fields={draftFields}
            errors={fieldErrors}
            dirty={draftDirty}
            saving={saving}
            onChange={onDraftFieldsChange}
            onSave={onSaveDraft}
            onPublish={onPublishDraft}
          />
        ) : (
          <PublishedFieldList fields={selectedVersion.fields ?? []} />
        )}
      </div>
    </section>
  );
}

function MetricCard({ icon: Icon, label, value }: { icon: LucideIcon; label: string; value: number }) {
  return (
    <div className="flex items-center gap-3 rounded-md border border-line bg-white px-3 py-2.5">
      <span className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-blue-50 text-blue-700"><Icon className="h-4 w-4" /></span>
      <div><p className="text-lg font-semibold leading-5 text-ink">{value}</p><p className="mt-0.5 text-xs text-slate-500">{label}</p></div>
    </div>
  );
}

function DraftFieldEditor({
  fields,
  errors,
  dirty,
  saving,
  onChange,
  onSave,
  onPublish,
}: {
  fields: DraftField[];
  errors: FieldErrors;
  dirty: boolean;
  saving: boolean;
  onChange: (fields: DraftField[]) => void;
  onSave: () => void;
  onPublish: () => void;
}) {
  const [expandedIndex, setExpandedIndex] = useState<number | null>(fields.length ? 0 : null);

  function updateField(index: number, patch: Partial<DraftField>) {
    onChange(fields.map((field, currentIndex) => (currentIndex === index ? { ...field, ...patch } : field)));
  }
  function updateFieldType(index: number, fieldType: string) {
    const current = fields[index];
    const validation = parseJSONObject(current.validationText) ?? {};
    if (fieldType === 'currency') {
      validation.currency_code ??= 'VND';
      validation.maximum_fraction_digits ??= resolveCurrencyFractionDigits(String(validation.currency_code));
      validation.allow_negative ??= false;
    } else if (current.field_type === 'currency') {
      delete validation.currency_code;
      delete validation.maximum_fraction_digits;
      delete validation.allow_negative;
    }
    updateField(index, { field_type: fieldType, validationText: stableJSONString(validation) });
  }
  function removeField(index: number) {
    onChange(fields.filter((_field, currentIndex) => currentIndex !== index));
    setExpandedIndex((current) => {
      if (current === index) return null;
      if (current !== null && current > index) return current - 1;
      return current;
    });
  }
  function addField() {
    onChange([...fields, blankDraftField(fields.length)]);
    setExpandedIndex(fields.length);
  }

  return (
    <div className={clsx(saving && 'pointer-events-none opacity-70')} aria-busy={saving}>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-slate-600">Chỉ bản nháp mới có thể chỉnh sửa.</p>
        <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50" onClick={addField}>
          <Plus className="h-4 w-4" />
          Thêm trường
        </button>
      </div>
      {fields.length ? (
        <div className="space-y-2">
          {fields.map((field, index) => (
            <div key={`${index}:${field.order_index}`} className={clsx('overflow-hidden rounded-md border bg-white', Object.keys(errors).some((key) => key.startsWith(`${index}.`)) ? 'border-rose-300' : 'border-line')}>
              <div className="flex items-center gap-2 px-3 py-2.5">
                <button className="flex min-w-0 flex-1 items-center gap-3 text-left" onClick={() => setExpandedIndex(expandedIndex === index ? null : index)}>
                  {expandedIndex === index ? <ChevronDown className="h-4 w-4 shrink-0 text-slate-500" /> : <ChevronRight className="h-4 w-4 shrink-0 text-slate-500" />}
                  <span className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-slate-100 text-xs font-semibold text-slate-600">{index + 1}</span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-semibold text-ink">{field.label || 'Chưa đặt tên trường'}</span>
                    <span className="mt-0.5 block truncate font-mono text-[11px] text-slate-500">{field.field_key || 'chưa_có_khóa'} · {fieldTypeLabel(field.field_type)}</span>
                  </span>
                  {field.required ? <span className="hidden shrink-0 rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700 sm:inline">Bắt buộc</span> : null}
                  {Object.keys(errors).some((key) => key.startsWith(`${index}.`)) ? <span className="shrink-0 rounded-md bg-rose-50 px-2 py-1 text-xs font-semibold text-rose-700">Cần kiểm tra</span> : null}
                </button>
                <button className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-slate-400 hover:bg-rose-50 hover:text-rose-600" onClick={() => removeField(index)} title="Xóa trường" aria-label={`Xóa trường ${field.label || index + 1}`}>
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
              {expandedIndex === index ? (
                <div className="border-t border-line bg-slate-50 p-3 md:p-4">
                  <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_220px]">
                    <FieldInput label="Tên trường *" value={field.label} error={errors[`${index}.label`]} onChange={(value) => updateField(index, { label: value })} />
                    <FieldInput label="Khóa kỹ thuật *" value={field.field_key} error={errors[`${index}.field_key`]} onChange={(value) => updateField(index, { field_key: value })} />
                    <label className="block">
                      <span className="text-xs font-semibold text-slate-600">Kiểu dữ liệu *</span>
                      <select className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm" value={field.field_type} onChange={(event) => updateFieldType(index, event.target.value)}>
                        {fieldTypeOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                      </select>
                    </label>
                  </div>
                  {field.field_type === 'currency' ? (
                    <CurrencyFieldConfiguration
                      validationText={field.validationText}
                      error={errors[`${index}.validation_json`]}
                      onChange={(validationText) => updateField(index, { validationText })}
                    />
                  ) : null}
                  <div className="mt-3 grid gap-3 md:grid-cols-[minmax(0,1fr)_180px]">
                    <FieldInput label="Hướng dẫn nhập liệu" value={field.help_text} onChange={(value) => updateField(index, { help_text: value })} />
                    <label className="flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 md:mt-5">
                      <input type="checkbox" className="h-4 w-4 rounded border-line" checked={field.required} onChange={(event) => updateField(index, { required: event.target.checked })} />
                      Trường bắt buộc
                    </label>
                  </div>
                  <details className="mt-3 rounded-md border border-line bg-white">
                    <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2 text-sm font-semibold text-slate-700">
                      <Settings2 className="h-4 w-4 text-slate-500" /> Cấu hình nâng cao
                    </summary>
                    <div className="grid gap-3 border-t border-line p-3 lg:grid-cols-3">
                      <JsonTextarea label="Lựa chọn (JSON)" value={field.optionsText} error={errors[`${index}.options_json`]} onChange={(value) => updateField(index, { optionsText: value })} />
                      <JsonTextarea label="Điều kiện kiểm tra (JSON)" value={field.validationText} error={errors[`${index}.validation_json`]} onChange={(value) => updateField(index, { validationText: value })} />
                      <JsonTextarea label="Điều kiện hiển thị (JSON)" value={field.visibilityText} error={errors[`${index}.visibility_json`]} onChange={(value) => updateField(index, { visibilityText: value })} />
                    </div>
                  </details>
                </div>
              ) : null}
            </div>
          ))}
        </div>
      ) : <EmptyState title="Bản nháp chưa có trường dữ liệu." action={<button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={addField}>Thêm trường đầu tiên</button>} />}
      <div className="sticky bottom-0 mt-4 flex flex-wrap justify-end gap-2 border-t border-line bg-white/95 pt-3 backdrop-blur">
        <button className="inline-flex items-center gap-2 rounded-md border border-emerald-200 px-3 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50 disabled:opacity-60" disabled={saving} onClick={onPublish}>
          <UploadCloud className="h-4 w-4" />
          Phát hành…
        </button>
        <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={saving || !dirty} onClick={onSave}>
          <Save className="h-4 w-4" />
          {saving ? 'Đang lưu…' : 'Lưu thay đổi'}
        </button>
      </div>
    </div>
  );
}

function CurrencyFieldConfiguration({ validationText, error, onChange }: { validationText: string; error?: string; onChange: (value: string) => void }) {
  const validation = parseJSONObject(validationText) ?? {};
  const currencyCode = String(validation.currency_code ?? 'VND').toUpperCase();
  const isVND = currencyCode === 'VND';
  function update(key: string, value: unknown) {
    const next = { ...validation };
    if (value === undefined || value === '') delete next[key];
    else next[key] = value;
    onChange(stableJSONString(next));
  }
  function updateCurrencyCode(value: string) {
    const code = value.toUpperCase();
    const next: AnyRecord = { ...validation, currency_code: code };
    if (/^[A-Z]{3}$/.test(code)) next.maximum_fraction_digits = resolveCurrencyFractionDigits(code);
    onChange(stableJSONString(next));
  }
  return (
    <section className="mt-3 rounded-md border border-blue-200 bg-blue-50/60 p-3" aria-label="Cấu hình trường tiền tệ">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        <FieldInput label="Mã tiền tệ" value={currencyCode} onChange={updateCurrencyCode} />
        <FieldInput label={isVND ? 'Số lẻ tối đa · VND = 0' : 'Số lẻ tối đa'} type="number" value={String(isVND ? 0 : validation.maximum_fraction_digits ?? resolveCurrencyFractionDigits(currencyCode))} disabled={isVND} onChange={(value) => update('maximum_fraction_digits', value === '' ? undefined : Number(value))} />
        <CurrencyConfigurationValue label="Giá trị nhỏ nhất" value={validation.min} currencyCode={currencyCode} allowNegative={Boolean(validation.allow_negative)} onChange={(value) => update('min', value)} />
        <CurrencyConfigurationValue label="Giá trị lớn nhất" value={validation.max} currencyCode={currencyCode} allowNegative={Boolean(validation.allow_negative)} onChange={(value) => update('max', value)} />
        <label className="flex items-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 xl:mt-5">
          <input type="checkbox" className="h-4 w-4 rounded border-line" checked={Boolean(validation.allow_negative)} onChange={(event) => update('allow_negative', event.target.checked)} />
          Cho phép số âm
        </label>
      </div>
      {error ? <p className="mt-2 text-xs font-medium text-rose-700">{error}</p> : <p className="mt-2 text-xs text-slate-600">VND luôn là số nguyên; tiền tệ khác dùng số lẻ ISO mặc định. Payload gửi API vẫn là số.</p>}
    </section>
  );
}

function CurrencyConfigurationValue({ label, value, currencyCode, allowNegative, onChange }: { label: string; value: unknown; currencyCode: string; allowNegative: boolean; onChange: (value: number | undefined) => void }) {
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <CurrencyInput
        className="mt-1"
        inputClassName="text-sm focus:ring-blue-500"
        value={value === undefined ? '' : String(value)}
        currencyCode={currencyCode}
        allowNegative={allowNegative}
        onValueChange={({ floatValue }) => onChange(floatValue ?? undefined)}
      />
    </label>
  );
}

function PublishedFieldList({ fields }: { fields: WorkflowFormField[] }) {
  return fields.length ? (
    <>
      <div className="space-y-2 md:hidden">
        {fields.map((field, index) => (
          <div key={field.field_key} className="rounded-md border border-line p-3">
            <div className="flex items-start gap-3">
              <span className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-slate-100 text-xs font-semibold text-slate-600">{index + 1}</span>
              <div className="min-w-0 flex-1">
                <p className="font-semibold text-ink">{field.label}</p>
                <p className="mt-1 break-all font-mono text-xs text-slate-500">{field.field_key}</p>
                <div className="mt-2 flex flex-wrap gap-2 text-xs"><span className="rounded bg-slate-100 px-2 py-1 text-slate-700">{fieldTypeLabel(field.field_type)}</span>{field.required ? <span className="rounded bg-blue-50 px-2 py-1 font-semibold text-blue-700">Bắt buộc</span> : <span className="rounded bg-slate-50 px-2 py-1 text-slate-500">Không bắt buộc</span>}</div>
                {field.help_text ? <p className="mt-2 text-xs leading-5 text-slate-600">{field.help_text}</p> : null}
              </div>
            </div>
          </div>
        ))}
      </div>
      <div className="hidden overflow-hidden rounded-md border border-line md:block">
      <table className="min-w-full divide-y divide-line text-sm">
        <thead className="bg-slate-50 text-xs font-semibold uppercase tracking-wide text-slate-500">
          <tr>
            <th className="w-12 px-3 py-2 text-left">#</th>
            <th className="px-3 py-2 text-left">Tên trường</th>
            <th className="px-3 py-2 text-left">Khóa kỹ thuật</th>
            <th className="px-3 py-2 text-left">Kiểu dữ liệu</th>
            <th className="px-3 py-2 text-left">Yêu cầu</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-line bg-white">
          {fields.map((field, index) => (
            <tr key={field.field_key}>
              <td className="px-3 py-3 text-xs text-slate-400">{index + 1}</td>
              <td className="px-3 py-3"><p className="font-medium text-ink">{field.label}</p>{field.help_text ? <p className="mt-1 max-w-md text-xs leading-5 text-slate-500">{field.help_text}</p> : null}</td>
              <td className="px-3 py-3 font-mono text-xs text-slate-600">{field.field_key}</td>
              <td className="px-3 py-3 text-slate-600">{fieldTypeLabel(field.field_type)}</td>
              <td className="px-3 py-3">{field.required ? <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">Bắt buộc</span> : <span className="text-xs text-slate-500">Không bắt buộc</span>}</td>
            </tr>
          ))}
        </tbody>
      </table>
      </div>
    </>
  ) : <EmptyState title="Phiên bản này chưa có trường dữ liệu." />;
}

function TemplateDialog({
  mode,
  form,
  originalStatus,
  submitAttempted,
  submitting,
  onChange,
  onClose,
  onSubmit,
}: {
  mode: 'create' | 'edit' | null;
  form: TemplateForm;
  originalStatus?: string;
  submitAttempted: boolean;
  submitting: boolean;
  onChange: (form: TemplateForm) => void;
  onClose: () => void;
  onSubmit: () => void;
}) {
  const missing = templateMissingFields(form, mode === 'create');
  return (
    <Dialog
      open={Boolean(mode)}
      title={mode === 'create' ? 'Tạo biểu mẫu workflow' : 'Sửa thông tin biểu mẫu'}
      description={mode === 'create' ? 'Tạo template nghiệp vụ trước khi bổ sung và phát hành phiên bản field.' : 'Cập nhật tên, mô tả và trạng thái của template. Phiên bản đã phát hành không bị thay đổi.'}
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Hủy</button>
          <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={submitting} onClick={onSubmit}>
            {mode === 'create' ? 'Tạo biểu mẫu' : 'Lưu thông tin'}
          </button>
        </div>
      }
    >
      <div className="grid gap-3 md:grid-cols-2">
        <FieldInput label="Mã biểu mẫu *" value={form.code} disabled={mode === 'edit'} error={submitAttempted && missing.includes('Mã biểu mẫu') ? 'Bắt buộc' : ''} onChange={(value) => onChange({ ...form, code: value })} />
        <FieldInput label="Tên biểu mẫu *" value={form.name} error={submitAttempted && missing.includes('Tên biểu mẫu') ? 'Bắt buộc' : ''} onChange={(value) => onChange({ ...form, name: value })} />
        <label className="block">
          <span className="text-xs font-semibold text-slate-600">Loại biểu mẫu *</span>
          <select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm disabled:bg-slate-100" value={form.form_type} disabled={mode === 'edit'} onChange={(event) => onChange({ ...form, form_type: event.target.value })}>
            {formTypeOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
        </label>
        <label className="block">
          <span className="text-xs font-semibold text-slate-600">Trạng thái</span>
          <select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" value={form.status} onChange={(event) => onChange({ ...form, status: event.target.value })}>
            <option value="active">Đang dùng</option>
            {originalStatus === 'archived' ? <option value="archived">Đã lưu trữ</option> : null}
          </select>
        </label>
        <label className="block md:col-span-2">
          <span className="text-xs font-semibold text-slate-600">Mô tả</span>
          <textarea className="mt-1 min-h-24 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500" value={form.description} onChange={(event) => onChange({ ...form, description: event.target.value })} />
        </label>
      </div>
    </Dialog>
  );
}

function ConfirmationDialog({
  action,
  template,
  version,
  fieldCount,
  pending,
  onClose,
  onConfirm,
}: {
  action: ConfirmAction;
  template?: WorkflowFormManagedTemplate;
  version?: WorkflowFormManagedVersion;
  fieldCount: number;
  pending: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const content = confirmationContent(action, template, version, fieldCount);
  return (
    <Dialog
      open={Boolean(action)}
      title={content.title}
      description={content.description}
      onClose={pending ? () => undefined : onClose}
      size="sm"
      footer={
        <div className="flex justify-end gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" disabled={pending} onClick={onClose}>Hủy</button>
          <button className={clsx('rounded-md px-3 py-2 text-sm font-semibold text-white disabled:opacity-60', action === 'publish' ? 'bg-emerald-600 hover:bg-emerald-700' : 'bg-rose-600 hover:bg-rose-700')} disabled={pending} onClick={onConfirm}>
            {pending ? 'Đang xử lý…' : content.confirmLabel}
          </button>
        </div>
      }
    >
      <div className={clsx('flex gap-3 rounded-md border p-3', action === 'publish' ? 'border-emerald-200 bg-emerald-50' : 'border-amber-200 bg-amber-50')}>
        <AlertTriangle className={clsx('mt-0.5 h-5 w-5 shrink-0', action === 'publish' ? 'text-emerald-700' : 'text-amber-700')} />
        <p className="text-sm leading-6 text-slate-700">{content.notice}</p>
      </div>
    </Dialog>
  );
}

function UnsavedChangesDialog({ open, onClose, onDiscard }: { open: boolean; onClose: () => void; onDiscard: () => void }) {
  return (
    <Dialog
      open={open}
      title="Bản nháp có thay đổi chưa lưu"
      description="Nếu tiếp tục, các thay đổi kể từ lần lưu gần nhất sẽ bị bỏ."
      onClose={onClose}
      size="sm"
      footer={
        <div className="flex justify-end gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Ở lại chỉnh sửa</button>
          <button className="rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700" onClick={onDiscard}>Bỏ thay đổi và tiếp tục</button>
        </div>
      }
    >
      <div className="flex gap-3 rounded-md border border-amber-200 bg-amber-50 p-3">
        <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-700" />
        <p className="text-sm leading-6 text-slate-700">Hãy chọn “Ở lại chỉnh sửa” nếu anh cần lưu hoặc phát hành bản nháp hiện tại.</p>
      </div>
    </Dialog>
  );
}

function WorkflowFormsLoading() {
  return (
    <div className="grid animate-pulse items-start gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
      <div className="rounded-lg border border-line bg-white p-4"><div className="h-5 w-40 rounded bg-slate-200" /><div className="mt-4 h-10 rounded bg-slate-100" /><div className="mt-3 space-y-2">{[1, 2, 3, 4].map((item) => <div key={item} className="h-20 rounded bg-slate-100" />)}</div></div>
      <WorkspaceLoading />
    </div>
  );
}

function WorkspaceLoading() {
  return <div className="animate-pulse rounded-lg border border-line bg-white p-5 shadow-panel"><div className="h-6 w-2/5 rounded bg-slate-200" /><div className="mt-3 h-4 w-3/4 rounded bg-slate-100" /><div className="mt-6 h-24 rounded bg-slate-100" /><div className="mt-4 h-56 rounded bg-slate-100" /></div>;
}

function ErrorPanel({ title, onRetry }: { title: string; onRetry: () => void }) {
  return (
    <div className="rounded-md border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700">
      <p className="font-semibold">{title}</p>
      <p className="mt-1 text-xs text-rose-600">Vui lòng thử tải lại dữ liệu.</p>
      <button className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold text-rose-700 hover:bg-rose-100" onClick={onRetry}>Thử lại</button>
    </div>
  );
}

function FieldInput({ label, value, type = 'text', error, disabled, onChange }: { label: string; value: string; type?: string; error?: string; disabled?: boolean; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <input
        type={type}
        className={clsx('mt-1 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-100', error ? 'border-rose-300 bg-rose-50' : 'border-line bg-white')}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
      />
      {error ? <span className="mt-1 block text-xs font-medium text-rose-600">{error}</span> : null}
    </label>
  );
}

function JsonTextarea({ label, value, error, onChange }: { label: string; value: string; error?: string; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <textarea
        className={clsx('mt-1 min-h-20 w-full rounded-md border px-3 py-2 font-mono text-xs outline-none focus:ring-2 focus:ring-blue-500', error ? 'border-rose-300 bg-rose-50' : 'border-line bg-white')}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      {error ? <span className="mt-1 block text-xs font-medium text-rose-600">{error}</span> : null}
    </label>
  );
}

function templateMatches(template: WorkflowFormTemplateLibraryItem, search: string, type: string, status: string) {
  const haystack = `${template.template_code} ${template.template_name} ${template.template_description ?? ''}`.toLowerCase();
  const needle = search.trim().toLowerCase();
  if (needle && !haystack.includes(needle)) return false;
  if (type !== 'all' && template.form_type !== type) return false;
  if (status !== 'all' && template.template_status !== status) return false;
  return true;
}

function templateMissingFields(form: TemplateForm, requireCode: boolean) {
  const missing: string[] = [];
  if (requireCode && !form.code.trim()) missing.push('Mã biểu mẫu');
  if (!form.name.trim()) missing.push('Tên biểu mẫu');
  if (!form.form_type.trim()) missing.push('Loại biểu mẫu');
  return missing;
}

function fieldToDraft(field: WorkflowFormField): DraftField {
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

function blankDraftField(index: number): DraftField {
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

function draftFieldsPayload(fields: DraftField[]) {
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

function validateDraftFields(fields: DraftField[]) {
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

function stableJSONString(value: unknown) {
  try {
    return JSON.stringify(value ?? {}, null, 2);
  } catch {
    return '{}';
  }
}

function parseJSONOrDefault(value: string, fallback: AnyRecord | unknown[]) {
  try {
    return JSON.parse(value || JSON.stringify(fallback));
  } catch {
    return fallback;
  }
}

function parseJSONObject(value: string): AnyRecord | null {
  try {
    const parsed = JSON.parse(value || '{}');
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function validJSON(value: string) {
  try {
    JSON.parse(value || '{}');
    return true;
  } catch {
    return false;
  }
}

function formTypeLabel(value?: string) {
  return formTypeOptions.find(([type]) => type === value)?.[1] ?? value ?? 'Biểu mẫu workflow';
}

function fieldTypeLabel(value?: string) {
  return fieldTypeOptions.find(([type]) => type === value)?.[1] ?? value ?? 'Chưa xác định';
}

function versionStatusLabel(value?: string) {
  if (value === 'draft') return 'Bản nháp';
  if (value === 'published') return 'Đã phát hành';
  if (value === 'archived') return 'Đã lưu trữ';
  return value || 'Chưa xác định';
}

function draftFieldsFingerprint(fields: DraftField[]) {
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

function confirmationContent(
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
