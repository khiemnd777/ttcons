import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Archive, ClipboardList, Copy, FilePlus2, Plus, Save, UploadCloud, X } from 'lucide-react';
import clsx from 'clsx';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';
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

const formTypeOptions = [
  ['generic_submission', 'Submission'],
  ['checklist', 'Checklist'],
  ['daily_report', 'Daily report'],
  ['meeting_minutes', 'Biên bản họp'],
];

const fieldTypeOptions = [
  ['text', 'Text'],
  ['textarea', 'Textarea'],
  ['number', 'Number'],
  ['date', 'Date'],
  ['select', 'Select'],
  ['multi_select', 'Multi select'],
  ['checkbox', 'Checkbox'],
  ['evidence_reference', 'Evidence'],
  ['structured_list', 'Structured list'],
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
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});

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
    setDraftFields((selectedVersion?.fields ?? []).map(fieldToDraft));
    setFieldErrors({});
  }, [selectedVersion?.version_id]);

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
      toast.success('Đã tạo workflow form template');
    },
    onError: (error) => toast.error('Không tạo được workflow form template', error),
  });

  const updateTemplate = useMutation({
    mutationFn: (payload: TemplateForm) => put<WorkflowFormManagedTemplate>(`/workflow-form-templates/${selectedTemplateId}`, payload),
    onSuccess: () => {
      closeTemplateDialog();
      invalidate();
      toast.success('Đã cập nhật workflow form template');
    },
    onError: (error) => toast.error('Không cập nhật được workflow form template', error),
  });

  const archiveTemplate = useMutation({
    mutationFn: () => post<WorkflowFormManagedTemplate>(`/workflow-form-templates/${selectedTemplateId}/archive`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã archive workflow form template');
    },
    onError: (error) => toast.error('Không archive được workflow form template', error),
  });

  const createVersion = useMutation({
    mutationFn: (sourceVersionId?: string) => post<WorkflowFormManagedVersion>(`/workflow-form-templates/${selectedTemplateId}/versions`, { source_version_id: sourceVersionId ?? '' }),
    onSuccess: (version) => {
      setSelectedVersionId(version.version_id);
      invalidate();
      toast.success('Đã tạo draft version');
    },
    onError: (error) => toast.error('Không tạo được draft version', error),
  });

  const saveVersion = useMutation({
    mutationFn: () => put<WorkflowFormManagedVersion>(`/workflow-form-versions/${selectedVersion?.version_id}`, { fields: draftFieldsPayload(draftFields) }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã lưu draft workflow form');
    },
    onError: (error) => toast.error('Không lưu được draft workflow form', error),
  });

  const publishVersion = useMutation({
    mutationFn: async () => {
      await put<WorkflowFormManagedVersion>(`/workflow-form-versions/${selectedVersion?.version_id}`, { fields: draftFieldsPayload(draftFields) });
      return post<WorkflowFormManagedVersion>(`/workflow-form-versions/${selectedVersion?.version_id}/publish`, {});
    },
    onSuccess: () => {
      invalidate();
      toast.success('Đã publish workflow form version');
    },
    onError: (error) => toast.error('Không publish được workflow form version', error),
  });

  const archiveVersion = useMutation({
    mutationFn: () => post<WorkflowFormManagedVersion>(`/workflow-form-versions/${selectedVersion?.version_id}/archive`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã archive workflow form version');
    },
    onError: (error) => toast.error('Không archive được workflow form version', error),
  });

  function openCreateTemplate() {
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
      toast.warn({ title: 'Thiếu dữ liệu workflow form', description: missing.join(', ') });
      return;
    }
    if (templateDialog === 'create') createTemplate.mutate(templateForm);
    else updateTemplate.mutate(templateForm);
  }

  function saveDraft(publish = false) {
    const errors = validateDraftFields(draftFields);
    setFieldErrors(errors);
    if (Object.keys(errors).length) {
      toast.warn({ title: 'Field workflow form chưa hợp lệ', description: Object.values(errors).slice(0, 4).join(', ') });
      return;
    }
    if (publish) publishVersion.mutate();
    else saveVersion.mutate();
  }

  return (
    <>
      <PageHeader
        title="Workflow Forms"
        description="Quản trị template, version và field dùng để map pipeline step thành biểu mẫu nghiệp vụ."
        actions={
          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={openCreateTemplate}>
            <Plus className="h-4 w-4" />
            New template
          </button>
        }
      />

      {library.isLoading ? <LoadingSpinner /> : null}
      {library.error ? (
        <div className="rounded-md border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700">
          <p className="font-semibold">Không tải được workflow form library</p>
          <button className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold text-rose-700 hover:bg-rose-100" onClick={() => library.refetch()}>
            Thử lại
          </button>
        </div>
      ) : null}

      {!library.isLoading && !library.error ? (
        <div className="grid gap-4 xl:grid-cols-[360px_minmax(0,1fr)]">
          <aside className="space-y-3">
            <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
              <div className="mb-3 flex items-center gap-2">
                <ClipboardList className="h-5 w-5 text-blue-700" />
                <h2 className="font-semibold text-ink">Templates</h2>
              </div>
              <div className="space-y-2">
                <input
                  className="w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="Search code, name"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                />
                <div className="grid grid-cols-2 gap-2">
                  <select className="rounded-md border border-line px-3 py-2 text-sm" value={typeFilter} onChange={(event) => setTypeFilter(event.target.value)}>
                    <option value="all">All type</option>
                    {formTypeOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                  </select>
                  <select className="rounded-md border border-line px-3 py-2 text-sm" value={statusFilter} onChange={(event) => setStatusFilter(event.target.value)}>
                    <option value="all">All status</option>
                    <option value="active">Active</option>
                    <option value="archived">Archived</option>
                  </select>
                </div>
              </div>
            </div>

            <div className="space-y-2">
              {filteredTemplates.length ? filteredTemplates.map((template) => (
                <button
                  key={template.template_id}
                  className={clsx(
                    'block w-full rounded-md border bg-white px-3 py-3 text-left shadow-sm hover:border-blue-300 hover:bg-blue-50',
                    selectedTemplateId === template.template_id ? 'border-blue-400 ring-2 ring-blue-100' : 'border-line',
                  )}
                  onClick={() => setSelectedTemplateId(template.template_id)}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold text-ink">{template.template_name}</p>
                      <p className="mt-1 truncate text-xs text-slate-500">{template.template_code}</p>
                    </div>
                    <StatusBadge status={template.template_status} />
                  </div>
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{formTypeLabel(template.form_type)}</span>
                    <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">v{template.latest_version_no ?? '-'}</span>
                    <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{template.step_usage_count ?? 0} steps</span>
                  </div>
                </button>
              )) : <EmptyState title="Chưa có workflow form template khớp bộ lọc." />}
            </div>
          </aside>

          <main className="space-y-4">
            {!selectedTemplateId ? <EmptyState title="Chọn hoặc tạo workflow form template." /> : null}
            {detail.isLoading ? <LoadingSpinner /> : null}
            {detail.error ? (
              <div className="rounded-md border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700">
                <p className="font-semibold">Không tải được workflow form template</p>
                <button className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold text-rose-700 hover:bg-rose-100" onClick={() => detail.refetch()}>
                  Thử lại
                </button>
              </div>
            ) : null}
            {detail.data ? (
              <>
                <TemplateHeader
                  template={detail.data}
                  onEdit={openEditTemplate}
                  onArchive={() => archiveTemplate.mutate()}
                  onNewDraft={() => createVersion.mutate(selectedVersion?.version_id)}
                  archiving={archiveTemplate.isPending}
                  creatingVersion={createVersion.isPending}
                />
                <VersionStrip versions={versions} selectedVersionId={selectedVersion?.version_id ?? ''} onSelect={setSelectedVersionId} />
                {selectedVersion ? (
                  <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
                    <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                      <div>
                        <h2 className="font-semibold text-ink">Version v{selectedVersion.version_no}</h2>
                        <p className="mt-1 text-xs text-slate-500">
                          {selectedVersion.field_count ?? 0} fields · {selectedVersion.step_usage_count ?? 0} mapped steps · {selectedVersion.instance_count ?? 0} instances
                        </p>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <button
                          className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                          onClick={() => createVersion.mutate(selectedVersion.version_id)}
                          disabled={createVersion.isPending}
                        >
                          <Copy className="h-4 w-4" />
                          Clone draft
                        </button>
                        <button
                          className="inline-flex items-center gap-2 rounded-md border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:opacity-60"
                          onClick={() => archiveVersion.mutate()}
                          disabled={!selectedVersion.version_id || (selectedVersion.step_usage_count ?? 0) > 0 || archiveVersion.isPending}
                          title={(selectedVersion.step_usage_count ?? 0) > 0 ? 'Version đang được pipeline step sử dụng' : 'Archive version'}
                        >
                          <Archive className="h-4 w-4" />
                          Archive
                        </button>
                      </div>
                    </div>
                    {draftSelected ? (
                      <DraftFieldEditor
                        fields={draftFields}
                        errors={fieldErrors}
                        saving={saveVersion.isPending || publishVersion.isPending}
                        onChange={setDraftFields}
                        onSave={() => saveDraft(false)}
                        onPublish={() => saveDraft(true)}
                      />
                    ) : (
                      <PublishedFieldList fields={selectedVersion.fields ?? []} />
                    )}
                  </section>
                ) : <EmptyState title="Template này chưa có version." action={<button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => createVersion.mutate('')}>Tạo draft version</button>} />}
              </>
            ) : null}
          </main>
        </div>
      ) : null}

      <TemplateDialog
        mode={templateDialog}
        form={templateForm}
        submitAttempted={templateSubmitAttempted}
        submitting={createTemplate.isPending || updateTemplate.isPending}
        onChange={setTemplateForm}
        onClose={closeTemplateDialog}
        onSubmit={submitTemplateForm}
      />
    </>
  );
}

