import { useMemo } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  AlertTriangle,
  CalendarDays,
  CheckCircle2,
  ClipboardList,
  CloudSun,
  ExternalLink,
  FileText,
  Hammer,
  Package,
  Pencil,
  RefreshCw,
  Upload,
  Users,
} from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import type { ProjectEvidencePage, ProjectOperationsResponse, WorkItem } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { EvidenceGallery } from '../work-items/EvidenceGallery';
import { currentReturnTo, workItemDetailPath } from '../work-items/workItemNavigation';
import { constructionReportViewModel } from './constructionReportViewModel';

type ConstructionReportDetailDialogProps = {
  open: boolean;
  projectId: string;
  record: Record<string, any> | null;
  canManage?: boolean;
  onEdit?: () => void;
  onAddEvidence?: () => void;
  onClose: () => void;
};

export function ConstructionReportDetailDialog({ open, projectId, record, canManage = false, onEdit, onAddEvidence, onClose }: ConstructionReportDetailDialogProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const report = useMemo(() => constructionReportViewModel(record ?? {}), [record]);
  const evidenceKey = report.evidenceIds.join(',');
  const sourceExists = Boolean(report.workItemId || report.workflowFormInstanceId);
  const manualReport = !report.workflowFormInstanceId;

  const operations = useQuery({
    queryKey: ['project-operations', projectId, 'full'],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${projectId}/operations`),
    enabled: open && Boolean(projectId) && Boolean(report.workItemId),
    retry: false,
  });
  const workItem = useMemo(
    () => operations.data?.work_items?.find((item) => item.id === report.workItemId),
    [operations.data?.work_items, report.workItemId],
  );

  const evidence = useQuery({
    queryKey: ['construction-report-evidence', projectId, record?.id, evidenceKey],
    queryFn: async () => {
      const params = new URLSearchParams();
      if (report.workItemId) params.set('work_item_id', report.workItemId);
      params.set('limit', '50');
      const page = await get<ProjectEvidencePage>(`/projects/${projectId}/evidence?${params.toString()}`);
      const requestedIds = new Set(report.evidenceIds);
      return page.items.filter((item) => requestedIds.has(item.id));
    },
    enabled: open && Boolean(projectId) && report.evidenceIds.length > 0,
    retry: false,
  });

  function openSource() {
    if (!report.workItemId) return;
    onClose();
    navigate(workItemDetailPath(report.workItemId, currentReturnTo(location), projectId));
  }

  return (
    <Dialog
      open={open}
      title="Chi tiết báo cáo thi công"
      description="Tổng hợp tiến độ, công việc, nguồn lực và minh chứng của một ngày thi công."
      size="report"
      onClose={onClose}
      footer={(
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>
          {manualReport && canManage && onEdit ? <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-blue-200 bg-white px-4 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={onEdit}><Pencil className="h-4 w-4" /> Sửa báo cáo</button> : null}
          {report.workItemId ? <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={openSource}><ExternalLink className="h-4 w-4" /> Mở công việc liên quan</button> : null}
        </div>
      )}
    >
      <div className="space-y-5">
        <section className="overflow-hidden rounded-lg border border-blue-200 bg-gradient-to-r from-blue-50 to-white">
          <div className="grid gap-4 p-4 sm:grid-cols-[minmax(0,1fr)_240px] sm:items-center">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="inline-flex items-center gap-1.5 rounded-md bg-white px-2.5 py-1 text-sm font-semibold text-ink ring-1 ring-inset ring-blue-100"><CalendarDays className="h-4 w-4 text-blue-600" /> {dateText(report.reportDate)}</span>
                <span className="inline-flex items-center gap-1.5 rounded-md bg-white px-2.5 py-1 text-sm text-slate-700 ring-1 ring-inset ring-blue-100"><CloudSun className="h-4 w-4 text-amber-500" /> {report.weather || 'Chưa ghi thời tiết'}</span>
                <span className="rounded-md bg-blue-100 px-2.5 py-1 text-xs font-semibold uppercase tracking-wide text-blue-700">{report.sourceKind === 'workflow' ? 'Báo cáo từ quy trình' : report.sourceKind === 'structured' ? 'Báo cáo hiện trường' : 'Báo cáo tóm tắt'}</span>
              </div>
              <p className="mt-3 text-sm text-slate-600">Nhịp thi công trong ngày được tách rõ thành kết quả, việc đang tiếp diễn, kế hoạch tiếp theo và blocker cần xử lý.</p>
            </div>
            <div>
              <div className="mb-2 flex items-end justify-between gap-3"><span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tiến độ báo cáo</span><strong className="text-2xl text-blue-700">{report.progressPercent}%</strong></div>
              <div className="h-2.5 overflow-hidden rounded-full bg-blue-100" role="progressbar" aria-label="Tiến độ báo cáo" aria-valuemin={0} aria-valuemax={100} aria-valuenow={report.progressPercent}><div className="h-full rounded-full bg-blue-600 transition-[width]" style={{ width: `${report.progressPercent}%` }} /></div>
            </div>
          </div>
        </section>

        <section aria-labelledby="report-work-heading">
          <SectionHeading id="report-work-heading" icon={<Hammer className="h-4 w-4" />} title="Công việc trong ngày" description="Đọc từ trái sang phải để nắm kết quả và bước tiếp theo." />
          <div className="mt-3 grid gap-3 md:grid-cols-2">
            <WorkGroup title="Đã làm" items={report.done} tone="green" empty="Chưa ghi hạng mục hoàn tất" />
            <WorkGroup title="Đang làm" items={report.current} tone="blue" empty="Không có hạng mục dở dang" />
            <WorkGroup title="Sẽ làm" items={report.next} tone="slate" empty="Chưa ghi kế hoạch tiếp theo" />
            <WorkGroup title="Blocker cần xử lý" items={report.blockers} tone="red" empty="Không có blocker cần xử lý" />
          </div>
        </section>

        <section aria-labelledby="report-resource-heading">
          <SectionHeading id="report-resource-heading" icon={<Users className="h-4 w-4" />} title="Nguồn lực sử dụng" description="Nhân công và vật tư đã ghi nhận trong ca thi công." />
          <div className="mt-3 grid gap-3 md:grid-cols-2">
            <ResourceCard icon={<Users className="h-4 w-4" />} title="Nhân công / tổ đội" items={report.crew} empty="Chưa ghi nhân công hoặc tổ đội" />
            <ResourceCard icon={<Package className="h-4 w-4" />} title="Vật tư sử dụng" items={report.materials} empty="Chưa ghi vật tư sử dụng" />
          </div>
        </section>

        <section aria-labelledby="report-note-heading">
          <SectionHeading id="report-note-heading" icon={<FileText className="h-4 w-4" />} title="Ghi chú hiện trường" description="Thông tin bổ sung; không được tính là blocker nếu báo cáo không khai báo blocker có cấu trúc." />
          <div className="mt-3 rounded-lg border border-line bg-slate-50 p-4">
            {report.fieldNotes.length ? <ItemList items={report.fieldNotes} bulletClass="bg-slate-400" /> : <p className="text-sm italic text-slate-500">Không có ghi chú hiện trường.</p>}
          </div>
        </section>

        <section aria-labelledby="report-evidence-heading">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between"><SectionHeading id="report-evidence-heading" icon={<ClipboardList className="h-4 w-4" />} title={`Minh chứng · ${report.evidenceIds.length}`} description="Ảnh và tài liệu được liên kết trực tiếp với báo cáo." />{manualReport && canManage && onAddEvidence ? <button type="button" className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={onAddEvidence}><Upload className="h-4 w-4" /> Thêm minh chứng</button> : null}</div>
          <div className="mt-3">
            {!report.evidenceIds.length ? <div className="rounded-lg border border-dashed border-slate-300 bg-slate-50 p-5"><EmptyState title="Chưa có minh chứng liên kết với báo cáo này" /></div> : null}
            {report.evidenceIds.length && evidence.isLoading ? <LoadingSpinner label="Đang tải minh chứng báo cáo..." /> : null}
            {report.evidenceIds.length && evidence.isError ? <InlineError message="Không tải được minh chứng đã liên kết." onRetry={() => evidence.refetch()} /> : null}
            {report.evidenceIds.length && evidence.isSuccess ? <EvidenceGallery evidence={evidence.data} showControls={false} showWorkItem variant="cards" emptyTitle="Không tìm thấy minh chứng trong phạm vi truy cập" /> : null}
          </div>
        </section>

        {sourceExists ? (
          <section aria-labelledby="report-source-heading">
            <SectionHeading id="report-source-heading" icon={<ClipboardList className="h-4 w-4" />} title="Nguồn báo cáo" description="Truy vết báo cáo về đầu việc và biểu mẫu quy trình đã tạo ra dữ liệu." />
            <div className="mt-3 rounded-lg border border-line bg-white p-4">
              {report.workItemId && operations.isLoading ? <LoadingSpinner label="Đang tải công việc liên quan..." /> : null}
              {report.workItemId && operations.isError ? <InlineError message="Không tải được thông tin công việc; vẫn có thể mở công việc bằng liên kết đã lưu." onRetry={() => operations.refetch()} /> : null}
              {!operations.isLoading && !operations.isError ? <SourceSummary workItem={workItem} workItemId={report.workItemId} workflowFormInstanceId={report.workflowFormInstanceId} onOpen={report.workItemId ? openSource : undefined} /> : null}
            </div>
          </section>
        ) : null}
      </div>
    </Dialog>
  );
}

function SectionHeading({ id, icon, title, description }: { id: string; icon: React.ReactNode; title: string; description: string }) {
  return <div className="flex items-start gap-2"><span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-md bg-slate-100 text-slate-600">{icon}</span><div><h3 id={id} className="font-semibold text-ink">{title}</h3><p className="mt-0.5 text-xs leading-5 text-slate-500">{description}</p></div></div>;
}

function WorkGroup({ title, items, tone, empty }: { title: string; items: string[]; tone: 'green' | 'blue' | 'slate' | 'red'; empty: string }) {
  const tones = {
    green: { border: 'border-emerald-200', badge: 'bg-emerald-100 text-emerald-700', bullet: 'bg-emerald-500', empty: 'text-emerald-700' },
    blue: { border: 'border-blue-200', badge: 'bg-blue-100 text-blue-700', bullet: 'bg-blue-500', empty: 'text-slate-500' },
    slate: { border: 'border-line', badge: 'bg-slate-100 text-slate-700', bullet: 'bg-slate-400', empty: 'text-slate-500' },
    red: { border: items.length ? 'border-red-200' : 'border-emerald-200', badge: items.length ? 'bg-red-100 text-red-700' : 'bg-emerald-100 text-emerald-700', bullet: 'bg-red-500', empty: 'text-emerald-700' },
  }[tone];
  return <article className={`min-h-32 rounded-lg border bg-white p-4 ${tones.border}`}><div className="flex items-center justify-between gap-3"><h4 className="text-sm font-semibold text-ink">{title}</h4><span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${tones.badge}`}>{items.length}</span></div>{items.length ? <div className="mt-3"><ItemList items={items} bulletClass={tones.bullet} /></div> : <p className={`mt-4 flex items-center gap-2 text-sm italic ${tones.empty}`}>{tone === 'red' ? <CheckCircle2 className="h-4 w-4" /> : null}{empty}</p>}</article>;
}

function ResourceCard({ icon, title, items, empty }: { icon: React.ReactNode; title: string; items: string[]; empty: string }) {
  return <article className="rounded-lg border border-line bg-white p-4"><h4 className="flex items-center gap-2 text-sm font-semibold text-ink"><span className="text-blue-600">{icon}</span>{title}</h4>{items.length ? <div className="mt-3"><ItemList items={items} bulletClass="bg-blue-400" /></div> : <p className="mt-3 text-sm italic text-slate-500">{empty}</p>}</article>;
}

function ItemList({ items, bulletClass }: { items: string[]; bulletClass: string }) {
  return <ul className="space-y-2">{items.map((item, index) => <li key={`${item}-${index}`} className="flex gap-2 text-sm leading-5 text-slate-700"><span className={`mt-2 h-1.5 w-1.5 shrink-0 rounded-full ${bulletClass}`} /> <span>{item}</span></li>)}</ul>;
}

function InlineError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <div className="flex flex-col gap-3 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 sm:flex-row sm:items-center sm:justify-between"><span className="flex items-start gap-2"><AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />{message}</span><button type="button" className="inline-flex items-center gap-1.5 self-start font-semibold hover:underline" onClick={onRetry}><RefreshCw className="h-3.5 w-3.5" /> Thử lại</button></div>;
}

function SourceSummary({ workItem, workItemId, workflowFormInstanceId, onOpen }: { workItem?: WorkItem; workItemId: string; workflowFormInstanceId: string; onOpen?: () => void }) {
  if (!workItemId && !workflowFormInstanceId) return <EmptyState title="Báo cáo không có nguồn quy trình" />;
  return <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between"><div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><p className="font-semibold text-ink">{workItem?.title || 'Đầu việc đã liên kết'}</p>{workItem?.status ? <StatusBadge status={workItem.status} /> : null}</div><p className="mt-1 text-xs text-slate-500">{workItem?.run_step_name || workItem?.run_node_name || 'Nguồn quy trình thi công'}{workflowFormInstanceId ? ' · Có biểu mẫu báo cáo' : ''}</p></div>{onOpen ? <button type="button" className="inline-flex shrink-0 items-center gap-2 rounded-md border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={onOpen}><ExternalLink className="h-4 w-4" /> Mở đầu việc</button> : null}</div>;
}
