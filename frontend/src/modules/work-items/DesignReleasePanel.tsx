import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, ChevronDown, FilePlus2, FileText, History, Send, Upload, X } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { dateTimeText } from '../../lib/utils/format';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get, post, postForm, put } from '../../lib/api/client';
import type { DesignRelease, DesignReleaseFile, DesignReleaseWorkspace, WorkItem } from '../../lib/types/domain';
import { openProtectedBlob } from '../../lib/utils/openProtectedBlob';

type PendingDesignFile = {
  key: string;
  file: File;
  documentType: 'layout' | 'render_3d' | 'construction_drawing' | 'other';
};

const documentTypeOptions: Array<{ value: PendingDesignFile['documentType']; label: string }> = [
  { value: 'layout', label: 'Mặt bằng / Layout' },
  { value: 'render_3d', label: 'Phối cảnh 3D' },
  { value: 'construction_drawing', label: 'Bản vẽ thi công' },
  { value: 'other', label: 'Tài liệu khác' },
];

export function DesignReleasePanel({ projectId, workItems = [] }: { projectId: string; workItems?: WorkItem[] }) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [editorOpen, setEditorOpen] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const workspace = useQuery({
    queryKey: ['design-releases', projectId],
    queryFn: () => get<DesignReleaseWorkspace>(`/projects/${projectId}/design-releases`),
    enabled: Boolean(projectId),
    retry: false,
  });
  const designWorkItem = workItems.find((item) => String(item.run_node_business_code ?? '').toLowerCase() === 'design'
    || String(item.run_step_business_code ?? '').toLowerCase().startsWith('design.'));
  const handoffWorkItem = workItems.find((item) => item.domain_action_key === 'source.design_documents_ready');
  const current = workspace.data?.current_release ?? null;
  const draft = workspace.data?.draft_release ?? null;
  const latestHandoff = workspace.data?.latest_handoff ?? null;

  const handoff = useMutation({
    mutationFn: () => post(`/design-releases/${current?.id}/handoffs`, {
      work_item_id: handoffWorkItem?.id,
      comment: `Bàn giao ${current?.revision_label ?? 'revision thiết kế'} cho QS`,
    }),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['design-releases', projectId] }),
        queryClient.invalidateQueries({ queryKey: ['work-item'] }),
        queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] }),
      ]);
      toast.success(`Đã bàn giao ${current?.revision_label ?? 'revision thiết kế'} cho QS`);
    },
    onError: (error) => toast.error('Không bàn giao được revision thiết kế', error),
  });

  if (workspace.isLoading) {
    return <section className="rounded-lg border border-line bg-white p-4 shadow-panel" aria-label="Đang tải revision hồ sơ thiết kế"><LoadingSpinner /></section>;
  }
  if (workspace.error) {
    return (
      <section className="rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
        <p className="font-semibold">Không tải được revision hồ sơ thiết kế.</p>
        <button type="button" className="mt-2 rounded-md border border-rose-200 bg-white px-3 py-2 font-semibold" onClick={() => workspace.refetch()}>Thử lại</button>
      </section>
    );
  }
  if (!workspace.data) return null;

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel" aria-label="Revision hồ sơ thiết kế">
      <div className="flex flex-col gap-3 border-b border-line px-4 py-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">Thiết kế</p>
          <h2 className="mt-1 text-base font-semibold text-ink">Revision hồ sơ thiết kế</h2>
          <p className="mt-1 text-sm text-slate-600">Mỗi lần phát hành là một gói hồ sơ độc lập. Revision mới không tự thay revision PM đã bàn giao cho QS.</p>
        </div>
        {workspace.data.capabilities.can_manage ? (
          <button type="button" className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setEditorOpen(true)}>
            <FilePlus2 className="h-4 w-4" />
            {draft ? `Tiếp tục ${draft.revision_label}` : 'Tạo revision mới'}
          </button>
        ) : null}
      </div>

      {workspace.data.requires_rehandoff && current ? (
        <div className="border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex gap-2">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
              <div>
                <p className="font-semibold">{latestHandoff ? `Có revision mới ${current.revision_label} chưa được bàn giao lại` : `${current.revision_label} chưa được PM bàn giao cho QS`}</p>
                <p className="mt-1">BOQ và mua hàng giữ nguyên nguồn cũ cho đến khi PM xác nhận bàn giao revision hiện hành.</p>
              </div>
            </div>
            {workspace.data.capabilities.can_handoff ? (
              <button type="button" disabled={handoff.isPending} className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-amber-700 px-3 py-2 font-semibold text-white hover:bg-amber-800 disabled:opacity-60" onClick={() => handoff.mutate()}>
                <Send className="h-4 w-4" />
                Bàn giao {current.revision_label} cho QS
              </button>
            ) : null}
          </div>
        </div>
      ) : current && latestHandoff ? (
        <div className="border-b border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900">
          <p className="font-semibold">PM đã bàn giao {latestHandoff.revision_label} cho QS</p>
          <p className="mt-1">{latestHandoff.handed_off_by_name || 'Không rõ người bàn giao'} · {dateTimeText(latestHandoff.handed_off_at)}</p>
        </div>
      ) : null}

      <div className="grid gap-4 p-4 lg:grid-cols-[minmax(0,1.35fr)_minmax(16rem,0.65fr)]">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Revision hiện hành</p>
          {current ? (
            <div className="mt-2 rounded-lg border border-blue-200 bg-blue-50/40 p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <p className="text-lg font-semibold text-ink">{current.revision_label}</p>
                  <p className="mt-1 text-sm text-slate-700">{current.change_reason}</p>
                  {current.notes ? <p className="mt-1 text-sm text-slate-500">{current.notes}</p> : null}
                </div>
                <span className="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-semibold text-emerald-800">Đã phát hành</span>
              </div>
              <p className="mt-3 text-xs text-slate-500">{current.released_by_name || 'Không rõ người phát hành'} · {dateTimeText(current.released_at)}</p>
              <DesignReleaseFiles files={current.files ?? []} />
            </div>
          ) : (
            <div className="mt-2"><EmptyState title="Chưa có revision thiết kế được phát hành" /></div>
          )}
        </div>

        <div className="space-y-3">
          {draft ? (
            <div className="rounded-lg border border-amber-200 bg-amber-50 p-4">
              <p className="text-xs font-semibold uppercase tracking-wide text-amber-700">Bản nháp đang làm</p>
              <p className="mt-1 font-semibold text-ink">{draft.revision_label}</p>
              <p className="mt-1 text-sm text-slate-700">{draft.change_reason}</p>
              <p className="mt-2 text-xs text-slate-500">{draft.files?.length ?? 0} tệp · cập nhật {dateTimeText(draft.updated_at)}</p>
            </div>
          ) : null}
          <button type="button" className="flex w-full items-center justify-between gap-3 rounded-lg border border-line px-4 py-3 text-left hover:bg-slate-50" aria-expanded={historyOpen} onClick={() => setHistoryOpen((value) => !value)}>
            <span className="flex items-center gap-2 text-sm font-semibold text-ink"><History className="h-4 w-4 text-blue-700" />Lịch sử revision</span>
            <span className="flex items-center gap-2 text-xs text-slate-500">{workspace.data.releases.length} bản <ChevronDown className={`h-4 w-4 transition-transform ${historyOpen ? 'rotate-180' : ''}`} /></span>
          </button>
          {historyOpen ? <DesignReleaseHistory releases={workspace.data.releases} /> : null}
        </div>
      </div>

      <DesignReleaseEditorDialog
        open={editorOpen}
        projectId={projectId}
        workItemId={designWorkItem?.id}
        draft={draft}
        onClose={() => setEditorOpen(false)}
        onChanged={async () => {
          await queryClient.invalidateQueries({ queryKey: ['design-releases', projectId] });
          await workspace.refetch();
        }}
      />
    </section>
  );
}