function TemplateHeader({
  template,
  onEdit,
  onArchive,
  onNewDraft,
  archiving,
  creatingVersion,
}: {
  template: WorkflowFormManagedTemplate;
  onEdit: () => void;
  onArchive: () => void;
  onNewDraft: () => void;
  archiving: boolean;
  creatingVersion: boolean;
}) {
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-lg font-semibold text-ink">{template.template_name}</h1>
            <StatusBadge status={template.template_status} />
            <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{formTypeLabel(template.form_type)}</span>
          </div>
          <p className="mt-1 text-sm text-slate-600">{template.template_description || 'Không có mô tả.'}</p>
          <p className="mt-2 text-xs text-slate-500">
            Code: {template.template_code} · {template.step_usage_count ?? 0} mapped steps · {template.instance_count ?? 0} active instances
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onEdit}>
            Edit metadata
          </button>
          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" onClick={onNewDraft} disabled={creatingVersion}>
            <FilePlus2 className="h-4 w-4" />
            New draft
          </button>
          <button className="inline-flex items-center gap-2 rounded-md border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:opacity-60" onClick={onArchive} disabled={archiving || (template.step_usage_count ?? 0) > 0}>
            <Archive className="h-4 w-4" />
            Archive
          </button>
        </div>
      </div>
    </section>
  );
}

