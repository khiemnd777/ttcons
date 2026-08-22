import { useEffect, useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import {
  CheckCircle2,
  ClipboardList,
  Download,
  Eye,
  FilePlus2,
  MessageSquareWarning,
  RefreshCw,
  Search,
} from 'lucide-react';
import clsx from 'clsx';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { dateText } from '../../lib/utils/format';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import { constructionReportViewModel } from '../operations/constructionReportViewModel';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';

type WorkReportStatus = 'draft' | 'recorded' | 'submitted' | 'needs_changes' | 'acknowledged';
type WorkReportScope = 'mine' | 'review' | 'all';
type ExportFormat = 'pdf' | 'xlsx' | 'csv';

type WorkReportRow = Record<string, any> & {
  source_id: string;
  work_report_id?: string;
  project_id: string;
  project_code: string;
  project_name: string;
  report_date: string;
  author_id: string;
  author_name: string;
  author_account_deleted?: boolean;
  recipient_id?: string;
  recipient_name?: string;
  recipient_account_deleted?: boolean;
  status: WorkReportStatus;
  current_revision_no: number;
  submitted_at?: string;
};

type WorkReportDetail = WorkReportRow & {
  report_delivery_eligible: boolean;
  work_item_title?: string;
  weather?: string;
  progress_percent?: number;
  structured_json?: Record<string, unknown>;
  work_summary?: string;
  manpower_summary?: string;
  material_usage_summary?: string;
  issues_summary?: string;
  revisions?: Array<Record<string, any>>;
  events?: Array<Record<string, any>>;
};

type Recipient = { id: string; full_name: string; position?: string };

const statusOptions: Array<[string, string]> = [
  ['', 'Tất cả trạng thái'],
  ['draft', 'Bản nháp'],
  ['submitted', 'Chờ xác nhận'],
  ['needs_changes', 'Yêu cầu bổ sung'],
  ['acknowledged', 'Đã xác nhận'],
  ['recorded', 'Nhật ký lịch sử'],
];

export function WorkReportsPage() {
  const { user, hasPermission } = useAuth();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const canReview = hasPermission('work_report.review') || hasPermission('work_report.manage');
  const canManage = hasPermission('work_report.manage');
  const canCreate = hasPermission('work_report.create') && hasPermission('operation.report') && hasPermission('operation.view');
  const canExport = hasPermission('work_report.export');
  const availableScopes = useMemo<Array<{ key: WorkReportScope; label: string }>>(() => [
    { key: 'mine', label: 'Báo cáo của tôi' },
    ...(canReview ? [{ key: 'review' as const, label: 'Chờ tôi xem' }] : []),
    ...(canManage ? [{ key: 'all' as const, label: 'Tất cả được phép xem' }] : []),
  ], [canManage, canReview]);
  const [scope, setScope] = useState<WorkReportScope>('mine');
  const [draftFilters, setDraftFilters] = useState({ projectId: '', status: '', dateFrom: '', dateTo: '', search: '' });
  const [filters, setFilters] = useState(draftFilters);
  const [selectedSourceID, setSelectedSourceID] = useState('');
  const [exportFormat, setExportFormat] = useState<ExportFormat>('xlsx');
  const [exporting, setExporting] = useState(false);

  const listQueryString = useMemo(() => {
    const params = new URLSearchParams({ scope });
    if (filters.projectId) params.set('project_id', filters.projectId);
    if (filters.status) params.set('status', filters.status);
    if (filters.dateFrom) params.set('date_from', filters.dateFrom);
    if (filters.dateTo) params.set('date_to', filters.dateTo);
    if (filters.search) params.set('search', filters.search.trim());
    return params.toString();
  }, [filters, scope]);

  const reports = useQuery({
    queryKey: ['work-reports', scope, filters],
    queryFn: () => get<WorkReportRow[]>(`/work-reports?${listQueryString}`),
    retry: false,
  });

  const detail = useQuery({
    queryKey: ['work-report-detail', selectedSourceID],
    queryFn: () => get<WorkReportDetail>(`/work-reports/daily-reports/${selectedSourceID}`),
    enabled: Boolean(selectedSourceID),
    retry: false,
  });

  function applyFilters() {
    if (draftFilters.dateFrom && draftFilters.dateTo && draftFilters.dateFrom > draftFilters.dateTo) {
      toast.warn({ title: 'Khoảng ngày chưa hợp lệ', description: 'Ngày bắt đầu phải trước hoặc bằng ngày kết thúc.' });
      return;
    }
    setFilters({ ...draftFilters, search: draftFilters.search.trim() });
  }

  function resetFilters() {
    const empty = { projectId: '', status: '', dateFrom: '', dateTo: '', search: '' };
    setDraftFilters(empty);
    setFilters(empty);
  }

  async function exportRegister() {
    setExporting(true);
    try {
      const params = new URLSearchParams(listQueryString);
      params.set('format', exportFormat);
      const blob = await downloadBlob(`/work-reports/export?${params.toString()}`);
      saveBlob(blob, `so-dang-ky-bao-cao.${exportFormat}`);
      toast.success(`Đã xuất sổ báo cáo dạng ${exportLabel(exportFormat)}`);
    } catch (error) {
      toast.error('Không xuất được sổ báo cáo', error);
    } finally {
      setExporting(false);
    }
  }

  const rows = reports.data ?? [];

  return (
    <div className="space-y-5">
      <header className="border-b border-line pb-5">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">Trao đổi công việc có xác nhận</p>
            <h1 className="mt-1 text-2xl font-semibold text-ink">Báo cáo</h1>
            <p className="mt-1 max-w-3xl text-sm leading-6 text-slate-600">
              Nhân sự lập báo cáo từ dữ liệu công việc thực tế, gửi đúng người quản lý và lưu lịch sử từng lần phản hồi.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {canExport ? (
              <ExportControl format={exportFormat} exporting={exporting} onFormatChange={setExportFormat} onExport={() => void exportRegister()} />
            ) : null}
            {canCreate ? (
              <button
                type="button"
                className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
                onClick={() => navigate('/operations?tab=reports')}
              >
                <FilePlus2 className="h-4 w-4" />
                Lập báo cáo tại Thi công
              </button>
            ) : null}
          </div>
        </div>
      </header>

      <section className="rounded-lg border border-line bg-white p-4 shadow-panel" aria-label="Bộ lọc báo cáo">
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-[minmax(15rem,1.4fr)_minmax(10rem,1fr)_9rem_9rem_minmax(14rem,1.2fr)_auto] xl:items-end">
          <AsyncProjectPicker
            value={draftFilters.projectId}
            onChange={(projectId) => setDraftFilters((current) => ({ ...current, projectId }))}
            endpoint="/work-reports/projects"
            queryKey="work-report-project-options"
            emptyLabel="Tất cả công trình được phép xem"
          />
          <FilterSelect label="Trạng thái" value={draftFilters.status} options={statusOptions} onChange={(status) => setDraftFilters((current) => ({ ...current, status }))} />
          <FilterInput type="date" label="Từ ngày" value={draftFilters.dateFrom} onChange={(dateFrom) => setDraftFilters((current) => ({ ...current, dateFrom }))} />
          <FilterInput type="date" label="Đến ngày" value={draftFilters.dateTo} onChange={(dateTo) => setDraftFilters((current) => ({ ...current, dateTo }))} />
          <FilterInput label="Tìm kiếm" placeholder="Công trình, người lập, nội dung..." value={draftFilters.search} onChange={(search) => setDraftFilters((current) => ({ ...current, search }))} onEnter={applyFilters} />
          <div className="flex gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={resetFilters}>Đặt lại</button>
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={applyFilters}>
              <Search className="h-4 w-4" /> Áp dụng
            </button>
          </div>
        </div>
      </section>

      <section>
        <div className="mb-3 flex gap-1 overflow-x-auto border-b border-line" role="tablist" aria-label="Phạm vi báo cáo">
          {availableScopes.map((item) => (
            <button
              key={item.key}
              type="button"
              role="tab"
              aria-selected={scope === item.key}
              className={clsx('whitespace-nowrap border-b-2 px-3 py-2 text-sm font-semibold', scope === item.key ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink')}
              onClick={() => setScope(item.key)}
            >
              {item.label}
            </button>
          ))}
        </div>

        <DataTable<WorkReportRow>
          rows={rows}
          loading={reports.isLoading}
          error={reports.error}
          onRetry={() => void reports.refetch()}
          empty={scope === 'review' ? 'Không có báo cáo đang chờ anh xem' : 'Chưa có báo cáo trong phạm vi này'}
          emptyAction={canCreate && scope === 'mine' ? (
            <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => navigate('/operations?tab=reports')}>Lập báo cáo tại Thi công</button>
          ) : undefined}
          pageSize={10}
          defaultSort={{ key: 'report_date', direction: 'desc' }}
          getRowKey={(row) => row.source_id}
          renderMobileCard={(row) => <WorkReportCard row={row} onOpen={() => setSelectedSourceID(row.source_id)} />}
          columns={[
            { key: 'action', header: '', action: true, render: (row) => <button type="button" className="inline-flex items-center gap-1 text-sm font-semibold text-blue-700 hover:text-blue-900" onClick={() => setSelectedSourceID(row.source_id)}><Eye className="h-4 w-4" /><span className="sr-only">Xem </span>Chi tiết</button> },
            { key: 'report_date', header: 'Ngày báo cáo', sortable: true, render: (row) => <span className="font-semibold text-ink">{dateText(row.report_date)}</span> },
            { key: 'project_name', header: 'Công trình', sortable: true, render: (row) => <div><p className="font-semibold text-ink">{row.project_name}</p><p className="text-xs text-slate-500">{row.project_code}</p></div> },
            { key: 'author_name', header: 'Người lập', sortable: true, render: (row) => identityReferenceText(row.author_name, row.author_account_deleted) },
            { key: 'recipient_name', header: 'Gửi đến', sortable: true, render: (row) => row.recipient_name ? identityReferenceText(row.recipient_name, row.recipient_account_deleted) : 'Chưa gửi' },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <WorkReportStatusBadge status={row.status} /> },
            { key: 'current_revision_no', header: 'Lần gửi', align: 'right', sortable: true, render: (row) => Number(row.current_revision_no) > 0 ? `Lần ${row.current_revision_no}` : '—' },
          ]}
        />
      </section>

      <WorkReportDialog
        open={Boolean(selectedSourceID)}
        detail={detail.data ?? null}
        loading={detail.isLoading}
        error={detail.error}
        userID={user?.id ?? ''}
        canSubmit={hasPermission('work_report.submit')}
        canReview={canReview}
        canManage={canManage}
        canExport={canExport}
        onRefresh={async () => {
          await Promise.all([
            detail.refetch(),
            queryClient.invalidateQueries({ queryKey: ['work-reports'] }),
          ]);
        }}
        onClose={() => setSelectedSourceID('')}
      />
    </div>
  );
}

function WorkReportDialog({
  open,
  detail,
  loading,
  error,
  userID,
  canSubmit,
  canReview,
  canManage,
  canExport,
  onRefresh,
  onClose,
}: {
  open: boolean;
  detail: WorkReportDetail | null;
  loading: boolean;
  error: unknown;
  userID: string;
  canSubmit: boolean;
  canReview: boolean;
  canManage: boolean;
  canExport: boolean;
  onRefresh: () => Promise<void>;
  onClose: () => void;
}) {
  const navigate = useNavigate();
  const toast = useToast();
  const [recipientID, setRecipientID] = useState('');
  const [submitComment, setSubmitComment] = useState('');
  const [reviewComment, setReviewComment] = useState('');
  const [busyAction, setBusyAction] = useState('');
  const view = useMemo(() => constructionReportViewModel(detail ?? {}), [detail]);
  const maySubmit = Boolean(detail && canSubmit && detail.author_id === userID && detail.report_delivery_eligible && (detail.status === 'draft' || detail.status === 'needs_changes'));
  const mayReview = Boolean(detail && canReview && detail.status === 'submitted' && (canManage || detail.recipient_id === userID));
  const recipients = useQuery({
    queryKey: ['work-report-recipients', detail?.project_id],
    queryFn: () => get<Recipient[]>(`/work-reports/recipients?project_id=${detail?.project_id}`),
    enabled: open && maySubmit && Boolean(detail?.project_id),
    retry: false,
  });

  useEffect(() => {
    if (!open) return;
    setRecipientID(detail?.recipient_id ?? '');
    setSubmitComment('');
    setReviewComment('');
  }, [detail?.recipient_id, detail?.source_id, open]);

  async function submitReport() {
    if (!detail || !recipientID) {
      toast.warn({ title: 'Chưa chọn người nhận', description: 'Chọn người quản lý sẽ nhận và phản hồi báo cáo.' });
      return;
    }
    setBusyAction('submit');
    try {
      await post(`/work-reports/daily-reports/${detail.source_id}/submit`, { recipient_id: recipientID, comment: submitComment.trim() });
      await onRefresh();
      toast.success(detail.status === 'needs_changes' ? 'Đã gửi lại báo cáo' : 'Đã gửi báo cáo');
    } catch (requestError) {
      toast.error('Không gửi được báo cáo', requestError);
    } finally {
      setBusyAction('');
    }
  }

  async function review(action: 'request-changes' | 'acknowledge') {
    if (!detail?.work_report_id) return;
    if (action === 'request-changes' && !reviewComment.trim()) {
      toast.warn({ title: 'Chưa nhập nội dung cần bổ sung', description: 'Nêu rõ phần người lập cần chỉnh sửa trước khi gửi lại.' });
      return;
    }
    setBusyAction(action);
    try {
      await post(`/work-reports/${detail.work_report_id}/${action}`, { comment: reviewComment.trim() });
      await onRefresh();
      toast.success(action === 'acknowledge' ? 'Đã xác nhận xem báo cáo' : 'Đã gửi yêu cầu bổ sung');
    } catch (requestError) {
      toast.error(action === 'acknowledge' ? 'Không xác nhận được báo cáo' : 'Không gửi được yêu cầu bổ sung', requestError);
    } finally {
      setBusyAction('');
    }
  }

  async function exportRevision(reportID: string, revisionNo: number) {
    setBusyAction(`pdf-${revisionNo}`);
    try {
      const blob = await downloadBlob(`/work-reports/${reportID}/pdf?revision=${revisionNo}`);
      saveBlob(blob, `bao-cao-thi-cong-${detail?.report_date ?? 'khong-ro-ngay'}-lan-${revisionNo}.pdf`);
      toast.success(`Đã xuất PDF lần gửi ${revisionNo}`);
    } catch (requestError) {
      toast.error('Không xuất được PDF báo cáo', requestError);
    } finally {
      setBusyAction('');
    }
  }

  return (
    <Dialog
      open={open}
      title={detail ? `Báo cáo thi công ngày ${dateText(detail.report_date)}` : 'Chi tiết báo cáo'}
      description={detail ? `${detail.project_code} · ${detail.project_name}` : undefined}
      size="report"
      onClose={onClose}
      footer={detail ? (
        <div className="flex flex-wrap items-center justify-between gap-2">
          <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => navigate(`/projects/${detail.project_id}/operations`)}>Mở nguồn tại Thi công</button>
          <div className="flex flex-wrap justify-end gap-2">
            <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>
            {mayReview ? (
              <>
                <button type="button" className="inline-flex items-center gap-2 rounded-md border border-amber-200 bg-white px-3 py-2 text-sm font-semibold text-amber-700 hover:bg-amber-50 disabled:opacity-60" disabled={Boolean(busyAction)} onClick={() => void review('request-changes')}>
                  {busyAction === 'request-changes' ? <RefreshCw className="h-4 w-4 animate-spin" /> : <MessageSquareWarning className="h-4 w-4" />} Yêu cầu bổ sung
                </button>
                <button type="button" className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:opacity-60" disabled={Boolean(busyAction)} onClick={() => void review('acknowledge')}>
                  {busyAction === 'acknowledge' ? <RefreshCw className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />} Xác nhận đã xem
                </button>
              </>
            ) : null}
            {maySubmit ? (
              <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={Boolean(busyAction) || recipients.isLoading} onClick={() => void submitReport()}>
                {busyAction === 'submit' ? <RefreshCw className="h-4 w-4 animate-spin" /> : <ClipboardList className="h-4 w-4" />} {detail.status === 'needs_changes' ? 'Gửi lại báo cáo' : 'Gửi báo cáo'}
              </button>
            ) : null}
          </div>
        </div>
      ) : undefined}
    >
      {loading ? <div className="grid min-h-52 place-items-center"><LoadingSpinner /></div> : error ? (
        <div className="rounded-md border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">Không tải được chi tiết báo cáo. {error instanceof Error ? error.message : ''}</div>
      ) : detail ? (
        <div className="space-y-5">
          <div className="grid gap-3 rounded-lg border border-line bg-slate-50 p-4 sm:grid-cols-2 lg:grid-cols-5">
            <DetailMetric label="Trạng thái" value={<WorkReportStatusBadge status={detail.status} />} />
            <DetailMetric label="Người lập" value={identityReferenceText(detail.author_name, detail.author_account_deleted)} />
            <DetailMetric label="Gửi đến" value={detail.recipient_name ? identityReferenceText(detail.recipient_name, detail.recipient_account_deleted) : 'Chưa gửi'} />
            <DetailMetric label="Tiến độ ghi nhận" value={`${Number(detail.progress_percent ?? 0)}%`} />
            <DetailMetric label="Lần gửi hiện tại" value={Number(detail.current_revision_no) > 0 ? `Lần ${detail.current_revision_no}` : 'Chưa gửi'} />
          </div>

          {detail.status === 'recorded' ? (
            <div className="rounded-md border border-slate-200 bg-slate-50 p-3 text-sm text-slate-600">Đây là nhật ký được tạo trước khi có vòng đời gửi–phản hồi. Hệ thống giữ lại để tra cứu và không giả lập trạng thái đã gửi.</div>
          ) : null}
          {detail.status === 'needs_changes' && detail.review_comment ? (
            <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800"><strong>Nội dung cần bổ sung:</strong> {detail.review_comment}</div>
          ) : null}

          <div className="grid gap-4 lg:grid-cols-4">
            <ReportSection title="Đã làm" items={view.done} empty="Chưa ghi hạng mục hoàn tất" tone="emerald" />
            <ReportSection title="Đang làm" items={view.current} empty="Không có hạng mục đang thực hiện" tone="blue" />
            <ReportSection title="Sẽ làm" items={view.next} empty="Chưa ghi kế hoạch tiếp theo" tone="slate" />
            <ReportSection title="Vấn đề / trở ngại" items={view.timelineProblems} empty="Không có vấn đề" tone="rose" />
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <ReportSection title="Nhân lực" items={view.crew} empty={detail.manpower_summary || 'Chưa ghi nhân lực'} tone="slate" />
            <ReportSection title="Vật tư sử dụng" items={view.materials} empty={detail.material_usage_summary || 'Chưa ghi vật tư'} tone="slate" />
          </div>

          {maySubmit ? (
            <section className="rounded-lg border border-blue-200 bg-blue-50/50 p-4">
              <h3 className="font-semibold text-ink">{detail.status === 'needs_changes' ? 'Gửi lại cho người quản lý' : 'Gửi báo cáo cho người quản lý'}</h3>
              <div className="mt-3 grid gap-3 lg:grid-cols-2">
                <label className="block">
                  <span className="text-xs font-semibold text-slate-600">Người nhận <span className="text-rose-600">*</span></span>
                  <select className={clsx('mt-1 w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100', !recipientID ? 'border-amber-300' : 'border-line')} value={recipientID} onChange={(event) => setRecipientID(event.target.value)} disabled={recipients.isLoading}>
                    <option value="">Chọn người nhận báo cáo</option>
                    {(recipients.data ?? []).map((recipient) => <option key={recipient.id} value={recipient.id}>{recipient.full_name}{recipient.position ? ` · ${recipient.position}` : ''}</option>)}
                  </select>
                  {recipients.error ? <span className="mt-1 block text-xs text-rose-600">Không tải được danh sách người nhận.</span> : null}
                </label>
                <label className="block">
                  <span className="text-xs font-semibold text-slate-600">Ghi chú khi gửi</span>
                  <textarea className="mt-1 min-h-20 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100" maxLength={2000} value={submitComment} onChange={(event) => setSubmitComment(event.target.value)} placeholder="Nội dung cần người nhận lưu ý..." />
                </label>
              </div>
            </section>
          ) : null}

          {mayReview ? (
            <section className="rounded-lg border border-amber-200 bg-amber-50/50 p-4">
              <h3 className="font-semibold text-ink">Phản hồi cho người lập</h3>
              <p className="mt-1 text-sm text-slate-600">Nhập nội dung bắt buộc nếu yêu cầu bổ sung; có thể để trống khi xác nhận đã xem.</p>
              <textarea className="mt-3 min-h-24 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-amber-100" maxLength={2000} value={reviewComment} onChange={(event) => setReviewComment(event.target.value)} placeholder="Nội dung phản hồi..." />
            </section>
          ) : null}

          <section>
            <h3 className="font-semibold text-ink">Lịch sử gửi và phản hồi</h3>
            {!detail.events?.length ? <p className="mt-2 rounded-md border border-dashed border-line p-4 text-sm text-slate-500">Báo cáo chưa được gửi.</p> : (
              <div className="mt-3 space-y-2">
                {detail.events.map((event) => (
                  <article key={event.id} className="rounded-md border border-line p-3 text-sm">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <p className="font-semibold text-ink">{eventLabel(String(event.event_type))} · Lần {event.revision_no}</p>
                      <time className="text-xs text-slate-500">{dateTimeText(event.created_at)}</time>
                    </div>
                    <p className="mt-1 text-slate-600">{identityReferenceText(event.actor_name, event.actor_account_deleted, 'Không rõ người thực hiện')}{event.comment ? ` — ${event.comment}` : ''}</p>
                  </article>
                ))}
              </div>
            )}
          </section>

          {detail.revisions?.length ? (
            <section>
              <h3 className="font-semibold text-ink">Các bản đã gửi</h3>
              <div className="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                {detail.revisions.map((revision) => (
                  <article key={revision.id} className="rounded-md border border-line p-3">
                    <p className="font-semibold text-ink">Lần {revision.revision_no}</p>
                    <p className="mt-1 text-xs text-slate-500">{dateTimeText(revision.submitted_at)} · {identityReferenceText(revision.submitted_by_name || detail.author_name, revision.submitted_by_account_deleted ?? detail.author_account_deleted)}</p>
                    {revision.comment ? <p className="mt-2 text-sm text-slate-600">{revision.comment}</p> : null}
                    <p className="mt-2 truncate font-mono text-[10px] text-slate-400" title={revision.snapshot_sha256}>SHA-256 {revision.snapshot_sha256}</p>
                    {canExport && detail.work_report_id ? (
                      <button type="button" className="mt-3 inline-flex items-center gap-1 text-sm font-semibold text-blue-700 hover:text-blue-900 disabled:opacity-60" disabled={Boolean(busyAction)} onClick={() => void exportRevision(detail.work_report_id!, Number(revision.revision_no))}>
                        {busyAction === `pdf-${revision.revision_no}` ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />} PDF bản này
                      </button>
                    ) : null}
                  </article>
                ))}
              </div>
            </section>
          ) : null}
        </div>
      ) : null}
    </Dialog>
  );
}