export function isDesignReleaseWorkItem(item?: WorkItem | null) {
  if (!item) return false;
  return String(item.run_node_business_code ?? '').toLowerCase() === 'design'
    || String(item.run_step_business_code ?? '').toLowerCase().startsWith('design.')
    || item.domain_action_key === 'source.design_documents_ready';
}

function DesignReleaseFiles({ files }: { files: DesignReleaseFile[] }) {
  const toast = useToast();
  if (!files.length) return <p className="mt-3 rounded-md border border-dashed border-slate-300 p-3 text-sm text-slate-500">Revision chưa có tệp.</p>;
  return (
    <div className="mt-3 divide-y divide-line overflow-hidden rounded-md border border-line bg-white">
      {files.map((file) => (
        <div key={file.id} className="flex items-center justify-between gap-3 px-3 py-2.5">
          <div className="flex min-w-0 items-center gap-2">
            <FileText className="h-4 w-4 shrink-0 text-blue-700" />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-ink">{file.name}</p>
              <p className="text-xs text-slate-500">{documentTypeLabel(file.document_type)} · {fileSizeText(file.file_size)}</p>
            </div>
          </div>
          <button type="button" className="shrink-0 rounded-md border border-line px-2.5 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-50" onClick={() => {
            void openProtectedBlob(() => downloadBlob(`/documents/${file.id}/file`)).catch((error) => toast.error('Không mở được tệp thiết kế', error));
          }}>Xem</button>
        </div>
      ))}
    </div>
  );
}