function VersionStrip({ versions, selectedVersionId, onSelect }: { versions: WorkflowFormManagedVersion[]; selectedVersionId: string; onSelect: (id: string) => void }) {
  if (!versions.length) return null;
  return (
    <section className="flex gap-2 overflow-x-auto rounded-lg border border-line bg-white p-3 shadow-panel">
      {versions.map((version) => (
        <button
          key={version.version_id}
          className={clsx(
            'min-w-44 rounded-md border px-3 py-2 text-left hover:border-blue-300 hover:bg-blue-50',
            selectedVersionId === version.version_id ? 'border-blue-400 bg-blue-50' : 'border-line',
          )}
          onClick={() => onSelect(version.version_id)}
        >
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-semibold text-ink">v{version.version_no}</span>
            <StatusBadge status={version.version_status} />
          </div>
          <p className="mt-1 text-xs text-slate-500">
            {version.latest_published ? 'Latest published' : `${version.field_count ?? 0} fields`}
          </p>
        </button>
      ))}
    </section>
  );
}

function DraftFieldEditor({
  fields,
  errors,
  saving,
  onChange,
  onSave,
  onPublish,
}: {
  fields: DraftField[];
  errors: FieldErrors;
  saving: boolean;
  onChange: (fields: DraftField[]) => void;
  onSave: () => void;
  onPublish: () => void;
}) {
  function updateField(index: number, patch: Partial<DraftField>) {
    onChange(fields.map((field, currentIndex) => (currentIndex === index ? { ...field, ...patch } : field)));
  }
  function removeField(index: number) {
    onChange(fields.filter((_field, currentIndex) => currentIndex !== index));
  }
  function addField() {
    onChange([...fields, blankDraftField(fields.length)]);
  }

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink">Draft fields</h3>
        <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50" onClick={addField}>
          <Plus className="h-4 w-4" />
          Add field
        </button>
      </div>
      {fields.length ? (
        <div className="space-y-3">
          {fields.map((field, index) => (
            <div key={`${index}:${field.field_key}`} className="rounded-md border border-line bg-slate-50 p-3">
              <div className="grid gap-3 lg:grid-cols-[160px_minmax(0,1fr)_160px_96px_44px]">
                <FieldInput label="Field key" value={field.field_key} error={errors[`${index}.field_key`]} onChange={(value) => updateField(index, { field_key: value })} />
                <FieldInput label="Label" value={field.label} error={errors[`${index}.label`]} onChange={(value) => updateField(index, { label: value })} />
                <label className="block">
                  <span className="text-xs font-semibold text-slate-600">Type</span>
                  <select className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm" value={field.field_type} onChange={(event) => updateField(index, { field_type: event.target.value })}>
                    {fieldTypeOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
                  </select>
                </label>
                <label className="flex items-end gap-2 pb-2 text-xs font-semibold text-slate-700">
                  <input type="checkbox" className="h-4 w-4 rounded border-line" checked={field.required} onChange={(event) => updateField(index, { required: event.target.checked })} />
                  Required
                </label>
                <button className="mt-5 grid h-9 w-9 place-items-center rounded-md border border-line text-slate-500 hover:bg-white hover:text-rose-600" onClick={() => removeField(index)} title="Remove field">
                  <X className="h-4 w-4" />
                </button>
              </div>
              <div className="mt-3 grid gap-3 lg:grid-cols-3">
                <FieldInput label="Help text" value={field.help_text} onChange={(value) => updateField(index, { help_text: value })} />
                <JsonTextarea label="Options JSON" value={field.optionsText} error={errors[`${index}.options_json`]} onChange={(value) => updateField(index, { optionsText: value })} />
                <JsonTextarea label="Validation JSON" value={field.validationText} error={errors[`${index}.validation_json`]} onChange={(value) => updateField(index, { validationText: value })} />
              </div>
            </div>
          ))}
        </div>
      ) : <EmptyState title="Draft version chưa có field." action={<button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={addField}>Add field</button>} />}
      <div className="mt-4 flex flex-wrap justify-end gap-2 border-t border-line pt-3">
        <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" disabled={saving} onClick={onSave}>
          <Save className="h-4 w-4" />
          Save draft
        </button>
        <button className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:opacity-60" disabled={saving} onClick={onPublish}>
          <UploadCloud className="h-4 w-4" />
          Publish version
        </button>
      </div>
    </div>
  );
}

