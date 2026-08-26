import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, ArrowLeft, Check, CheckCircle2, LockKeyhole, PlayCircle, RefreshCw, Search, UserCog, Users } from 'lucide-react';
import clsx from 'clsx';
import { Link } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { useToast } from '../../components/ToastProvider';
import { ApiError, get, post, put } from '../../lib/api/client';
import type {
  Project,
  ProjectMember,
  ProjectMemberAccessRole,
  ProjectMembersResponse,
  ProjectMemberStaffOption,
  ProjectStartupReadiness,
} from '../../lib/types/domain';
import { SimpleForm, type FormValues } from '../_shared/SimpleForm';
import { buildProjectFormFields, useProjectFormCatalogs } from './projectFormFields';

type WizardStep = 1 | 2 | 3;

type MemberDraft = {
  staff: ProjectMemberStaffOption;
  role_ids: string[];
  project_role_ids: string[];
  notes: string;
};

type BulkMemberResult = {
  items: ProjectMember[];
  total: number;
};

export function ProjectStartupWizard({
  open,
  projectId,
  onCreateProject,
  onUpdateProject,
  onClose,
  onChanged,
}: {
  open: boolean;
  projectId?: string | null;
  onCreateProject?: (values: FormValues) => Promise<Project>;
  onUpdateProject?: (projectId: string, values: FormValues) => Promise<Project>;
  onClose: () => void;
  onChanged: () => void;
}) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [createdProject, setCreatedProject] = useState<Project | null>(null);
  const [step, setStep] = useState<WizardStep>(projectId ? 2 : 1);
  const [resolvedProjectID, setResolvedProjectID] = useState('');
  const [search, setSearch] = useState('');
  const [drafts, setDrafts] = useState<Record<string, MemberDraft>>({});
  const [submittedMembers, setSubmittedMembers] = useState(false);
  const [selectedPMStaffID, setSelectedPMStaffID] = useState('');
  const activeProjectID = projectId || createdProject?.id || '';
  const projectFormCatalogs = useProjectFormCatalogs(open);

  const projectDetail = useQuery({
    queryKey: ['project', activeProjectID],
    queryFn: () => get<Project>(`/projects/${activeProjectID}`),
    enabled: open && Boolean(projectId && activeProjectID),
    retry: false,
  });

  const members = useQuery({
    queryKey: ['project-members', activeProjectID],
    queryFn: () => get<ProjectMembersResponse>(`/projects/${activeProjectID}/members?include_inactive=true`),
    enabled: open && Boolean(activeProjectID),
    retry: false,
  });
  const readiness = useQuery({
    queryKey: ['project-startup-readiness', activeProjectID],
    queryFn: () => get<ProjectStartupReadiness>(`/projects/${activeProjectID}/startup-readiness`),
    enabled: open && Boolean(activeProjectID),
    retry: false,
  });

  const savedActiveMembers = useMemo(
    () => (members.data?.items ?? []).filter((member) => member.active),
    [members.data?.items],
  );
  const activeMembers = useMemo(
    () => savedActiveMembers.filter((member) => member.user_id && member.account_active !== false),
    [savedActiveMembers],
  );
  const eligibleActiveMembers = useMemo(
    () => activeMembers.filter(memberCanBeProjectManager),
    [activeMembers],
  );

  useEffect(() => {
    if (!open) {
      setCreatedProject(null);
      setResolvedProjectID('');
      setSearch('');
      setDrafts({});
      setSubmittedMembers(false);
      setSelectedPMStaffID('');
      setStep(projectId ? 2 : 1);
    }
  }, [open, projectId]);

  useEffect(() => {
    if (!open || !activeProjectID || !members.data || resolvedProjectID === activeProjectID) return;
    const canManage = members.data.capabilities.can_manage;
    setStep(canManage && (!activeMembers.length || !eligibleActiveMembers.length) ? 2 : 3);
    setResolvedProjectID(activeProjectID);
  }, [activeMembers.length, activeProjectID, eligibleActiveMembers.length, members.data, open, resolvedProjectID]);

  useEffect(() => {
    setSelectedPMStaffID(readiness.data?.pm_staff_id ?? '');
  }, [activeProjectID, readiness.data?.pm_staff_id]);

  const saveMembers = useMutation({
    mutationFn: (payload: MemberDraft[]) => post<BulkMemberResult>(`/projects/${activeProjectID}/members/bulk`, {
      members: payload.map((draft) => ({
        staff_id: draft.staff.id,
        role_ids: draft.role_ids,
        project_role_ids: draft.project_role_ids,
        notes: draft.notes,
      })),
    }),
  });
  const setProjectManager = useMutation({
    mutationFn: (staffId: string) => put(`/projects/${activeProjectID}/project-manager`, { staff_id: staffId }),
  });
  const startPipeline = useMutation({
    mutationFn: () => post(`/projects/${activeProjectID}/start-pipeline`, { template_id: readiness.data?.template_id }),
  });

  const title = readiness.data
    ? `Thiết lập ${readiness.data.project_code} · ${readiness.data.project_name}`
    : createdProject
      ? `Thiết lập ${createdProject.code} · ${createdProject.name}`
      : onCreateProject
        ? 'Tạo công trình mới'
        : 'Thiết lập công trình';
  const canEditMembers = Boolean(members.data?.capabilities.can_manage);
  const canUpdateProject = Boolean(readiness.data?.capabilities.can_update_project);
  const editableProject = createdProject ?? projectDetail.data;
  const accessRoles = members.data?.options.access_roles ?? [];
  const roleLabels = members.data?.options.role_labels ?? [];
  const inactiveByStaffID = useMemo(
    () => new Map((members.data?.items ?? []).filter((member) => !member.active).map((member) => [member.staff_id, member])),
    [members.data?.items],
  );
  const staffOptions = members.data?.options.staff ?? [];
  const normalizedSearch = normalizeMemberSearch(search);
  const visibleStaffOptions = staffOptions.filter((staff) => {
    if (staff.member_active) return false;
    if (!normalizedSearch) return true;
    return normalizeMemberSearch([
      staff.full_name,
      staff.email,
      staff.department,
      staff.position,
      ...staff.global_roles.map((role) => role.display_name),
    ].filter(Boolean).join(' ')).includes(normalizedSearch);
  });
  const selectedDrafts = Object.values(drafts);
  const invalidDrafts = selectedDrafts.filter((draft) => !draft.role_ids.length || !draftHasProjectView(draft, accessRoles));
  const draftPMEligible = selectedDrafts.some((draft) => draftCanBeProjectManager(draft, accessRoles));
  const hasPMEligibleMember = eligibleActiveMembers.length > 0 || draftPMEligible;
  const hasAnyMember = activeMembers.length > 0 || selectedDrafts.length > 0;
  const canContinueMembers = canEditMembers && hasAnyMember && invalidDrafts.length === 0 && hasPMEligibleMember;

  function defaultDraft(staff: ProjectMemberStaffOption): MemberDraft {
    const previous = inactiveByStaffID.get(staff.id);
    if (previous) {
      return {
        staff,
        role_ids: previous.role_ids,
        project_role_ids: previous.project_roles.map((role) => role.id),
        notes: previous.notes ?? '',
      };
    }
    const exactRoleLabels = new Map(roleLabels.map((label) => [normalizeRoleLabel(label), label]));
    const matchingLabels = staff.global_roles
      .map((role) => exactRoleLabels.get(normalizeRoleLabel(role.display_name)))
      .filter((label): label is string => Boolean(label));
    const sampleAccessRole = staff.is_sample && accessRoles.length === 1 ? accessRoles[0] : undefined;
    return {
      staff,
      role_ids: sampleAccessRole ? [sampleAccessRole.display_name] : Array.from(new Set(matchingLabels)),
      project_role_ids: sampleAccessRole ? [sampleAccessRole.id] : [],
      notes: '',
    };
  }

  function toggleStaff(staff: ProjectMemberStaffOption) {
    setSubmittedMembers(false);
    setDrafts((current) => {
      if (current[staff.id]) {
        const next = { ...current };
        delete next[staff.id];
        return next;
      }
      if (Object.keys(current).length >= 50) {
        toast.warn({ title: 'Đã đạt giới hạn', description: 'Mỗi lần có thể thêm tối đa 50 thành viên.' });
        return current;
      }
      return { ...current, [staff.id]: defaultDraft(staff) };
    });
  }

  function updateDraft(staffID: string, changes: Partial<MemberDraft>) {
    setSubmittedMembers(false);
    setDrafts((current) => current[staffID]
      ? { ...current, [staffID]: { ...current[staffID], ...changes } }
      : current);
  }

  async function submitProject(values: FormValues) {
    const normalizedValues = {
      ...values,
      partner_id: values.partner_id || null,
      pipeline_template_id: values.pipeline_template_id || null,
    };
    if (activeProjectID && editableProject) {
      try {
        const updated = onUpdateProject
          ? await onUpdateProject(activeProjectID, normalizedValues)
          : await put<Project>(`/projects/${activeProjectID}`, normalizedValues);
        setCreatedProject(updated);
        queryClient.setQueryData(['project', activeProjectID], updated);
        await Promise.all([
          readiness.refetch(),
          queryClient.invalidateQueries({ queryKey: ['projects'] }),
          queryClient.invalidateQueries({ queryKey: ['my-project-startup-actions'] }),
          queryClient.invalidateQueries({ queryKey: ['my-work-navigation-summary'] }),
          queryClient.invalidateQueries({ queryKey: ['project-explorer'] }),
        ]);
        if (!onUpdateProject) toast.success('Đã cập nhật thông tin công trình');
        onChanged();
        setStep(2);
      } catch (error) {
        if (!onUpdateProject) toast.error('Không cập nhật được công trình', error);
      }
      return;
    }
    if (!onCreateProject) return;
    const project = await onCreateProject(normalizedValues);
    setCreatedProject(project);
    setStep(2);
    onChanged();
  }

  function canNavigateBack(target: WizardStep) {
    if (target >= step) return false;
    if (target === 1) return Boolean(activeProjectID && canUpdateProject);
    return target === 2 && canEditMembers;
  }

  async function continueFromMembers() {
    setSubmittedMembers(true);
    if (!canContinueMembers) {
      const problems: string[] = [];
      if (!hasAnyMember) problems.push('chọn ít nhất một nhân sự');
      if (invalidDrafts.some((draft) => !draft.role_ids.length)) problems.push('gán vai trò vận hành cho mọi người đã chọn');
      if (invalidDrafts.some((draft) => !draftHasProjectView(draft, accessRoles))) problems.push('bảo đảm mỗi người có quyền project.view');
      if (!hasPMEligibleMember) problems.push('chọn ít nhất một người đủ quyền PM');
      toast.warn({ title: 'Chưa thể tiếp tục', description: problems.join('; ') || 'Anh không có quyền quản lý thành viên công trình.' });
      return;
    }
    try {
      if (selectedDrafts.length) {
        await saveMembers.mutateAsync(selectedDrafts);
        toast.success({ title: `Đã thêm ${selectedDrafts.length} thành viên`, description: 'Danh sách được lưu nguyên tử và đã sẵn sàng cho bước thiết lập.' });
      }
      setDrafts({});
      await Promise.all([members.refetch(), readiness.refetch()]);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['my-project-startup-actions'] }),
        queryClient.invalidateQueries({ queryKey: ['my-work-navigation-summary'] }),
      ]);
      onChanged();
      setStep(3);
    } catch (error) {
      if (error instanceof ApiError && error.code === 'VALIDATION_ERROR') {
        toast.warn({ title: 'Chưa thể lưu danh sách nhân sự', description: error.message });
        return;
      }
      toast.error('Không lưu được danh sách nhân sự', error);
    }
  }

  async function assignProjectManager() {
    if (!selectedPMStaffID) return;
    try {
      await setProjectManager.mutateAsync(selectedPMStaffID);
      await Promise.all([
        readiness.refetch(),
        members.refetch(),
        queryClient.invalidateQueries({ queryKey: ['my-project-startup-actions'] }),
      ]);
      onChanged();
      toast.success('Đã bàn giao trách nhiệm thiết lập cho PM');
    } catch (error) {
      if (error instanceof ApiError && error.code === 'VALIDATION_ERROR') {
        toast.warn({ title: 'Chưa thể chỉ định PM', description: error.message });
        return;
      }
      toast.error('Không chỉ định được PM', error);
    }
  }

  async function runPipeline() {
    try {
      await startPipeline.mutateAsync();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['project-startup-readiness', activeProjectID] }),
        queryClient.invalidateQueries({ queryKey: ['my-project-startup-actions'] }),
        queryClient.invalidateQueries({ queryKey: ['my-work-items'] }),
        queryClient.invalidateQueries({ queryKey: ['project-operations', activeProjectID] }),
        queryClient.invalidateQueries({ queryKey: ['pipeline-run', activeProjectID] }),
      ]);
      onChanged();
      toast.success('Đã bắt đầu quy trình và giao các công việc đầu tiên');
      onClose();
    } catch (error) {
      if (error instanceof ApiError && error.code === 'VALIDATION_ERROR') {
        toast.warn({ title: 'Chưa thể bắt đầu quy trình', description: error.message });
        return;
      }
      toast.error('Không bắt đầu được quy trình', error);
    }
  }

  const footer = step === 1 ? undefined : (
    <div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
      <div className="flex flex-col gap-2 sm:flex-row">
        {step === 2 && canNavigateBack(1) ? (
          <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setStep(1)}><ArrowLeft className="h-4 w-4" />Quay lại Thông tin</button>
        ) : null}
        {step === 3 && canNavigateBack(2) ? (
          <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setStep(2)}><ArrowLeft className="h-4 w-4" />Quay lại Chọn nhân sự</button>
        ) : null}
        {activeProjectID ? <Link className="inline-flex justify-center rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" to={`/projects/${activeProjectID}?tab=members`}>Quản lý thành viên</Link> : null}
      </div>
      <div className="flex justify-end gap-2">
        <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>
        {step === 2 ? (
          <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" disabled={saveMembers.isPending || !canContinueMembers} onClick={() => void continueFromMembers()}>{saveMembers.isPending ? 'Đang lưu…' : 'Lưu nhân sự & tiếp tục'}</button>
        ) : null}
        {step === 3 && readiness.data?.capabilities.can_start_pipeline ? (
          <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" disabled={!readiness.data.ready_to_start || startPipeline.isPending} onClick={() => void runPipeline()}>
            {startPipeline.isPending ? <RefreshCw className="h-4 w-4 animate-spin" /> : <PlayCircle className="h-4 w-4" />}
            {startPipeline.isPending ? 'Đang khởi động…' : 'Bắt đầu quy trình'}
          </button>
        ) : null}
      </div>
    </div>
  );

  return (
    <Dialog
      open={open}
      title={title}
      description="Hoàn tất thông tin, nhân sự và trách nhiệm trước khi tạo công việc thật."
      size="xl"
      onClose={onClose}
      footer={footer}
    >
      <div className="space-y-5">
        <WizardProgress step={step} canNavigateBack={canNavigateBack} onNavigate={setStep} />

        {step === 1 ? (
          projectFormCatalogs.isLoading || (Boolean(activeProjectID) && !editableProject && projectDetail.isLoading) ? <LoadingSpinner />
            : projectFormCatalogs.error ? <WizardError title="Không tải được danh mục tạo công trình" onRetry={() => void projectFormCatalogs.refetch()} />
              : projectDetail.error && !editableProject ? <WizardError title="Không tải được thông tin công trình" onRetry={() => void projectDetail.refetch()} />
                : editableProject || onCreateProject ? (
                  <SimpleForm
                    key={`${editableProject?.id ?? 'new-project'}-${editableProject?.updated_at ?? ''}`}
                    submitLabel={editableProject ? 'Lưu thay đổi & tiếp tục' : 'Tạo công trình & tiếp tục'}
                    cancelLabel="Đóng"
                    fields={buildProjectFormFields(editableProject, projectFormCatalogs.partners, projectFormCatalogs.templates)}
                    layout="two-column"
                    sections={[
                      { title: 'Thông tin công trình', fields: ['name', 'code', 'partner_id', 'client_name', 'address'] },
                      { title: 'Kế hoạch & giá trị', fields: ['planned_start_date', 'planned_end_date', 'estimated_budget', 'contract_amount'] },
                      { title: 'Quy trình', fields: ['pipeline_template_id'] },
                    ]}
                    onCancel={onClose}
                    onSubmit={submitProject}
                  />
                ) : <WizardError title="Không thể mở thông tin công trình" onRetry={() => void projectDetail.refetch()} />
        ) : null}

        {step === 2 ? (
          <MemberSelectionStep
            loading={members.isLoading}
            error={members.error}
            canManage={canEditMembers}
            search={search}
            onSearchChange={setSearch}
            activeMembers={savedActiveMembers}
            visibleStaffOptions={visibleStaffOptions}
            totalCandidateCount={staffOptions.filter((staff) => !staff.member_active).length}
            drafts={drafts}
            accessRoles={accessRoles}
            roleLabels={roleLabels}
            submitted={submittedMembers}
            hasPMEligibleMember={hasPMEligibleMember}
            onToggleStaff={toggleStaff}
            onUpdateDraft={updateDraft}
            onRetry={() => void members.refetch()}
          />
        ) : null}

        {step === 3 ? (
          <SetupStep
            loading={readiness.isLoading || members.isLoading}
            error={readiness.error || members.error}
            data={readiness.data}
            teamMembers={savedActiveMembers}
            eligibleMembers={eligibleActiveMembers}
            selectedPMStaffID={selectedPMStaffID}
            assigningPM={setProjectManager.isPending}
            onSelectedPMChange={setSelectedPMStaffID}
            onAssignPM={() => void assignProjectManager()}
            onRetry={() => { void readiness.refetch(); void members.refetch(); }}
          />
        ) : null}
      </div>
    </Dialog>
  );
}