function DesignReleaseHistory({ releases }: { releases: DesignRelease[] }) {
  if (!releases.length) return <p className="rounded-md border border-dashed border-slate-300 p-3 text-sm text-slate-500">Chưa có lịch sử revision.</p>;
  return (
    <div className="space-y-2">
      {releases.map((release) => (
        <div key={release.id} className="rounded-md border border-line p-3">
          <div className="flex items-center justify-between gap-2">
            <p className="font-semibold text-ink">{release.revision_label}</p>
            <span className={`rounded-full px-2 py-0.5 text-xs font-semibold ${release.status === 'released' ? 'bg-emerald-100 text-emerald-800' : release.status === 'draft' ? 'bg-amber-100 text-amber-800' : 'bg-slate-100 text-slate-600'}`}>{designReleaseStatusLabel(release.status)}</span>
          </div>
          <p className="mt-1 text-sm text-slate-600">{release.change_reason}</p>
          <p className="mt-1 text-xs text-slate-500">{release.files?.length ?? 0} tệp · {dateTimeText(release.released_at || release.created_at)}</p>
        </div>
      ))}
    </div>
  );
}

function DesignReleaseEditorDialog({ open, projectId, workItemId, draft, onClose, onChanged }: {
  open: boolean;
  projectId: string;
  workItemId?: string;
  draft: DesignRelease | null;
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const toast = useToast();
  const [revisionLabel, setRevisionLabel] = useState('');
  const [changeReason, setChangeReason] = useState('');
  const [notes, setNotes] = useState('');
  const [pendingFiles, setPendingFiles] = useState<PendingDesignFile[]>([]);
  const [attempted, setAttempted] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    if (!open) return;
    setRevisionLabel(draft?.revision_label ?? '');
    setChangeReason(draft?.change_reason ?? '');
    setNotes(draft?.notes ?? '');
    setPendingFiles([]);
    setAttempted(false);
  }, [draft, open]);

  async function save(releaseNow: boolean) {
    setAttempted(true);
    if (!changeReason.trim()) {
      toast.warn({ title: 'Thiếu lý do cập nhật', description: 'Nhập lý do để revision có lịch sử thay đổi rõ ràng.' });
      return;
    }
    if (releaseNow && !(draft?.files?.length || pendingFiles.length)) {
      toast.warn({ title: 'Thiếu tệp thiết kế', description: 'Cần ít nhất một tệp trước khi phát hành revision.' });
      return;
    }
    setSubmitting(true);
    try {
      let release = draft;
      const payload = {
        revision_label: revisionLabel.trim(),
        change_reason: changeReason.trim(),
        notes: notes.trim(),
        work_item_id: workItemId,
      };
      release = release
        ? await put<DesignRelease>(`/design-releases/${release.id}`, payload)
        : await post<DesignRelease>(`/projects/${projectId}/design-releases`, payload);

      for (const candidate of [...pendingFiles]) {
        const form = new FormData();
        form.append('file', candidate.file);
        form.append('name', candidate.file.name);
        form.append('document_type', candidate.documentType);
        await postForm(`/design-releases/${release.id}/files`, form);
        setPendingFiles((current) => current.filter((item) => item.key !== candidate.key));
      }

      if (releaseNow) {
        await post(`/design-releases/${release.id}/release`, {});
        toast.success(`Đã phát hành ${release.revision_label}`);
      } else {
        toast.success(`Đã lưu nháp ${release.revision_label}`);
      }
      await onChanged();
      onClose();
    } catch (error) {
      await onChanged();
      onClose();
      toast.error(releaseNow ? 'Không phát hành được revision thiết kế' : 'Không lưu được revision thiết kế', error);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog
      open={open}
      title={draft ? `Cập nhật ${draft.revision_label}` : 'Tạo revision thiết kế mới'}
      description="Gói revision có thể chứa nhiều tệp. Sau khi phát hành, nội dung được khóa để bảo toàn lịch sử."
      onClose={onClose}
      size="lg"
      footer={(
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" disabled={submitting} className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" onClick={onClose}>Hủy</button>
          <button type="button" disabled={submitting} className="rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60" onClick={() => void save(false)}>Lưu nháp</button>
          <button type="button" disabled={submitting} className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" onClick={() => void save(true)}>{submitting ? 'Đang lưu...' : 'Phát hành revision'}</button>
        </div>
      )}
    >
      <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <label className="block text-sm font-semibold text-slate-700">Mã revision
            <input className="mt-1 w-full rounded-md border border-line px-3 py-2 font-normal text-ink outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100" value={revisionLabel} maxLength={40} onChange={(event) => setRevisionLabel(event.target.value)} placeholder="Tự sinh, ví dụ R04" />
          </label>
          <label className="block text-sm font-semibold text-slate-700">Lý do cập nhật <span className="text-rose-600">*</span>
            <input className={`mt-1 w-full rounded-md border px-3 py-2 font-normal text-ink outline-none focus:ring-2 ${attempted && !changeReason.trim() ? 'border-rose-400 focus:ring-rose-100' : 'border-line focus:border-blue-400 focus:ring-blue-100'}`} value={changeReason} maxLength={1000} onChange={(event) => setChangeReason(event.target.value)} placeholder="Ví dụ: Cập nhật layout theo phản hồi client" />
            {attempted && !changeReason.trim() ? <span className="mt-1 block text-xs font-normal text-rose-600">Lý do cập nhật là bắt buộc.</span> : null}
          </label>
        </div>
        <label className="block text-sm font-semibold text-slate-700">Ghi chú
          <textarea className="mt-1 min-h-24 w-full rounded-md border border-line px-3 py-2 font-normal text-ink outline-none focus:border-blue-400 focus:ring-2 focus:ring-blue-100" value={notes} maxLength={4000} onChange={(event) => setNotes(event.target.value)} placeholder="Phạm vi thay đổi, lưu ý cho PM/QS..." />
        </label>

        <div className="rounded-lg border border-line p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="text-sm font-semibold text-ink">Tệp trong gói revision</p>
              <p className="mt-1 text-xs text-slate-500">Hỗ trợ PDF, PNG, JPG và WebP theo giới hạn tải tệp của hệ thống.</p>
            </div>
            <label className="inline-flex cursor-pointer items-center justify-center gap-2 rounded-md border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50">
              <Upload className="h-4 w-4" />Chọn nhiều tệp
              <input className="sr-only" type="file" multiple accept="application/pdf,image/png,image/jpeg,image/webp" onChange={(event) => {
                const selected = Array.from(event.target.files ?? []).map((file) => ({ key: `${file.name}-${file.size}-${file.lastModified}-${Math.random()}`, file, documentType: 'other' as const }));
                setPendingFiles((current) => [...current, ...selected]);
                event.target.value = '';
              }} />
            </label>
          </div>

          {draft?.files?.length ? (
            <div className="mt-3">
              <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">Đã tải lên</p>
              <DesignReleaseFiles files={draft.files} />
            </div>
          ) : null}

          {pendingFiles.length ? (
            <div className="mt-3 space-y-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Chờ tải lên</p>
              {pendingFiles.map((candidate) => (
                <div key={candidate.key} className="grid gap-2 rounded-md border border-line p-3 sm:grid-cols-[minmax(0,1fr)_13rem_auto] sm:items-center">
                  <div className="min-w-0"><p className="truncate text-sm font-semibold text-ink">{candidate.file.name}</p><p className="text-xs text-slate-500">{fileSizeText(candidate.file.size)}</p></div>
                  <select className="rounded-md border border-line bg-white px-3 py-2 text-sm" value={candidate.documentType} onChange={(event) => setPendingFiles((current) => current.map((item) => item.key === candidate.key ? { ...item, documentType: event.target.value as PendingDesignFile['documentType'] } : item))}>
                    {documentTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </select>
                  <button type="button" className="grid h-9 w-9 place-items-center rounded-md text-slate-500 hover:bg-slate-100" aria-label={`Bỏ tệp ${candidate.file.name}`} onClick={() => setPendingFiles((current) => current.filter((item) => item.key !== candidate.key))}><X className="h-4 w-4" /></button>
                </div>
              ))}
            </div>
          ) : !(draft?.files?.length) ? (
            <p className="mt-3 rounded-md border border-dashed border-slate-300 p-4 text-center text-sm text-slate-500">Chưa chọn tệp. Có thể lưu nháp trước, nhưng cần ít nhất một tệp để phát hành.</p>
          ) : null}
        </div>
      </div>
    </Dialog>
  );
}

function documentTypeLabel(value: string) {
  return documentTypeOptions.find((option) => option.value === value)?.label ?? 'Tài liệu khác';
}

function designReleaseStatusLabel(value: string) {
  if (value === 'released') return 'Hiện hành';
  if (value === 'draft') return 'Nháp';
  if (value === 'superseded') return 'Đã thay thế';
  return value;
}

function fileSizeText(value?: number) {
  const bytes = Number(value ?? 0);
  if (!bytes) return 'Không rõ dung lượng';
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