function WorkReportCard({ row, onOpen }: { row: WorkReportRow; onOpen: () => void }) {
  return (
    <article className="rounded-lg border border-line bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div><p className="text-xs font-semibold text-slate-500">{dateText(row.report_date)}</p><h3 className="mt-1 font-semibold text-ink">{row.project_name}</h3><p className="text-xs text-slate-500">{row.project_code}</p></div>
        <WorkReportStatusBadge status={row.status} />
      </div>
      <dl className="mt-4 grid grid-cols-2 gap-3 text-sm"><div><dt className="text-xs text-slate-500">Người lập</dt><dd className="font-medium text-ink">{identityReferenceText(row.author_name, row.author_account_deleted)}</dd></div><div><dt className="text-xs text-slate-500">Gửi đến</dt><dd className="font-medium text-ink">{row.recipient_name ? identityReferenceText(row.recipient_name, row.recipient_account_deleted) : 'Chưa gửi'}</dd></div></dl>
      <button type="button" className="mt-4 inline-flex items-center gap-1 text-sm font-semibold text-blue-700" onClick={onOpen}><Eye className="h-4 w-4" /> Xem chi tiết</button>
    </article>
  );
}

function WorkReportStatusBadge({ status }: { status: WorkReportStatus }) {
  const style = {
    draft: 'bg-slate-100 text-slate-700', recorded: 'bg-slate-100 text-slate-500', submitted: 'bg-blue-50 text-blue-700', needs_changes: 'bg-amber-50 text-amber-700', acknowledged: 'bg-emerald-50 text-emerald-700',
  }[status];
  const label = {
    draft: 'Bản nháp', recorded: 'Nhật ký lịch sử', submitted: 'Chờ xác nhận', needs_changes: 'Yêu cầu bổ sung', acknowledged: 'Đã xác nhận',
  }[status];
  return <span className={clsx('inline-flex rounded-md px-2 py-1 text-xs font-semibold', style)}>{label}</span>;
}

