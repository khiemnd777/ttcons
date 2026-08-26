import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  AlertTriangle,
  CalendarDays,
  CheckCircle2,
  ClipboardCheck,
  CloudSun,
  Download,
  ExternalLink,
  FileClock,
  FileText,
  History,
  Images,
  MessageSquareWarning,
  Package,
  RefreshCw,
  ShieldCheck,
  Users,
} from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, post } from '../../lib/api/client';
import type { EvidenceRecord, WorkReportDetail, WorkReportEvent, WorkReportRevision, WorkReportStatus } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { constructionReportViewModel } from '../operations/constructionReportViewModel';
import { EvidenceGallery } from '../work-items/EvidenceGallery';

type ReviewTab = 'content' | 'history' | 'revisions';

type WorkReportReviewDialogProps = {
  open: boolean;
  detail: WorkReportDetail | null;
  loading: boolean;
  error: unknown;
  userID: string;
  canReview: boolean;
  canManage: boolean;
  canOpenSource: boolean;
  canExport: boolean;
  onRefresh: () => Promise<void>;
  onClose: () => void;
};

export function WorkReportReviewDialog({
  open,
  detail,
  loading,
  error,
  userID,
  canReview,
  canManage,
  canOpenSource,
  canExport,
  onRefresh,
  onClose,
}: WorkReportReviewDialogProps) {
  const navigate = useNavigate();
  const toast = useToast();
  const [activeTab, setActiveTab] = useState<ReviewTab>('content');
  const [selectedRevisionNo, setSelectedRevisionNo] = useState(0);
  const [reviewComment, setReviewComment] = useState('');
  const [busyAction, setBusyAction] = useState('');
  const [reviewCommentInvalid, setReviewCommentInvalid] = useState(false);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const navigateReviewValidation = useValidationNavigation(contentRef);
  const currentRevisionNo = Number(detail?.current_revision_no ?? 0);
  const revisions = useMemo(() => detail?.revisions ?? [], [detail?.revisions]);
  const selectedRevision = useMemo(
    () => revisions.find((revision) => Number(revision.revision_no) === selectedRevisionNo) ?? revisions.find((revision) => Number(revision.revision_no) === currentRevisionNo) ?? null,
    [currentRevisionNo, revisions, selectedRevisionNo],
  );
  const selectedSnapshot = selectedRevision?.snapshot_json ?? detail ?? {};
  const view = useMemo(() => constructionReportViewModel(selectedSnapshot), [selectedSnapshot]);
  const selectedEvidence = selectedRevision?.evidence_records ?? [];
  const isCurrentRevision = !selectedRevision || Number(selectedRevision.revision_no) === currentRevisionNo;
  const mayReview = Boolean(detail && isCurrentRevision && canReview && detail.status === 'submitted' && (canManage || detail.recipient_id === userID));

  const evidenceFilePath = useCallback((record: EvidenceRecord) => {
    if (!detail?.work_report_id || !selectedRevision) return `/evidence/${record.id}/file`;
    return `/work-reports/${detail.work_report_id}/revisions/${selectedRevision.revision_no}/evidence/${record.id}/file`;
  }, [detail?.work_report_id, selectedRevision]);

  useEffect(() => {
    if (!open) return;
    setActiveTab('content');
    setSelectedRevisionNo(Number(detail?.current_revision_no ?? 0));
    setReviewComment('');
    setReviewCommentInvalid(false);
  }, [detail?.source_id, open]);

  useEffect(() => {
    if (!open || !detail) return;
    setSelectedRevisionNo((current) => current || Number(detail.current_revision_no ?? 0));
  }, [detail, open]);

  async function review(action: 'request-changes' | 'acknowledge') {
    if (!detail?.work_report_id) return;
    if (action === 'request-changes' && !reviewComment.trim()) {
      setReviewCommentInvalid(true);
      toast.warn({ title: 'Chưa nhập nội dung cần bổ sung', description: 'Nêu rõ phần người lập cần chỉnh sửa trước khi gửi lại.' });
      navigateReviewValidation('review_comment');
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

  async function exportRevision(revisionNo: number) {
    if (!detail?.work_report_id) return;
    setBusyAction(`pdf-${revisionNo}`);
    try {
      const blob = await downloadBlob(`/work-reports/${detail.work_report_id}/pdf?revision=${revisionNo}`);
      saveBlob(blob, `bao-cao-thi-cong-${detail.report_date ?? 'khong-ro-ngay'}-lan-${revisionNo}.pdf`);
      toast.success(`Đã xuất PDF lần gửi ${revisionNo}`);
    } catch (requestError) {
      toast.error('Không xuất được PDF báo cáo', requestError);
    } finally {
      setBusyAction('');
    }
  }

  function selectRevision(revision: WorkReportRevision) {
    setSelectedRevisionNo(Number(revision.revision_no));
    setActiveTab('content');
  }

  function openSource() {
    if (!detail) return;
    navigate(`/projects/${detail.project_id}/operations?tab=timeline&report=${encodeURIComponent(detail.source_id)}`);
  }

  const footer = detail ? (
    <div className="flex flex-col-reverse gap-3 sm:flex-row sm:items-center sm:justify-between">
      {canOpenSource ? (
        <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700" onClick={openSource}>
          <ExternalLink className="h-4 w-4" /> Mở nguồn tại Thi công
        </button>
      ) : <span />}
      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        {!isCurrentRevision && currentRevisionNo > 0 ? (
          <button type="button" className="rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={() => setSelectedRevisionNo(currentRevisionNo)}>Về bản hiện tại</button>
        ) : null}
        {mayReview ? (
          <>
            <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-amber-300 bg-white px-3 py-2 text-sm font-semibold text-amber-800 hover:bg-amber-50 disabled:opacity-60" disabled={Boolean(busyAction)} onClick={() => void review('request-changes')}>
              {busyAction === 'request-changes' ? <RefreshCw className="h-4 w-4 animate-spin" /> : <MessageSquareWarning className="h-4 w-4" />} Yêu cầu bổ sung
            </button>
            <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-emerald-700 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-emerald-800 disabled:opacity-60" disabled={Boolean(busyAction)} onClick={() => void review('acknowledge')}>
              {busyAction === 'acknowledge' ? <RefreshCw className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />} Xác nhận đã xem
            </button>
          </>
        ) : null}
      </div>
    </div>
  ) : undefined;

  return (
    <Dialog
      open={open}
      title={detail ? `Duyệt báo cáo thi công · ${dateText(detail.report_date)}` : 'Duyệt báo cáo thi công'}
      description={detail ? `${detail.project_code} · ${detail.project_name}${detail.work_item_title ? ` · ${detail.work_item_title}` : ''}` : 'Đọc nội dung, kiểm tra minh chứng và phản hồi trong cùng một không gian.'}
      size="report"
      onClose={onClose}
      footer={footer}
    >
      {loading ? (
        <div className="grid min-h-80 place-items-center"><LoadingSpinner label="Đang tải báo cáo và minh chứng..." /></div>
      ) : error ? (
        <div className="flex min-h-64 flex-col items-center justify-center rounded-lg border border-rose-200 bg-rose-50 p-6 text-center">
          <AlertTriangle className="h-8 w-8 text-rose-500" />
          <h3 className="mt-3 font-semibold text-rose-900">Không tải được báo cáo</h3>
          <p className="mt-1 max-w-lg text-sm text-rose-700">{error instanceof Error ? error.message : 'Dữ liệu chi tiết hoặc minh chứng chưa sẵn sàng.'}</p>
          <button type="button" className="mt-4 inline-flex items-center gap-2 rounded-md border border-rose-300 bg-white px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-100" onClick={() => void onRefresh()}><RefreshCw className="h-4 w-4" /> Thử lại</button>
        </div>
      ) : detail ? (
        <div ref={contentRef} className="space-y-5">
          <ReportSummary detail={detail} revision={selectedRevision} view={view} />

          {!isCurrentRevision ? (
            <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
              <FileClock className="mt-0.5 h-4 w-4 shrink-0" />
              <p>Anh đang xem <strong>Lần {selectedRevision?.revision_no}</strong>. Nội dung và minh chứng bên dưới là snapshot bất biến của lần gửi này; thao tác duyệt chỉ mở ở bản hiện tại.</p>
            </div>
          ) : null}

          <div className="flex gap-1 overflow-x-auto border-b border-line" role="tablist" aria-label="Nội dung duyệt báo cáo">
            <ReviewTabButton active={activeTab === 'content'} icon={<ClipboardCheck className="h-4 w-4" />} onClick={() => setActiveTab('content')}>Nội dung &amp; minh chứng</ReviewTabButton>
            <ReviewTabButton active={activeTab === 'history'} icon={<History className="h-4 w-4" />} count={detail.events.length} onClick={() => setActiveTab('history')}>Lịch sử &amp; phản hồi</ReviewTabButton>
            <ReviewTabButton active={activeTab === 'revisions'} icon={<FileClock className="h-4 w-4" />} count={detail.revisions.length} onClick={() => setActiveTab('revisions')}>Các lần gửi</ReviewTabButton>
          </div>

          <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_22rem] xl:items-start">
            <main className="min-w-0">
              {activeTab === 'content' ? (
                <ReportContent view={view} evidence={selectedEvidence} filePath={evidenceFilePath} />
              ) : activeTab === 'history' ? (
                <ReportHistory events={detail.events} />
              ) : (
                <ReportRevisions
                  revisions={detail.revisions}
                  currentRevisionNo={currentRevisionNo}
                  selectedRevisionNo={Number(selectedRevision?.revision_no ?? 0)}
                  canExport={canExport}
                  busyAction={busyAction}
                  onSelect={selectRevision}
                  onExport={(revisionNo) => void exportRevision(revisionNo)}
                />
              )}
            </main>

            <ReviewRail
              detail={detail}
              revision={selectedRevision}
              evidenceCount={selectedEvidence.length}
              isCurrentRevision={isCurrentRevision}
              mayReview={mayReview}
              reviewComment={reviewComment}
              reviewCommentInvalid={reviewCommentInvalid}
              onReviewCommentChange={(value) => {
                setReviewComment(value);
                if (value.trim()) setReviewCommentInvalid(false);
              }}
            />
          </div>
        </div>
      ) : null}
    </Dialog>
  );
}

function ReportSummary({ detail, revision, view }: { detail: WorkReportDetail; revision: WorkReportRevision | null; view: ReturnType<typeof constructionReportViewModel> }) {
  return (
    <section className="overflow-hidden rounded-xl border border-blue-200 bg-gradient-to-r from-blue-50 via-white to-white">
      <div className="grid gap-5 p-4 sm:p-5 lg:grid-cols-[minmax(0,1fr)_17rem] lg:items-center">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-md bg-white px-2.5 py-1 text-sm font-semibold text-ink ring-1 ring-inset ring-blue-100"><CalendarDays className="h-4 w-4 text-blue-600" /> {dateText(view.reportDate || detail.report_date)}</span>
            <span className="inline-flex items-center gap-1.5 rounded-md bg-white px-2.5 py-1 text-sm text-slate-700 ring-1 ring-inset ring-blue-100"><CloudSun className="h-4 w-4 text-amber-500" /> {view.weather || 'Chưa ghi thời tiết'}</span>
            <span className="rounded-md bg-blue-100 px-2.5 py-1 text-xs font-semibold uppercase tracking-wide text-blue-700">{sourceKindLabel(view.sourceKind)}</span>
            <span className="rounded-md bg-white px-2.5 py-1 text-xs font-semibold text-slate-600 ring-1 ring-inset ring-blue-100">Lần {(revision?.revision_no ?? detail.current_revision_no) || '—'}</span>
          </div>
          <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-600">Ưu tiên đọc kết quả trong ngày, đối chiếu minh chứng hiện trường, sau đó xác nhận đã xem hoặc trả lại với nội dung cần bổ sung.</p>
        </div>
        <div>
          <div className="mb-2 flex items-end justify-between gap-3"><span className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tiến độ ghi nhận</span><strong className="text-2xl text-blue-700">{view.progressPercent}%</strong></div>
          <div className="h-2.5 overflow-hidden rounded-full bg-blue-100" role="progressbar" aria-label="Tiến độ ghi nhận" aria-valuemin={0} aria-valuemax={100} aria-valuenow={view.progressPercent}><div className="h-full rounded-full bg-blue-600" style={{ width: `${view.progressPercent}%` }} /></div>
        </div>
      </div>
    </section>
  );
}

function ReportContent({ view, evidence, filePath }: { view: ReturnType<typeof constructionReportViewModel>; evidence: EvidenceRecord[]; filePath: (record: EvidenceRecord) => string }) {
  return (
    <div className="space-y-6">
      <section aria-labelledby="review-work-heading">
        <SectionHeading id="review-work-heading" icon={<ClipboardCheck className="h-4 w-4" />} title="Công việc trong ngày" description="Kết quả, phần đang tiếp diễn, kế hoạch và trở ngại được đặt cạnh nhau để đối chiếu nhanh." />
        <div className="mt-3 grid gap-3 md:grid-cols-2">
          <WorkGroup title="Đã làm" items={view.done} tone="emerald" empty="Chưa ghi hạng mục hoàn tất" />
          <WorkGroup title="Đang làm" items={view.current} tone="blue" empty="Không có hạng mục đang thực hiện" />
          <WorkGroup title="Sẽ làm" items={view.next} tone="slate" empty="Chưa ghi kế hoạch tiếp theo" />
          <WorkGroup title="Vấn đề / trở ngại" items={view.timelineProblems} tone="rose" empty="Không có vấn đề được ghi nhận" />
        </div>
      </section>

      <section aria-labelledby="review-evidence-heading">
        <SectionHeading id="review-evidence-heading" icon={<Images className="h-4 w-4" />} title={`Minh chứng hiện trường · ${evidence.length}`} description="Danh sách chỉ đọc, được khóa theo đúng snapshot của lần gửi đang xem." />
        <div className="mt-3 rounded-xl border border-line bg-slate-50/60 p-3 sm:p-4">
          <EvidenceGallery
            evidence={evidence}
            emptyTitle="Lần gửi này chưa có hình ảnh hoặc tài liệu minh chứng"
            showControls={false}
            variant="review"
            initialVisibleCount={5}
            filePath={filePath}
            protectedAccessDescription="Tệp được mở qua API kiểm tra quyền báo cáo và ràng buộc đúng lần gửi."
          />
        </div>
      </section>

      <section aria-labelledby="review-resource-heading">
        <SectionHeading id="review-resource-heading" icon={<Users className="h-4 w-4" />} title="Nguồn lực sử dụng" description="Nhân lực và vật tư đã được người lập ghi nhận trong ngày." />
        <div className="mt-3 grid gap-3 md:grid-cols-2">
          <ResourceCard icon={<Users className="h-4 w-4" />} title="Nhân lực / tổ đội" items={view.crew} empty="Chưa ghi nhân lực" />
          <ResourceCard icon={<Package className="h-4 w-4" />} title="Vật tư sử dụng" items={view.materials} empty="Chưa ghi vật tư" />
        </div>
      </section>

      <section aria-labelledby="review-notes-heading">
        <SectionHeading id="review-notes-heading" icon={<FileText className="h-4 w-4" />} title="Ghi chú hiện trường" description="Thông tin bổ sung được giữ tách biệt với blocker có cấu trúc." />
        <div className="mt-3 rounded-lg border border-line bg-slate-50 p-4">{view.fieldNotes.length ? <ItemList items={view.fieldNotes} bulletClass="bg-slate-400" /> : <p className="text-sm italic text-slate-500">Không có ghi chú hiện trường.</p>}</div>
      </section>
    </div>
  );
}

function ReviewRail({ detail, revision, evidenceCount, isCurrentRevision, mayReview, reviewComment, reviewCommentInvalid, onReviewCommentChange }: {
  detail: WorkReportDetail;
  revision: WorkReportRevision | null;
  evidenceCount: number;
  isCurrentRevision: boolean;
  mayReview: boolean;
  reviewComment: string;
  reviewCommentInvalid: boolean;
  onReviewCommentChange: (value: string) => void;
}) {
  return (
    <aside className="space-y-3 xl:sticky xl:top-0">
      <section className="overflow-hidden rounded-xl border border-line bg-white shadow-sm" aria-labelledby="review-decision-heading">
        <div className="border-b border-line bg-slate-50 px-4 py-3">
          <h3 id="review-decision-heading" className="flex items-center gap-2 font-semibold text-ink"><ShieldCheck className="h-4 w-4 text-blue-600" /> Kiểm tra trước khi phản hồi</h3>
          <p className="mt-1 text-xs leading-5 text-slate-500">Thông tin nhận diện bản gửi và phạm vi xác nhận.</p>
        </div>
        <dl className="divide-y divide-line px-4">
          <RailMetric label="Trạng thái" value={<WorkReportStatusBadge status={detail.status} />} />
          <RailMetric label="Bản đang xem" value={revision ? `Lần ${revision.revision_no}${isCurrentRevision ? ' · Hiện tại' : ''}` : 'Chưa gửi'} />
          <RailMetric label="Minh chứng" value={`${evidenceCount} tệp`} />
          <RailMetric label="Người lập" value={identityReferenceText(detail.author_name, detail.author_account_deleted)} />
          <RailMetric label="Gửi đến" value={detail.recipient_name ? identityReferenceText(detail.recipient_name, detail.recipient_account_deleted) : 'Chưa gửi'} />
          <RailMetric label="Thời điểm gửi" value={dateTimeText(revision?.submitted_at ?? detail.submitted_at)} />
        </dl>
      </section>

      {detail.status === 'recorded' ? <Notice tone="slate">Đây là nhật ký cũ chưa có vòng đời gửi–phản hồi; hệ thống giữ lại để tra cứu.</Notice> : null}
      {detail.status === 'needs_changes' && detail.review_comment ? <Notice tone="amber"><strong>Nội dung cần bổ sung:</strong> {detail.review_comment}</Notice> : null}

      <div className="flex items-start gap-2 rounded-lg border border-blue-200 bg-blue-50 p-3 text-xs leading-5 text-blue-900">
        <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0" />
        <p><strong>“Xác nhận đã xem”</strong> chỉ xác nhận đã tiếp nhận báo cáo, không phải nghiệm thu chất lượng, xác nhận khối lượng hay hoàn thành đầu việc.</p>
      </div>

      {mayReview ? (
        <label data-validation-key="review_comment" data-validation-error={reviewCommentInvalid ? 'true' : undefined} className={clsx('block scroll-mt-24 rounded-lg border p-3', reviewCommentInvalid ? 'border-rose-300 bg-rose-50' : 'border-amber-200 bg-amber-50/70')}>
          <span className="block text-sm font-semibold text-ink">Phản hồi cho người lập</span>
          <span className="mt-1 block text-xs leading-5 text-slate-600">Bắt buộc khi yêu cầu bổ sung; có thể để trống khi xác nhận đã xem.</span>
          <textarea name="review_comment" aria-invalid={reviewCommentInvalid} className="mt-3 min-h-28 w-full resize-y rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-amber-200" maxLength={2000} value={reviewComment} onChange={(event) => onReviewCommentChange(event.target.value)} placeholder="Nêu nội dung cần bổ sung..." />
          <span className="mt-1 block text-right text-[11px] tabular-nums text-slate-400">{reviewComment.length}/2000</span>
        </label>
      ) : null}
    </aside>
  );
}

function ReportHistory({ events }: { events: WorkReportEvent[] }) {
  if (!events.length) return <EmptyPanel icon={<History className="h-6 w-6" />} title="Báo cáo chưa có lịch sử gửi và phản hồi" />;
  return (
    <section aria-labelledby="report-history-heading">
      <SectionHeading id="report-history-heading" icon={<History className="h-4 w-4" />} title="Lịch sử gửi và phản hồi" description="Dòng thời gian bất biến của từng lần gửi, yêu cầu bổ sung và xác nhận đã xem." />
      <div className="relative mt-4 space-y-3 before:absolute before:bottom-6 before:left-[17px] before:top-6 before:w-px before:bg-blue-200">
        {events.map((event) => (
          <article key={event.id} className="relative pl-11">
            <span className={clsx('absolute left-2 top-4 z-10 h-5 w-5 rounded-full border-4 border-white', eventTone(event.event_type))} />
            <div className="rounded-lg border border-line bg-white p-4">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div><h4 className="font-semibold text-ink">{eventLabel(event.event_type)}</h4><p className="mt-1 text-xs text-slate-500">Lần {event.revision_no} · {identityReferenceText(event.actor_name, event.actor_account_deleted, 'Không rõ người thực hiện')}</p></div>
                <time className="text-xs text-slate-500">{dateTimeText(event.created_at)}</time>
              </div>
              {event.comment ? <p className="mt-3 rounded-md bg-slate-50 px-3 py-2 text-sm leading-6 text-slate-700">{event.comment}</p> : null}
            </div>
          </article>
        ))}
      </div>
    </section>
  );
}

function ReportRevisions({ revisions, currentRevisionNo, selectedRevisionNo, canExport, busyAction, onSelect, onExport }: {
  revisions: WorkReportRevision[];
  currentRevisionNo: number;
  selectedRevisionNo: number;
  canExport: boolean;
  busyAction: string;
  onSelect: (revision: WorkReportRevision) => void;
  onExport: (revisionNo: number) => void;
}) {
  if (!revisions.length) return <EmptyPanel icon={<FileClock className="h-6 w-6" />} title="Báo cáo chưa có bản đã gửi" />;
  return (
    <section aria-labelledby="report-revisions-heading">
      <SectionHeading id="report-revisions-heading" icon={<FileClock className="h-4 w-4" />} title="Các lần gửi" description="Mỗi lần gửi giữ nguyên nội dung, minh chứng và mã SHA-256 tại thời điểm gửi." />
      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        {revisions.map((revision) => {
          const selected = Number(revision.revision_no) === selectedRevisionNo;
          const current = Number(revision.revision_no) === currentRevisionNo;
          return (
            <article key={revision.id} className={clsx('rounded-lg border p-4', selected ? 'border-blue-300 bg-blue-50/50 ring-1 ring-blue-100' : 'border-line bg-white')}>
              <div className="flex items-start justify-between gap-3"><div><h4 className="font-semibold text-ink">Lần {revision.revision_no}</h4><p className="mt-1 text-xs text-slate-500">{dateTimeText(revision.submitted_at)}</p></div>{current ? <span className="rounded-md bg-blue-100 px-2 py-1 text-xs font-semibold text-blue-700">Hiện tại</span> : null}</div>
              <p className="mt-3 text-sm text-slate-600">{identityReferenceText(revision.submitted_by_name, revision.submitted_by_account_deleted, 'Không rõ người gửi')} · {revision.evidence_records.length} minh chứng</p>
              {revision.comment ? <p className="mt-2 line-clamp-2 text-sm text-slate-600">{revision.comment}</p> : null}
              <p className="mt-3 truncate font-mono text-[10px] text-slate-400" title={revision.snapshot_sha256}>SHA-256 {revision.snapshot_sha256}</p>
              <div className="mt-4 flex flex-wrap gap-2">
                <button type="button" className="rounded-md border border-blue-200 bg-white px-2.5 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-50" onClick={() => onSelect(revision)}>Xem nội dung</button>
                {canExport ? <button type="button" className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2.5 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-60" disabled={Boolean(busyAction)} onClick={() => onExport(Number(revision.revision_no))}>{busyAction === `pdf-${revision.revision_no}` ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />} PDF bản này</button> : null}
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}

function ReviewTabButton({ active, icon, count, children, onClick }: { active: boolean; icon: ReactNode; count?: number; children: ReactNode; onClick: () => void }) {
  return <button type="button" role="tab" aria-selected={active} className={clsx('inline-flex min-h-11 shrink-0 items-center gap-2 border-b-2 px-3 text-sm font-semibold', active ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink')} onClick={onClick}>{icon}<span>{children}</span>{typeof count === 'number' ? <span className={clsx('rounded-full px-1.5 py-0.5 text-[11px]', active ? 'bg-blue-100 text-blue-700' : 'bg-slate-100 text-slate-500')}>{count}</span> : null}</button>;
}

function SectionHeading({ id, icon, title, description }: { id: string; icon: ReactNode; title: string; description: string }) {
  return <div className="flex items-start gap-2"><span className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-md bg-slate-100 text-slate-600">{icon}</span><div><h3 id={id} className="font-semibold text-ink">{title}</h3><p className="mt-0.5 text-xs leading-5 text-slate-500">{description}</p></div></div>;
}

function WorkGroup({ title, items, tone, empty }: { title: string; items: string[]; tone: 'emerald' | 'blue' | 'slate' | 'rose'; empty: string }) {
  const styles = {
    emerald: { border: 'border-emerald-200', badge: 'bg-emerald-100 text-emerald-700', bullet: 'bg-emerald-500', empty: 'text-emerald-700' },
    blue: { border: 'border-blue-200', badge: 'bg-blue-100 text-blue-700', bullet: 'bg-blue-500', empty: 'text-slate-500' },
    slate: { border: 'border-line', badge: 'bg-slate-100 text-slate-700', bullet: 'bg-slate-400', empty: 'text-slate-500' },
    rose: { border: items.length ? 'border-rose-200' : 'border-emerald-200', badge: items.length ? 'bg-rose-100 text-rose-700' : 'bg-emerald-100 text-emerald-700', bullet: 'bg-rose-500', empty: 'text-emerald-700' },
  }[tone];
  return <article className={clsx('min-h-32 rounded-lg border bg-white p-4', styles.border)}><div className="flex items-center justify-between gap-3"><h4 className="text-sm font-semibold text-ink">{title}</h4><span className={clsx('rounded-full px-2 py-0.5 text-xs font-semibold', styles.badge)}>{items.length}</span></div>{items.length ? <div className="mt-3"><ItemList items={items} bulletClass={styles.bullet} /></div> : <p className={clsx('mt-4 flex items-center gap-2 text-sm italic', styles.empty)}>{tone === 'rose' ? <CheckCircle2 className="h-4 w-4" /> : null}{empty}</p>}</article>;
}

function ResourceCard({ icon, title, items, empty }: { icon: ReactNode; title: string; items: string[]; empty: string }) {
  return <article className="rounded-lg border border-line bg-white p-4"><h4 className="flex items-center gap-2 text-sm font-semibold text-ink"><span className="text-blue-600">{icon}</span>{title}</h4>{items.length ? <div className="mt-3"><ItemList items={items} bulletClass="bg-blue-400" /></div> : <p className="mt-3 text-sm italic text-slate-500">{empty}</p>}</article>;
}

function ItemList({ items, bulletClass }: { items: string[]; bulletClass: string }) {
  return <ul className="space-y-2">{items.map((item, index) => <li key={`${item}-${index}`} className="flex gap-2 text-sm leading-5 text-slate-700"><span className={clsx('mt-2 h-1.5 w-1.5 shrink-0 rounded-full', bulletClass)} /><span>{item}</span></li>)}</ul>;
}

function RailMetric({ label, value }: { label: string; value: ReactNode }) {
  return <div className="grid grid-cols-[7rem_minmax(0,1fr)] gap-3 py-3 text-sm"><dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</dt><dd className="min-w-0 break-words font-medium text-ink">{value}</dd></div>;
}

function Notice({ tone, children }: { tone: 'slate' | 'amber'; children: ReactNode }) {
  return <div className={clsx('rounded-lg border p-3 text-sm leading-6', tone === 'amber' ? 'border-amber-200 bg-amber-50 text-amber-900' : 'border-slate-200 bg-slate-50 text-slate-700')}>{children}</div>;
}

function EmptyPanel({ icon, title }: { icon: ReactNode; title: string }) {
  return <div className="grid min-h-52 place-items-center rounded-xl border border-dashed border-slate-300 bg-slate-50 p-6 text-center"><div><span className="mx-auto grid h-12 w-12 place-items-center rounded-full bg-white text-slate-400 shadow-sm">{icon}</span><p className="mt-3 text-sm font-medium text-slate-600">{title}</p></div></div>;
}

export function WorkReportStatusBadge({ status }: { status: WorkReportStatus }) {
  const style = {
    draft: 'bg-slate-100 text-slate-700',
    recorded: 'bg-slate-100 text-slate-500',
    submitted: 'bg-blue-50 text-blue-700',
    needs_changes: 'bg-amber-50 text-amber-700',
    acknowledged: 'bg-emerald-50 text-emerald-700',
  }[status];
  const label = {
    draft: 'Nháp tại nguồn',
    recorded: 'Nhật ký lịch sử',
    submitted: 'Chờ xác nhận',
    needs_changes: 'Yêu cầu bổ sung',
    acknowledged: 'Đã xác nhận',
  }[status];
  return <span className={clsx('inline-flex rounded-md px-2 py-1 text-xs font-semibold', style)}>{label}</span>;
}

function eventLabel(eventType: string) {
  return { submitted: 'Đã gửi báo cáo', resubmitted: 'Đã gửi lại', needs_changes: 'Yêu cầu bổ sung', acknowledged: 'Đã xác nhận xem' }[eventType] ?? eventType;
}

function eventTone(eventType: string) {
  if (eventType === 'acknowledged') return 'bg-emerald-500';
  if (eventType === 'needs_changes') return 'bg-amber-500';
  return 'bg-blue-500';
}

function sourceKindLabel(sourceKind: ReturnType<typeof constructionReportViewModel>['sourceKind']) {
  return sourceKind === 'workflow' ? 'Từ quy trình' : sourceKind === 'structured' ? 'Hiện trường' : 'Tóm tắt';
}

function dateTimeText(value: unknown) {
  if (!value) return '—';
  const date = new Date(String(value));
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString('vi-VN');
}

function saveBlob(blob: Blob, filename: string) {
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = objectURL;
  anchor.download = filename;
  anchor.click();
  window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000);
}