function WizardProgress({ step, canNavigateBack, onNavigate }: { step: WizardStep; canNavigateBack: (step: WizardStep) => boolean; onNavigate: (step: WizardStep) => void }) {
  const steps: Array<{ id: WizardStep; label: string }> = [
    { id: 1, label: 'Thông tin' },
    { id: 2, label: 'Chọn nhân sự' },
    { id: 3, label: 'Thiết lập' },
  ];
  return (
    <ol className="grid grid-cols-3 overflow-hidden rounded-lg border border-line bg-slate-50" aria-label="Tiến trình thiết lập công trình">
      {steps.map((item) => {
        const done = item.id < step;
        const active = item.id === step;
        const navigable = canNavigateBack(item.id);
        const content = (
          <>
            <span className={clsx('grid h-6 w-6 shrink-0 place-items-center rounded-full border text-xs', active ? 'border-blue-600 bg-blue-600 text-white' : done ? 'border-emerald-500 bg-emerald-500 text-white' : 'border-slate-300 bg-white')}>{done ? <Check className="h-3.5 w-3.5" /> : item.id}</span>
            <span className="truncate">{item.label}</span>
          </>
        );
        return (
          <li key={item.id} aria-current={active ? 'step' : undefined} className={clsx('relative min-w-0 text-center text-xs font-semibold sm:text-sm', active ? 'bg-blue-50 text-blue-800' : done ? 'text-emerald-700' : 'text-slate-500')}>
            {navigable ? (
              <button type="button" className="flex w-full items-center justify-center gap-2 px-2 py-3 hover:bg-emerald-50 focus-ring" aria-label={`Quay lại bước ${item.label}`} onClick={() => onNavigate(item.id)}>{content}</button>
            ) : (
              <div className="flex items-center justify-center gap-2 px-2 py-3">{content}</div>
            )}
          </li>
        );
      })}
    </ol>
  );
}