function ReportSection({ title, items, empty, tone }: { title: string; items: string[]; empty: string; tone: 'emerald' | 'blue' | 'slate' | 'rose' }) {
  const border = { emerald: 'border-emerald-200', blue: 'border-blue-200', slate: 'border-slate-200', rose: 'border-rose-200' }[tone];
  return <section className={clsx('rounded-lg border bg-white p-4', border)}><h3 className="text-sm font-semibold text-ink">{title}</h3>{items.length ? <ul className="mt-2 space-y-1.5 text-sm text-slate-700">{items.map((item, index) => <li key={`${item}-${index}`} className="flex gap-2"><span aria-hidden="true">•</span><span>{item}</span></li>)}</ul> : <p className="mt-2 text-sm text-slate-500">{empty}</p>}</section>;
}

function DetailMetric({ label, value }: { label: string; value: React.ReactNode }) {
  return <div className="min-w-0"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p><div className="mt-1 text-sm font-semibold text-ink">{value}</div></div>;
}

function FilterInput({ label, value, onChange, type = 'text', placeholder, onEnter }: { label: string; value: string; onChange: (value: string) => void; type?: 'text' | 'date'; placeholder?: string; onEnter?: () => void }) {
  return <label className="block"><span className="text-xs font-semibold text-slate-600">{label}</span><input type={type} className="mt-1 h-10 w-full rounded-md border border-line bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-blue-100" value={value} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') onEnter?.(); }} /></label>;
}

