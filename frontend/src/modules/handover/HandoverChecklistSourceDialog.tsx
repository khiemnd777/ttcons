import { type ReactNode, useState } from 'react';
import { ExternalLink, Eye, FileText, Info, LockKeyhole } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { StatusBadge } from '../../components/StatusBadge';
import { useAuth } from '../../lib/auth/AuthContext';
import { currency, dateText } from '../../lib/utils/format';
import { EvidenceReferenceViewer } from '../_shared/EvidenceReferenceViewer';
import {
  automaticSourcePresentation,
  checklistItemCodeLabel,
  checklistItemCodeTooltip,
  checklistSourceMode,
  linkedSourcePresentations,
  type AutomaticSourcePresentation,
  type LinkedSourcePresentation,
  type SourceResult,
} from './handoverSourcePresentation';
import type { ChecklistItem, HandoverDetail } from './handoverTypes';

type HandoverChecklistSourceDialogProps = {
  open: boolean;
  item: ChecklistItem | null;
  detail: HandoverDetail | null;
  canEdit?: boolean;
  onEdit?: () => void;
  onClose: () => void;
};

export function HandoverChecklistSourceDialog({ open, item, detail, canEdit = false, onEdit, onClose }: HandoverChecklistSourceDialogProps) {
  const { hasAnyPermission } = useAuth();
  const [viewerSource, setViewerSource] = useState<LinkedSourcePresentation | null>(null);
  const mode = item ? checklistSourceMode(item) : null;
  const automaticSource = item && detail && mode === 'automatic' ? automaticSourcePresentation(item, detail) : null;
  const linkedSources = item && mode === 'linked' ? linkedSourcePresentations(item) : [];
  const canOpenAutomaticSource = Boolean(automaticSource && hasAnyPermission(automaticSource.permissions));

  const footer = <div className="flex flex-col-reverse gap-2 sm:flex-row sm:flex-wrap sm:justify-end">
    <button type="button" className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>
    {mode === 'linked' && canEdit && onEdit ? <button type="button" className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={onEdit}>Cập nhật liên kết</button> : null}
    {automaticSource && canOpenAutomaticSource ? <Link className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" to={automaticSource.targetPath} onClick={onClose}>{automaticSource.targetLabel}<ExternalLink className="h-4 w-4" /></Link> : null}
    {linkedSources.map((source) => source.openMode === 'external_document' && source.targetUrl ? <a key={source.id} className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" href={source.targetUrl} target="_blank" rel="noreferrer">Mở tài liệu<ExternalLink className="h-4 w-4" /></a> : source.openMode === 'protected_evidence' ? <button key={source.id} type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setViewerSource(source)}><Eye className="h-4 w-4" />Xem minh chứng</button> : null)}
  </div>;

  return <><Dialog
    open={open}
    title={mode === 'linked' ? 'Nguồn xác minh đã liên kết' : 'Nguồn xác minh tự động'}
    description={item ? item.title : undefined}
    onClose={onClose}
    size="lg"
    footer={footer}
  >
    {!item || !detail ? <SourceUnavailable message="Không xác định được mục checklist cần xem nguồn." /> : mode === 'automatic' && automaticSource ? <AutomaticSourceContent item={item} source={automaticSource} canOpenSource={canOpenAutomaticSource} /> : mode === 'linked' && linkedSources.length ? <LinkedSourceContent item={item} sources={linkedSources} /> : <SourceUnavailable message="Mục checklist này chưa có nguồn xác minh có thể hiển thị." />}
  </Dialog>
  <EvidenceReferenceViewer
    reference={viewerSource ? {
      id: viewerSource.id,
      name: viewerSource.name,
      mimeType: viewerSource.mimeType,
      metadata: [viewerSource.typeLabel],
      headerStatus: viewerSource.status ? <StatusBadge status={viewerSource.status} /> : undefined,
    } : null}
    onClose={() => setViewerSource(null)}
  />
  </>;
}

function AutomaticSourceContent({ item, source, canOpenSource }: { item: ChecklistItem; source: AutomaticSourcePresentation; canOpenSource: boolean }) {
  return <div className="space-y-5">
    <ChecklistHeader item={item} />

    <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_260px]">
      <section className="rounded-lg border border-line p-4">
        <h3 className="font-semibold text-ink">Nguồn và tiêu chí</h3>
        <dl className="mt-4 space-y-3 text-sm">
          <InfoRow label="Module nguồn" value={source.moduleLabel} />
          <InfoRow label="Nguồn xác minh" value={source.sourceLabel} />
          <div>
            <dt className="text-slate-500">Tiêu chí đạt</dt>
            <dd className="mt-1 rounded-md bg-slate-50 p-3 leading-6 text-ink">{source.criterion}</dd>
          </div>
        </dl>
        {source.linkedRecord ? <div className="mt-4 rounded-md border border-line bg-white p-3">
          <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{source.linkedRecord.label}</div>
          <div className="mt-1 font-semibold text-ink">{source.linkedRecord.title}</div>
          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-600">
            <span>Trạng thái: {acceptanceStatusLabel(source.linkedRecord.status)}</span>
            <span>Kết quả: {acceptanceResultLabel(source.linkedRecord.result)}</span>
          </div>
        </div> : null}
      </section>

      <section className="rounded-lg border border-line p-4">
        <h3 className="font-semibold text-ink">Lần kiểm tra gần nhất</h3>
        <div className="mt-4"><ResultBadge result={source.result} /></div>
        <dl className="mt-4 space-y-3 text-sm">
          {source.metrics.map((metric) => <InfoRow key={metric.label} label={metric.label} value={metric.value === undefined ? '-' : metric.unit === 'currency' ? currency(metric.value) : `${metric.value} ${metric.suffix ?? ''}`.trim()} />)}
          <InfoRow label="Thời điểm" value={dateTimeText(source.checkedAt)} />
        </dl>
      </section>
    </div>

    <Notice tone="blue">Đây là kết quả được lưu tại lần kiểm tra readiness gần nhất. Dữ liệu trong module nguồn có thể đã thay đổi; hãy mở module nguồn để xem trạng thái hiện tại.</Notice>
    {!canOpenSource ? <Notice tone="slate" icon={<LockKeyhole className="mt-0.5 h-4 w-4 shrink-0" />}>Bạn không có quyền mở module nguồn. Kết quả snapshot vẫn được hiển thị để phục vụ đối chiếu hồ sơ.</Notice> : null}
  </div>;
}

function LinkedSourceContent({ item, sources }: { item: ChecklistItem; sources: LinkedSourcePresentation[] }) {
  const hasPlaceholder = sources.some((source) => source.placeholderUrl);
  const hasOpenableFile = sources.some((source) => source.openMode !== 'none');
  return <div className="space-y-5">
    <ChecklistHeader item={item} />

    <div className="grid gap-4 sm:grid-cols-2">
      {sources.map((source) => <section key={`${source.kind}:${source.id}`} className="rounded-lg border border-line p-4">
        <div className="flex items-start gap-3">
          <span className="grid h-9 w-9 shrink-0 place-items-center rounded-md bg-blue-50 text-blue-700"><FileText className="h-4 w-4" /></span>
          <div className="min-w-0"><div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{source.kind === 'document' ? 'Tài liệu hồ sơ' : 'Minh chứng'}</div><h3 className="mt-1 break-words font-semibold text-ink">{source.name}</h3></div>
        </div>
        <dl className="mt-4 space-y-3 text-sm">
          <InfoRow label="Loại" value={source.typeLabel} />
          <InfoRow label="Trạng thái nguồn" value={sourceStatusLabel(source)} />
          {source.mimeType ? <InfoRow label="Định dạng" value={source.mimeType} /> : null}
          {source.fileSize ? <InfoRow label="Dung lượng" value={fileSizeText(source.fileSize)} /> : null}
        </dl>
      </section>)}
    </div>

    <section className="rounded-lg border border-line bg-slate-50 p-4">
      <h3 className="font-semibold text-ink">Thông tin xác minh</h3>
      <dl className="mt-4 grid gap-3 text-sm sm:grid-cols-2">
        <InfoRow label="Trạng thái checklist" value={item.status === 'verified' ? 'Đã xác thực' : 'Chờ xử lý'} />
        <InfoRow label="Thời điểm xác minh" value={dateTimeText(item.verified_at)} />
      </dl>
      <div className="mt-4 rounded-md border border-line bg-white p-3 text-sm leading-6 text-slate-600">{item.notes || 'Không có ghi chú xác minh.'}</div>
    </section>

    <Notice tone="blue">Nguồn này do người dùng liên kết vào checklist. Việc xem nguồn không thay đổi trạng thái checklist, readiness hoặc lịch sử bàn giao.</Notice>
    {hasPlaceholder ? <Notice tone="amber">URL của tài liệu demo đang dùng miền `example.com`; thông tin liên kết vẫn xem được nhưng URL mẫu không bảo đảm chứa tệp PDF thật.</Notice> : null}
    {!hasOpenableFile ? <Notice tone="amber">Bản ghi nguồn tồn tại nhưng chưa có tệp nội bộ hoặc URL có thể mở từ màn hình này.</Notice> : null}
  </div>;
}

function ChecklistHeader({ item }: { item: ChecklistItem }) {
  return <section className="flex flex-col gap-3 rounded-lg border border-line bg-slate-50 p-4 sm:flex-row sm:items-start sm:justify-between">
    <div>
      <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">Mục checklist</div>
      <div className="mt-1 font-semibold text-ink">{item.title}</div>
      <div className="mt-1 text-xs text-slate-500" title={checklistItemCodeTooltip(item.item_code)}>{checklistItemCodeLabel(item.item_code)}{item.required ? ' · Bắt buộc' : ''}</div>
    </div>
    <StatusBadge status={item.status} />
  </section>;
}

function SourceUnavailable({ message }: { message: string }) {
  return <div className="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800">{message}</div>;
}

function Notice({ children, tone, icon = <Info className="mt-0.5 h-4 w-4 shrink-0" /> }: { children: ReactNode; tone: 'blue' | 'slate' | 'amber'; icon?: ReactNode }) {
  const color = tone === 'blue' ? 'border-blue-200 bg-blue-50 text-blue-800' : tone === 'amber' ? 'border-amber-200 bg-amber-50 text-amber-800' : 'border-slate-200 bg-slate-50 text-slate-600';
  return <div className={`flex items-start gap-3 rounded-md border p-3 text-sm ${color}`}>{icon}<p>{children}</p></div>;
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return <div className="flex items-start justify-between gap-4"><dt className="text-slate-500">{label}</dt><dd className="max-w-[65%] text-right font-medium text-ink">{value}</dd></div>;
}

function ResultBadge({ result }: { result: SourceResult }) {
  const presentation = result === 'passed'
    ? { label: 'Đạt', className: 'border-emerald-200 bg-emerald-50 text-emerald-700' }
    : result === 'failed'
      ? { label: 'Chưa đạt', className: 'border-amber-200 bg-amber-50 text-amber-800' }
      : { label: 'Chưa kiểm tra', className: 'border-slate-200 bg-slate-50 text-slate-600' };
  return <span className={`inline-flex rounded-full border px-2.5 py-1 text-xs font-semibold ${presentation.className}`}>{presentation.label}</span>;
}

function sourceStatusLabel(source: LinkedSourcePresentation) {
  if (source.kind === 'document') return ({ active: 'Đang hoạt động', cancelled: 'Đã hủy', deleted: 'Đã xóa' } as Record<string, string>)[source.status ?? ''] ?? source.status ?? '-';
  return ({ accepted: 'Đã chấp nhận', submitted: 'Đang chờ', draft: 'Nháp', rejected: 'Từ chối' } as Record<string, string>)[source.status ?? ''] ?? source.status ?? '-';
}

function acceptanceStatusLabel(value?: string) {
  return ({ completed: 'Hoàn tất', submitted: 'Đã gửi', draft: 'Nháp', cancelled: 'Đã hủy' } as Record<string, string>)[value ?? ''] ?? value ?? '-';
}

function acceptanceResultLabel(value?: string) {
  return ({ pass: 'Đạt', conditional_pass: 'Đạt có điều kiện', fail: 'Không đạt' } as Record<string, string>)[value ?? ''] ?? value ?? '-';
}

function dateTimeText(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return dateText(value);
  return new Intl.DateTimeFormat('vi-VN', { dateStyle: 'short', timeStyle: 'short' }).format(date);
}

function fileSizeText(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