function MemberSelectionStep({
  loading,
  error,
  canManage,
  search,
  onSearchChange,
  activeMembers,
  visibleStaffOptions,
  totalCandidateCount,
  drafts,
  accessRoles,
  roleLabels,
  submitted,
  hasPMEligibleMember,
  onToggleStaff,
  onUpdateDraft,
  onRetry,
}: {
  loading: boolean;
  error: unknown;
  canManage: boolean;
  search: string;
  onSearchChange: (value: string) => void;
  activeMembers: ProjectMember[];
  visibleStaffOptions: ProjectMemberStaffOption[];
  totalCandidateCount: number;
  drafts: Record<string, MemberDraft>;
  accessRoles: ProjectMemberAccessRole[];
  roleLabels: string[];
  submitted: boolean;
  hasPMEligibleMember: boolean;
  onToggleStaff: (staff: ProjectMemberStaffOption) => void;
  onUpdateDraft: (staffID: string, changes: Partial<MemberDraft>) => void;
  onRetry: () => void;
}) {
  const selectedDrafts = Object.values(drafts);
  if (loading) return <LoadingSpinner />;
  if (error) return <WizardError title="Không tải được danh sách nhân sự" onRetry={onRetry} />;
  if (!canManage) {
    return (
      <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
        Anh chỉ có thể xem thiết lập. Người có quyền `project.member.manage` cần hoàn tất bước chọn nhân sự.
      </div>
    );
  }
  return (
    <div className="space-y-4">
      <section>
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h3 className="flex items-center gap-2 font-semibold text-ink"><Users className="h-5 w-5 text-blue-700" />Chọn nhân sự tham gia công trình</h3>
            <p className="mt-1 text-sm text-slate-600">Tìm và chọn nhanh bằng checkbox. Thành viên đã lưu được khóa tại đây để tránh vô hiệu hóa ngoài ý muốn.</p>
          </div>
          <span className="shrink-0 rounded-md bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-800">Đã chọn mới: {selectedDrafts.length}/50</span>
        </div>
      </section>

      {activeMembers.length ? (
        <section className="rounded-lg border border-emerald-200 bg-emerald-50/50 p-3">
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-emerald-800">Đã là thành viên · khóa trong wizard</p>
          <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {activeMembers.map((member) => (
              <div key={member.id} className="flex items-start gap-2 rounded-md border border-emerald-200 bg-white px-3 py-2">
                <input type="checkbox" checked disabled className="mt-0.5 h-4 w-4 rounded border-slate-300" aria-label={`${member.full_name} đã là thành viên`} />
                <div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold text-slate-700">{member.full_name}</p><p className={clsx('truncate text-xs', member.account_active === false ? 'text-amber-700' : 'text-slate-500')}>{member.account_active === false ? 'Tài khoản không còn hoạt động · cần xử lý tại màn Thành viên' : member.department || member.position || 'Chưa có phòng ban'}</p></div>
                <LockKeyhole className="h-4 w-4 shrink-0 text-emerald-600" />
              </div>
            ))}
          </div>
        </section>
      ) : null}

      <section className="overflow-hidden rounded-lg border border-line">
        <div className="border-b border-line bg-slate-50 p-3">
          <label className="relative block">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input className="w-full rounded-md border border-line bg-white py-2 pl-9 pr-3 text-sm focus-ring" value={search} onChange={(event) => onSearchChange(event.target.value)} placeholder="Tìm tên, email, phòng ban, chức vụ, vai trò…" autoFocus />
          </label>
        </div>
        {!totalCandidateCount ? (
          <p className="p-6 text-center text-sm text-slate-500">Không còn nhân sự hoạt động phù hợp để chọn.</p>
        ) : !visibleStaffOptions.length ? (
          <p className="p-6 text-center text-sm text-slate-500">Không tìm thấy nhân sự phù hợp với từ khóa.</p>
        ) : (
          <div className="max-h-72 divide-y divide-line overflow-y-auto">
            {visibleStaffOptions.map((staff) => {
              const selected = Boolean(drafts[staff.id]);
              return (
                <label key={staff.id} className={clsx('grid cursor-pointer grid-cols-[24px_minmax(0,1fr)] gap-2 px-3 py-3 hover:bg-slate-50 sm:grid-cols-[24px_minmax(0,1.2fr)_minmax(0,1fr)_auto]', selected && 'bg-blue-50')}>
                  <input type="checkbox" className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600" checked={selected} onChange={() => onToggleStaff(staff)} />
                  <span className="min-w-0"><span className="block truncate text-sm font-semibold text-slate-700">{staff.full_name}</span><span className="block truncate text-xs text-slate-500">{staff.email || 'Chưa có email'}</span></span>
                  <span className="hidden min-w-0 sm:block"><span className="block truncate text-sm text-slate-700">{staff.department || 'Chưa có phòng ban'}</span><span className="block truncate text-xs text-slate-500">{staff.position || 'Chưa có chức vụ'}</span></span>
                  <span className="hidden rounded-md bg-slate-100 px-2 py-1 text-xs font-medium text-slate-600 sm:inline-flex">{staff.member_id ? 'Kích hoạt lại' : staff.global_roles.map((role) => role.display_name).join(', ') || 'Chưa có role global'}</span>
                </label>
              );
            })}
          </div>
        )}
      </section>

      {selectedDrafts.length ? (
        <section className="space-y-3">
          <div><h3 className="font-semibold text-ink">Cấu hình người đã chọn</h3><p className="mt-1 text-sm text-slate-600">Mỗi người cần ít nhất một vai trò vận hành và quyền xem công trình.</p></div>
          {selectedDrafts.map((draft) => (
            <MemberDraftCard key={draft.staff.id} draft={draft} accessRoles={accessRoles} roleLabels={roleLabels} submitted={submitted} onChange={(changes) => onUpdateDraft(draft.staff.id, changes)} />
          ))}
        </section>
      ) : null}

      <section className={clsx('rounded-lg border p-3 text-sm', hasPMEligibleMember ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : 'border-amber-200 bg-amber-50 text-amber-900')}>
        <div className="flex items-start gap-2">{hasPMEligibleMember ? <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" /> : <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />}<p>{hasPMEligibleMember ? 'Đã có ít nhất một người đủ quyền PM: xem công trình, quản lý thành viên và chạy pipeline.' : 'Cần ít nhất một người có đủ `project.view`, `project.member.manage` và `pipeline.run` trước khi tiếp tục.'}</p></div>
      </section>
    </div>
  );
}

function MemberDraftCard({ draft, accessRoles, roleLabels, submitted, onChange }: { draft: MemberDraft; accessRoles: ProjectMemberAccessRole[]; roleLabels: string[]; submitted: boolean; onChange: (changes: Partial<MemberDraft>) => void }) {
  const accessReady = draftHasProjectView(draft, accessRoles);
  const pmReady = draftCanBeProjectManager(draft, accessRoles);
  return (
    <article className={clsx('rounded-lg border p-4', submitted && (!draft.role_ids.length || !accessReady) ? 'border-rose-300 bg-rose-50/30' : 'border-line bg-white')}>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div><p className="font-semibold text-ink">{draft.staff.full_name}</p><p className="mt-0.5 text-xs text-slate-500">{[draft.staff.department, draft.staff.position].filter(Boolean).join(' · ') || 'Chưa có đơn vị công tác'}</p></div>
        <span className={clsx('w-fit rounded-md px-2 py-1 text-xs font-semibold', pmReady ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600')}>{pmReady ? 'Đủ quyền PM' : 'Thành viên'}</span>
      </div>
      <div className="mt-4 grid gap-4 lg:grid-cols-2">
        <div>
          <p className="mb-2 text-sm font-semibold text-slate-700">Vai trò vận hành <span className="text-rose-600">*</span></p>
          <div className="grid max-h-48 gap-2 overflow-y-auto rounded-md border border-line p-2 sm:grid-cols-2">
            {roleLabels.map((label) => <RoleCheckbox key={label} label={label} checked={draft.role_ids.includes(label)} onChange={() => onChange({ role_ids: toggleValue(draft.role_ids, label) })} />)}
          </div>
          {submitted && !draft.role_ids.length ? <p className="mt-1 text-xs font-medium text-rose-700">Chọn ít nhất một vai trò vận hành.</p> : null}
        </div>
        <div>
          <p className="mb-2 text-sm font-semibold text-slate-700">Role phân quyền theo công trình</p>
          <div className="grid max-h-48 gap-2 overflow-y-auto rounded-md border border-line p-2">
            {accessRoles.map((role) => <RoleCheckbox key={role.id} label={role.display_name} description={role.description} checked={draft.project_role_ids.includes(role.id)} onChange={() => onChange({ project_role_ids: toggleValue(draft.project_role_ids, role.id) })} />)}
          </div>
          {draft.staff.global_roles.length ? <p className="mt-1 text-xs text-slate-500">Role global: {draft.staff.global_roles.map((role) => role.display_name).join(', ')}.</p> : null}
          {submitted && !accessReady ? <p className="mt-1 text-xs font-medium text-rose-700">Cần role global hoặc role công trình cấp `project.view`.</p> : null}
        </div>
      </div>
    </article>
  );
}

function RoleCheckbox({ label, description, checked, onChange }: { label: string; description?: string; checked: boolean; onChange: () => void }) {
  return (
    <label className={clsx('flex cursor-pointer items-start gap-2 rounded-md border px-3 py-2', checked ? 'border-blue-200 bg-blue-50' : 'border-line bg-white hover:bg-slate-50')}>
      <input type="checkbox" className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600" checked={checked} onChange={onChange} />
      <span className="min-w-0"><span className="block text-sm font-semibold text-slate-700">{label}</span>{description ? <span className="mt-0.5 block text-xs leading-4 text-slate-500">{description}</span> : null}</span>
    </label>
  );
}

function SetupStep({ loading, error, data, teamMembers, eligibleMembers, selectedPMStaffID, assigningPM, onSelectedPMChange, onAssignPM, onRetry }: {
  loading: boolean;
  error: unknown;
  data?: ProjectStartupReadiness;
  teamMembers: ProjectMember[];
  eligibleMembers: ProjectMember[];
  selectedPMStaffID: string;
  assigningPM: boolean;
  onSelectedPMChange: (value: string) => void;
  onAssignPM: () => void;
  onRetry: () => void;
}) {
  if (loading) return <LoadingSpinner />;
  if (error || !data) return <WizardError title="Không tải được trạng thái khởi tạo công trình" onRetry={onRetry} />;
  const roleCoverage = data.role_coverage ?? [];
  const coveredRoleCount = roleCoverage.filter((role) => role.covered).length;
  return (
    <div className="space-y-4">
      <section className="grid gap-3 sm:grid-cols-2">
        <div className="rounded-lg border border-line bg-slate-50 p-4"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Người chịu trách nhiệm</p><p className="mt-2 font-semibold text-ink">{data.owner_name || 'Chưa xác định'}</p><p className="mt-1 text-xs text-slate-600">{data.pm_user_id ? 'PM công trình' : 'Người tạo công trình (tạm thời)'}</p></div>
        <div className="rounded-lg border border-line bg-slate-50 p-4"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Mẫu quy trình</p><p className="mt-2 font-semibold text-ink">{data.template_name || 'Chưa có mẫu đã phát hành'}</p><p className="mt-1 text-xs text-slate-600">{data.template_id ? 'Sẵn sàng kiểm tra bước đầu' : 'Cần chọn hoặc phát hành mẫu quy trình'}</p></div>
      </section>

      <div className="grid gap-4 xl:grid-cols-2">
        <section className="min-w-0 rounded-lg border border-line p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <h3 className="flex items-center gap-2 font-semibold text-ink"><Users className="h-5 w-5 text-blue-700" />Đội ngũ công trình</h3>
              <p className="mt-1 text-sm text-slate-600">Toàn bộ thành viên đã lưu và vai trò vận hành của họ.</p>
            </div>
            <span className="shrink-0 rounded-full bg-slate-100 px-2.5 py-1 text-xs font-semibold text-slate-700">{teamMembers.length} người</span>
          </div>
          {teamMembers.length ? (
            <div className="mt-3 max-h-72 divide-y divide-line overflow-y-auto rounded-md border border-line bg-white">
              {teamMembers.map((member) => (
                <div key={member.id} className="grid gap-2 p-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.25fr)_auto] sm:items-center">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-ink" title={member.full_name}>{member.full_name}</p>
                    <p className="truncate text-xs text-slate-500" title={[member.department, member.position].filter(Boolean).join(' · ')}>{[member.department, member.position].filter(Boolean).join(' · ') || member.email || 'Chưa có thông tin đơn vị'}</p>
                  </div>
                  <div className="flex min-w-0 flex-wrap gap-1.5">
                    {member.role_ids.length ? member.role_ids.map((role) => <span key={role} className="rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">{role}</span>) : <span className="text-xs text-amber-700">Chưa có vai trò vận hành</span>}
                  </div>
                  <div className="flex flex-wrap items-center gap-1.5 sm:justify-end">
                    {member.staff_id === data.pm_staff_id ? <span className="rounded-full bg-indigo-50 px-2 py-0.5 text-xs font-semibold text-indigo-700">PM phụ trách</span> : null}
                    <span className={clsx('rounded-full px-2 py-0.5 text-xs font-semibold', member.account_active === false ? 'bg-amber-50 text-amber-800' : 'bg-emerald-50 text-emerald-700')}>{member.account_active === false ? 'Tài khoản ngừng' : 'Đang hoạt động'}</span>
                  </div>
                </div>
              ))}
            </div>
          ) : <p className="mt-3 rounded-md border border-dashed border-line p-4 text-sm text-slate-600">Chưa có thành viên được lưu cho công trình.</p>}
        </section>

        <section className="min-w-0 rounded-lg border border-line p-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <h3 className="flex items-center gap-2 font-semibold text-ink"><CheckCircle2 className="h-5 w-5 text-blue-700" />Độ phủ vai trò quy trình</h3>
              <p className="mt-1 text-sm text-slate-600">Tổng hợp vai trò từ toàn bộ giai đoạn và bước của mẫu quy trình.</p>
            </div>
            {roleCoverage.length ? <span className={clsx('shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold', coveredRoleCount === roleCoverage.length ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-800')}>{coveredRoleCount}/{roleCoverage.length} đã có người</span> : null}
          </div>
          {roleCoverage.length ? (
            <div className="mt-3 max-h-72 space-y-2 overflow-y-auto pr-1">
              {roleCoverage.map((role) => {
                const roleMembers = Array.isArray(role.members) ? role.members : [];
                return (
                  <div key={normalizeRoleLabel(role.role_label)} className={clsx('rounded-md border p-3', role.covered ? 'border-emerald-200 bg-emerald-50/60' : 'border-amber-200 bg-amber-50/70')}>
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="text-sm font-semibold text-ink">{role.role_label}</p>
                        <p className="mt-0.5 text-xs text-slate-500">{role.node_count} giai đoạn · {role.step_count} bước</p>
                      </div>
                      <span className={clsx('shrink-0 rounded-full px-2 py-0.5 text-xs font-semibold', role.covered ? 'bg-white/80 text-emerald-700' : 'bg-white/80 text-amber-800')}>{role.covered ? 'Đã có người' : 'Còn thiếu'}</span>
                    </div>
                    <p className={clsx('mt-2 text-xs', role.covered ? 'text-slate-700' : 'font-medium text-amber-900')}>{roleMembers.length ? roleMembers.map((member) => member.full_name).join(', ') : 'Chưa có thành viên hoạt động mang vai trò này.'}</p>
                  </div>
                );
              })}
            </div>
          ) : <p className="mt-3 rounded-md border border-dashed border-line p-4 text-sm text-slate-600">Mẫu quy trình chưa khai báo vai trò vận hành.</p>}
        </section>
      </div>

      {data.capabilities.can_manage_project_manager ? (
        <section className="rounded-lg border border-line p-4">
          <h3 className="flex items-center gap-2 font-semibold text-ink"><UserCog className="h-5 w-5 text-blue-700" />Chỉ định PM chịu trách nhiệm</h3>
          <p className="mt-1 text-sm text-slate-600">Danh sách chỉ hiển thị thành viên có đủ quyền xem công trình, quản lý thành viên và chạy pipeline.</p>
          <div className="mt-3 flex flex-col gap-2 sm:flex-row">
            <select className="min-w-0 flex-1 rounded-md border border-line bg-white px-3 py-2 text-sm focus-ring" value={selectedPMStaffID} onChange={(event) => onSelectedPMChange(event.target.value)}>
              <option value="">Chọn PM đủ điều kiện</option>
              {eligibleMembers.map((member) => <option key={member.id} value={member.staff_id}>{member.full_name} · {member.role_ids.join(', ')}</option>)}
            </select>
            <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" disabled={!selectedPMStaffID || selectedPMStaffID === data.pm_staff_id || assigningPM} onClick={onAssignPM}>{assigningPM ? 'Đang lưu…' : 'Bàn giao cho PM'}</button>
          </div>
          {!eligibleMembers.length ? <p className="mt-2 text-sm text-amber-800">Chưa có thành viên đủ quyền PM. Quay lại bước Chọn nhân sự để bổ sung role.</p> : null}
        </section>
      ) : null}

      <section className="rounded-lg border border-line p-4">
        <h3 className="font-semibold text-ink">Người thực hiện bước đầu</h3>
        <div className="mt-3 space-y-2">
          {data.first_steps.length ? data.first_steps.map((startupStep) => (
            <div key={startupStep.id} className={clsx('rounded-md border p-3', startupStep.covered ? 'border-emerald-200 bg-emerald-50' : 'border-amber-200 bg-amber-50')}>
              <div className="flex items-start justify-between gap-3"><div><p className="text-sm font-semibold text-ink">{startupStep.name}</p><p className="mt-1 text-xs text-slate-600">Vai trò: {startupStep.role_ids.join(', ') || 'Không giới hạn'}{startupStep.required_permission ? ` · Quyền: ${startupStep.required_permission}` : ''}</p></div><span className={clsx('shrink-0 text-xs font-semibold', startupStep.covered ? 'text-emerald-700' : 'text-amber-800')}>{startupStep.covered ? 'Đã có người' : 'Thiếu người'}</span></div>
              <p className="mt-2 text-xs text-slate-700">{startupStep.eligible_users.length ? startupStep.eligible_users.map((person) => person.full_name).join(', ') : 'Chưa có thành viên phù hợp.'}</p>
            </div>
          )) : <p className="text-sm text-slate-600">Node đầu không có bước tạo công việc.</p>}
        </div>
      </section>

      <section className={clsx('rounded-lg border p-4', data.ready_to_start ? 'border-emerald-200 bg-emerald-50' : 'border-amber-200 bg-amber-50')}>
        <h3 className={clsx('font-semibold', data.ready_to_start ? 'text-emerald-800' : 'text-amber-900')}>{data.ready_to_start ? 'Đã sẵn sàng bắt đầu' : `Còn ${data.blockers.length} điều kiện cần xử lý`}</h3>
        {!data.ready_to_start ? <ul className="mt-2 space-y-1 text-sm text-amber-900">{data.blockers.map((blocker, index) => <li key={`${blocker.key}-${blocker.step_id ?? index}`}>• {blocker.label}</li>)}</ul> : <p className="mt-1 text-sm text-emerald-800">Khi bắt đầu, hệ thống mới tạo Work Item thật và thông báo đúng nhóm vai trò.</p>}
      </section>
    </div>
  );
}

function WizardError({ title, onRetry }: { title: string; onRetry: () => void }) {
  return <div className="rounded-md border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700"><p className="font-semibold">{title}</p><button type="button" className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 font-semibold" onClick={onRetry}>Thử lại</button></div>;
}

function memberCanBeProjectManager(member: ProjectMember) {
  return rolesGrantProjectManager([...member.global_roles, ...member.project_roles]);
}

function draftCanBeProjectManager(draft: MemberDraft, accessRoles: ProjectMemberAccessRole[]) {
  return rolesGrantProjectManager([
    ...draft.staff.global_roles,
    ...accessRoles.filter((role) => draft.project_role_ids.includes(role.id)),
  ]);
}

function draftHasProjectView(draft: MemberDraft, accessRoles: ProjectMemberAccessRole[]) {
  return [...draft.staff.global_roles, ...accessRoles.filter((role) => draft.project_role_ids.includes(role.id))]
    .some((role) => role.grants_project_view);
}

function rolesGrantProjectManager(roles: ProjectMemberAccessRole[]) {
  return roles.some((role) => role.grants_project_view)
    && roles.some((role) => role.grants_project_member_manage)
    && roles.some((role) => role.grants_pipeline_run);
}

function normalizeMemberSearch(value: string) {
  return value.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLocaleLowerCase('vi-VN').trim();
}

function normalizeRoleLabel(value: string) {
  return normalizeMemberSearch(value).replace(/\s+/g, ' ');
}

function toggleValue(values: string[], value: string) {
  return values.includes(value) ? values.filter((item) => item !== value) : [...values, value];
}