function FilterSelect({ label, value, options, onChange }: { label: string; value: string; options: Array<[string, string]>; onChange: (value: string) => void }) {
  return <label className="block"><span className="text-xs font-semibold text-slate-600">{label}</span><select className="mt-1 h-10 w-full rounded-md border border-line bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-blue-100" value={value} onChange={(event) => onChange(event.target.value)}>{options.map(([optionValue, optionLabel]) => <option key={optionValue} value={optionValue}>{optionLabel}</option>)}</select></label>;
}

function ExportControl({ format, exporting, onFormatChange, onExport }: { format: ExportFormat; exporting: boolean; onFormatChange: (format: ExportFormat) => void; onExport: () => void }) {
  return <div className="inline-flex overflow-hidden rounded-md border border-blue-200 bg-white"><label className="sr-only" htmlFor="work-report-export-format">Định dạng xuất sổ báo cáo</label><select id="work-report-export-format" className="border-r border-blue-100 bg-white px-2 py-2 text-sm font-semibold text-blue-700 outline-none" value={format} onChange={(event) => onFormatChange(event.target.value as ExportFormat)} disabled={exporting}><option value="pdf">PDF</option><option value="xlsx">Excel</option><option value="csv">CSV</option></select><button type="button" className="inline-flex items-center gap-2 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60" disabled={exporting} onClick={onExport}>{exporting ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}{exporting ? 'Đang xuất…' : 'Xuất sổ'}</button></div>;
}

function saveBlob(blob: Blob, filename: string) {
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = objectURL;
  anchor.download = filename;
  anchor.click();
  window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000);
}

function exportLabel(format: ExportFormat) {
  return format === 'xlsx' ? 'Excel' : format.toUpperCase();
}

function eventLabel(eventType: string) {
  return { submitted: 'Đã gửi báo cáo', resubmitted: 'Đã gửi lại', needs_changes: 'Yêu cầu bổ sung', acknowledged: 'Đã xác nhận xem' }[eventType] ?? eventType;
}

function dateTimeText(value: unknown) {
  if (!value) return '—';
  const date = new Date(String(value));
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString('vi-VN');
}
