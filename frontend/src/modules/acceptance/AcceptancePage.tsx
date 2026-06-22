import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Plus, RotateCcw, X } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { FormDrawer } from '../../components/FormDrawer';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';
import { dateText } from '../../lib/utils/format';
import type { AcceptanceRecord } from '../../lib/types/domain';

const acceptanceActionLabels: Record<string, string> = {
  pass: 'Đã pass nghiệm thu',
  'conditional-pass': 'Đã conditional pass nghiệm thu',
  fail: 'Đã fail nghiệm thu',
  'create-rework': 'Đã tạo rework từ nghiệm thu',
};

export function AcceptancePage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<AcceptanceRecord | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const acceptances = useQuery({ queryKey: ['acceptances', projectId], queryFn: () => get<AcceptanceRecord[]>(`/projects/${projectId}/acceptances`), enabled: Boolean(projectId) });
  const detail = useQuery({
    queryKey: ['acceptance', selected?.id],
    queryFn: () => get<AcceptanceRecord>(`/acceptances/${selected?.id}`),
    enabled: Boolean(selected?.id),
    retry: false,
  });
  const invalidate = (acceptanceId?: string) => {
    queryClient.invalidateQueries({ queryKey: ['acceptances', projectId] });
    if (acceptanceId) queryClient.invalidateQueries({ queryKey: ['acceptance', acceptanceId] });
  };
  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/acceptances`, values),
    onSuccess: () => {
      invalidate();
      setOpen(false);
      toast.success('Đã tạo nghiệm thu');
    },
    onError: (error) => toast.error('Không tạo được nghiệm thu', error),
  });
  const action = useMutation({
    mutationFn: ({ acceptanceId, name }: { acceptanceId: string; name: string }) => post(`/acceptances/${acceptanceId}/${name}`, {}),
    onSuccess: (_data, variables) => {
      invalidate(variables.acceptanceId);
      toast.success(acceptanceActionLabels[variables.name] ?? 'Đã cập nhật nghiệm thu');
    },
    onError: (error) => toast.error('Không cập nhật được nghiệm thu', error),
  });
  const selectedRecord = detail.data ?? selected;

  return (
    <>
      <PageHeader
        title="Nghiệm thu"
        description="Tạo nghiệm thu, checklist, pass/fail, punch list, rework request và re-nghiệm thu."
        actions={
          <>
            {!global ? <ProjectBackLink projectId={projectId} /> : null}
            {global ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Plus className="h-4 w-4" />Nghiệm thu</button>
          </>
        }
      />
      <DataTable
        rows={acceptances.data ?? []}
        loading={acceptances.isLoading}
        error={acceptances.error}
        onRetry={() => acceptances.refetch()}
        searchPlaceholder="Tìm nghiệm thu"
        defaultSort={{ key: 'updated_at', direction: 'desc' }}
        columns={[
        { key: 'title', header: 'Biên bản', sortable: true, render: (row) => (
          <div className="min-w-0">
            <p className="font-semibold text-ink">{row.acceptance_code ?? row.id} · {row.title}</p>
            <p className="mt-1 text-xs text-slate-500">{row.work_item_title ?? 'Không gắn work item'}</p>
          </div>
        ) },
        { key: 'type', header: 'Loại', render: (row) => <EnumChip group="acceptance_type" value={row.type} /> },
        { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        { key: 'result', header: 'Kết quả', render: (row) => <StatusBadge status={row.result ?? 'pending'} /> },
        { key: 'signature_status', header: 'Chữ ký', render: (row) => <StatusBadge status={row.signature_status ?? 'pending'} /> },
        { key: 'document_status', header: 'PDF', render: (row) => <StatusBadge status={row.document_status ?? 'pending'} /> },
        { key: 'checklist_count', header: 'Checklist', align: 'right', sortValue: (row) => Number(row.checklist_count ?? 0), render: (row) => Number(row.checklist_count ?? 0) },
        { key: 'updated_at', header: 'Cập nhật', sortable: true, sortValue: (row) => row.updated_at ?? row.created_at ?? '', render: (row) => dateText(row.updated_at ?? row.created_at) },
        { key: 'action', header: 'Action', render: (row) => (
          <RowActions onView={() => setSelected(row)}>
            <button title="Pass" disabled={row.status !== 'submitted' || action.isPending} className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => action.mutate({ acceptanceId: row.id, name: 'pass' })}><Check className="h-4 w-4" /></button>
            <button title="Conditional pass" disabled={row.status !== 'submitted' || action.isPending} className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => action.mutate({ acceptanceId: row.id, name: 'conditional-pass' })}><RotateCcw className="h-4 w-4" /></button>
            <button title="Fail" disabled={row.status !== 'submitted' || action.isPending} className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => action.mutate({ acceptanceId: row.id, name: 'fail' })}><X className="h-4 w-4" /></button>
            <button title="Tạo rework" disabled={row.status !== 'completed' || row.result !== 'fail' || action.isPending} className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => action.mutate({ acceptanceId: row.id, name: 'create-rework' })}><RotateCcw className="h-4 w-4" /></button>
          </RowActions>
        ) },
      ]} />
      <FormDrawer open={open} title="Tạo nghiệm thu" onClose={() => setOpen(false)}>
        <SimpleForm layout="two-column" fields={[
          { name: 'type', label: 'Loại', type: 'select', defaultValue: 'partial', required: true, options: enumOptions('acceptance_type', ['partial', 'final']) },
          { name: 'title', label: 'Tiêu đề', defaultValue: 'Nghiệm thu hạng mục mới', required: true },
          { name: 'acceptance_date', label: 'Ngày nghiệm thu', type: 'date' },
          { name: 'location', label: 'Khu vực' },
          { name: 'client_representative', label: 'Đại diện CĐT' },
          { name: 'contractor_representative', label: 'Đại diện nhà thầu' },
          { name: 'signature_status', label: 'Chữ ký', type: 'select', defaultValue: 'pending', options: enumOptions('status', ['pending', 'signed', 'waived']) },
          { name: 'document_status', label: 'Tài liệu/PDF', type: 'select', defaultValue: 'pending', options: enumOptions('status', ['pending', 'linked', 'waived']) },
          { name: 'scope_summary', label: 'Phạm vi nghiệm thu', type: 'textarea' },
          { name: 'notes', label: 'Ghi chú', type: 'textarea' },
        ]} onSubmit={(values) => create.mutateAsync(values)} />
      </FormDrawer>
      <RecordDetailDialog
        open={Boolean(selected)}
        title={detail.isFetching ? 'Đang tải chi tiết nghiệm thu' : 'Chi tiết nghiệm thu'}
        record={selectedRecord}
        onClose={() => setSelected(null)}
        fields={[
          { key: 'acceptance_code', label: 'Mã hồ sơ' },
          { key: 'title', label: 'Biên bản' },
          { key: 'type', label: 'Loại', render: (row) => <EnumChip group="acceptance_type" value={row.type} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'result', label: 'Kết quả', render: (row) => <StatusBadge status={row.result ?? 'pending'} /> },
          { key: 'signature_status', label: 'Chữ ký', render: (row) => <StatusBadge status={row.signature_status ?? 'pending'} /> },
          { key: 'document_status', label: 'Tài liệu/PDF', render: (row) => <StatusBadge status={row.document_status ?? 'pending'} /> },
          { key: 'acceptance_date', label: 'Ngày nghiệm thu', render: (row) => dateText(row.acceptance_date) },
          { key: 'work_item_title', label: 'Work item' },
          { key: 'workflow_form_title', label: 'Workflow form' },
          { key: 'pdf_file_name', label: 'PDF' },
          { key: 'client_representative', label: 'Đại diện CĐT' },
          { key: 'contractor_representative', label: 'Đại diện nhà thầu' },
          { key: 'scope_summary', label: 'Phạm vi' },
          { key: 'summary_json', label: 'Tổng hợp' },
          { key: 'checklist', label: 'Checklist', render: (row) => (
            row.checklist?.length ? (
              <div className="space-y-2">
                {row.checklist.map((item, index) => (
                  <div key={item.id ?? index} className="rounded-md border border-line bg-white px-2 py-1.5">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <span className="font-semibold">{index + 1}. {item.title}</span>
                      <StatusBadge status={item.decision} />
                    </div>
                    {item.required_action ? <p className="mt-1 text-xs text-slate-600">{item.required_action}</p> : null}
                    {item.evidence_name ? <p className="mt-1 text-xs text-slate-500">Evidence: {item.evidence_name}</p> : null}
                  </div>
                ))}
              </div>
            ) : '-'
          ) },
          { key: 'notes', label: 'Ghi chú' },
          { key: 'created_at', label: 'Ngày tạo' },
        ]}
      />
    </>
  );
}
