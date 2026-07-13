import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { useToast } from '../../components/ToastProvider';
import { get } from '../../lib/api/client';
import type { ProjectOperationsResponse, WorkItem } from '../../lib/types/domain';
import { statusLabel } from '../../lib/utils/status';
import { ConstructionEvidenceCapture, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';
import { constructionReportViewModel } from './constructionReportViewModel';

type ConstructionReportEvidenceDialogProps = {
  open: boolean;
  projectId: string;
  record: Record<string, any> | null;
  onClose: () => void;
  onSubmit: (workItemId: string, uploads: ConstructionEvidenceUploadDraft[]) => Promise<unknown>;
};

export function ConstructionReportEvidenceDialog({ open, projectId, record, onClose, onSubmit }: ConstructionReportEvidenceDialogProps) {
  const toast = useToast();
  const report = useMemo(() => constructionReportViewModel(record ?? {}), [record]);
  const [workItemId, setWorkItemId] = useState('');
  const [uploads, setUploads] = useState<ConstructionEvidenceUploadDraft[]>([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!open) return;
    setWorkItemId(report.workItemId);
    setUploads([]);
    setSaving(false);
  }, [open, report.workItemId]);

  const operations = useQuery({
    queryKey: ['project-operations', projectId, 'full'],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${projectId}/operations`),
    enabled: open && Boolean(projectId),
    retry: false,
  });
  const workItems = useMemo(
    () => [...(operations.data?.work_items ?? [])]
      .filter((item) => item.status !== 'cancelled')
      .sort((left, right) => workItemRank(left) - workItemRank(right) || left.title.localeCompare(right.title, 'vi')),
    [operations.data?.work_items],
  );

  async function save() {
    if (!workItemId) {
      toast.warn({ title: 'Chưa chọn công việc liên quan', description: 'Minh chứng thi công phải được liên kết với một công việc trong công trình.' });
      return;
    }
    if (!uploads.length) {
      toast.warn({ title: 'Chưa có minh chứng mới', description: 'Chọn tệp hoặc chụp ít nhất một ảnh trước khi lưu.' });
      return;
    }
    setSaving(true);
    try {
      await onSubmit(workItemId, uploads);
    } catch {
      // Mutation handlers own the user-facing error toast.
    } finally {
      setSaving(false);
    }
  }

  return <Dialog
    open={open}
    title="Thêm minh chứng báo cáo"
    description={`Báo cáo hiện có ${report.evidenceIds.length} minh chứng. Tệp mới được lưu theo công việc liên quan và liên kết với báo cáo.`}
    size="lg"
    onClose={onClose}
    footer={<div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end"><button type="button" disabled={saving} className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" onClick={onClose}>Hủy</button><button type="button" disabled={saving || !uploads.length} className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" onClick={save}>{saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}{saving ? 'Đang tải...' : `Tải ${uploads.length || ''} minh chứng`}</button></div>}
  >
    <div className="space-y-4">
      <section className="rounded-lg border border-line bg-slate-50 p-4">
        <label className="block"><span className="text-sm font-semibold text-slate-700">Công việc liên quan <span className="text-rose-600">*</span></span>
          {operations.isLoading ? <div className="mt-2"><LoadingSpinner label="Đang tải danh sách công việc..." /></div> : null}
          {operations.isError ? <p className="mt-2 text-sm text-rose-700">Không tải được danh sách công việc. Vui lòng thử lại.</p> : null}
          {operations.isSuccess ? <select className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100" value={workItemId} disabled={Boolean(report.workItemId)} onChange={(event) => setWorkItemId(event.target.value)}><option value="">Chọn công việc</option>{workItems.map((item) => <option key={item.id} value={item.id}>{item.title} · {statusLabel(item.status)}</option>)}</select> : null}
          <p className="mt-1 text-xs text-slate-500">{report.workItemId ? 'Báo cáo đã liên kết với công việc này nên không thể đổi tại bước thêm minh chứng.' : 'Công việc được liên kết với báo cáo sau lần tải đầu tiên.'}</p>
        </label>
      </section>
      <ConstructionEvidenceCapture drafts={uploads} onChange={setUploads} hasSource={Boolean(workItemId)} disabled={saving} compact />
    </div>
  </Dialog>;
}

function workItemRank(item: WorkItem) {
  return ['ready', 'in_progress', 'needs_changes', 'blocked'].includes(item.status) ? 0 : ['waiting_review', 'waiting_approval'].includes(item.status) ? 1 : 2;
}