function PublishedFieldList({ fields }: { fields: WorkflowFormField[] }) {
  return fields.length ? (
    <div className="overflow-hidden rounded-md border border-line">
      <table className="min-w-full divide-y divide-line text-sm">
        <thead className="bg-slate-50 text-xs font-semibold uppercase tracking-wide text-slate-500">
          <tr>
            <th className="px-3 py-2 text-left">Key</th>
            <th className="px-3 py-2 text-left">Label</th>
            <th className="px-3 py-2 text-left">Type</th>
            <th className="px-3 py-2 text-left">Required</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-line bg-white">
          {fields.map((field) => (
            <tr key={field.field_key}>
              <td className="px-3 py-2 font-mono text-xs text-slate-700">{field.field_key}</td>
              <td className="px-3 py-2 font-medium text-ink">{field.label}</td>
              <td className="px-3 py-2 text-slate-600">{field.field_type}</td>
              <td className="px-3 py-2 text-slate-600">{field.required ? 'Yes' : 'No'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  ) : <EmptyState title="Version này chưa có field." />;
}

function TemplateDialog({
  mode,
  form,
  submitAttempted,
  submitting,
  onChange,
  onClose,
  onSubmit,
}: {
  mode: 'create' | 'edit' | null;
  form: TemplateForm;
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
      title={mode === 'create' ? 'New workflow form template' : 'Edit workflow form template'}
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Hủy</button>
          <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={submitting} onClick={onSubmit}>
            {mode === 'create' ? 'Tạo template' : 'Lưu metadata'}
          </button>
        </div>
      }
    >
      <div className="grid gap-3 md:grid-cols-2">
        <FieldInput label="Code" value={form.code} disabled={mode === 'edit'} error={submitAttempted && missing.includes('Code') ? 'Bắt buộc' : ''} onChange={(value) => onChange({ ...form, code: value })} />
        <FieldInput label="Name" value={form.name} error={submitAttempted && missing.includes('Name') ? 'Bắt buộc' : ''} onChange={(value) => onChange({ ...form, name: value })} />
        <label className="block">
          <span className="text-xs font-semibold text-slate-600">Form type</span>
          <select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm disabled:bg-slate-100" value={form.form_type} disabled={mode === 'edit'} onChange={(event) => onChange({ ...form, form_type: event.target.value })}>
            {formTypeOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
          </select>
        </label>
        <label className="block">
          <span className="text-xs font-semibold text-slate-600">Status</span>
          <select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" value={form.status} onChange={(event) => onChange({ ...form, status: event.target.value })}>
            <option value="active">Active</option>
            <option value="archived">Archived</option>
          </select>
        </label>
        <label className="block md:col-span-2">
          <span className="text-xs font-semibold text-slate-600">Description</span>
          <textarea className="mt-1 min-h-24 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500" value={form.description} onChange={(event) => onChange({ ...form, description: event.target.value })} />
        </label>
      </div>
    </Dialog>
  );
}

function FieldInput({ label, value, error, disabled, onChange }: { label: string; value: string; error?: string; disabled?: boolean; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <input
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
  if (requireCode && !form.code.trim()) missing.push('Code');
  if (!form.name.trim()) missing.push('Name');
  if (!form.form_type.trim()) missing.push('Form type');
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
    label: 'Field mới',
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
    if (!/^[a-z][a-z0-9_]*$/.test(key)) errors[`${index}.field_key`] = 'Key phải dạng snake_case';
    if (seen.has(key)) errors[`${index}.field_key`] = 'Key bị trùng';
    seen.add(key);
    if (!field.label.trim()) errors[`${index}.label`] = 'Label bắt buộc';
    if (!validJSON(field.optionsText)) errors[`${index}.options_json`] = 'JSON không hợp lệ';
    if (!validJSON(field.validationText)) errors[`${index}.validation_json`] = 'JSON không hợp lệ';
  });
  if (!fields.length) errors.form = 'Cần ít nhất một field';
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

function validJSON(value: string) {
  try {
    JSON.parse(value || '{}');
    return true;
  } catch {
    return false;
  }
}

function formTypeLabel(value?: string) {
  return formTypeOptions.find(([type]) => type === value)?.[1] ?? value ?? 'Workflow form';
}
