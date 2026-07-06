import { FormEvent, ReactNode, useMemo, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Clock3, FileText, Image as ImageIcon, Loader2, Upload } from 'lucide-react';
import clsx from 'clsx';
import { useToast } from '../../components/ToastProvider';
import { post, postForm } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import { EvidenceGallery, formatEvidenceFileSize, isImageEvidence, type EvidenceReviewAction } from './EvidenceGallery';

const maxEvidenceUploadBytes = 10 * 1024 * 1024;

type EvidencePanelProps = {
  workItemId: string;
  projectId?: string;
  evidence: EvidenceRecord[];
};

type EvidenceUploadType = 'photo' | 'document';
type EvidenceVisibility = 'internal' | 'project' | 'client';

export function EvidencePanel({ workItemId, projectId, evidence }: EvidencePanelProps) {
  const [uploadOpen, setUploadOpen] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [evidenceType, setEvidenceType] = useState<EvidenceUploadType>('photo');
  const [visibility, setVisibility] = useState<EvidenceVisibility>('project');
  const [name, setName] = useState('');
  const [caption, setCaption] = useState('');
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const stats = useMemo(() => evidenceStats(evidence), [evidence]);
  const queryClient = useQueryClient();
  const toast = useToast();

  const uploadEvidence = useMutation({
    mutationFn: (formData: FormData) => postForm<EvidenceRecord>(`/work-items/${workItemId}/evidence/upload`, formData),
    onSuccess: () => {
      setSelectedFile(null);
      setName('');
      setCaption('');
      setEvidenceType('photo');
      setVisibility('project');
      setUploadOpen(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
      queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
      queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      if (projectId) queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
      toast.success('Đã tải minh chứng');
    },
    onError: (error) => toast.error('Không tải được minh chứng', error),
  });

  const reviewEvidence = useMutation({
    mutationFn: ({ record, action, comment }: { record: EvidenceRecord; action: EvidenceReviewAction; comment: string }) =>
      post<EvidenceRecord>(`/evidence/${record.id}/${action}`, { comment, reason: action === 'reject' ? comment : undefined }),
    onSuccess: (_record, variables) => {
      queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
      queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      if (projectId) queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
      toast.success(variables.action === 'accept' ? 'Đã chấp nhận minh chứng' : 'Đã từ chối minh chứng');
    },
    onError: (error) => toast.error('Không rà soát được minh chứng', error),
  });

  function onFileChange(file: File | null) {
    setSelectedFile(file);
    if (file && !name.trim()) setName(file.name);
  }

  function submitUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedFile) {
      toast.warn({ title: 'Chưa chọn tệp', description: 'Minh chứng cần một tệp ảnh hoặc PDF.' });
      return;
    }
    if (selectedFile.size > maxEvidenceUploadBytes) {
      toast.warn({ title: 'Tệp quá lớn', description: `Dung lượng tối đa ${formatEvidenceFileSize(maxEvidenceUploadBytes)}.` });
      return;
    }
    if (selectedFile.type && evidenceType === 'photo' && !selectedFile.type.startsWith('image/')) {
      toast.warn({ title: 'Sai loại minh chứng', description: 'Ảnh minh chứng cần tệp PNG, JPG hoặc WebP.' });
      return;
    }
    if (selectedFile.type && evidenceType === 'document' && selectedFile.type !== 'application/pdf') {
      toast.warn({ title: 'Sai loại minh chứng', description: 'Tài liệu minh chứng hiện nhận tệp PDF.' });
      return;
    }
    const formData = new FormData();
    formData.append('file', selectedFile);
    formData.append('evidence_type', evidenceType);
    formData.append('visibility', visibility);
    formData.append('name', name.trim() || selectedFile.name);
    if (caption.trim()) formData.append('caption', caption.trim());
    uploadEvidence.mutate(formData);
  }

  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 rounded-md border border-line bg-slate-50 px-3 py-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Sổ minh chứng</p>
          <h3 className="mt-0.5 text-sm font-semibold text-ink">{evidence.length} hồ sơ minh chứng</h3>
          <p className="mt-1 text-xs text-slate-500">Ảnh {stats.photos} · Tài liệu {stats.documents} · Đang chờ {stats.pending}</p>
        </div>
        <button
          type="button"
          className={clsx(
            'inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-semibold',
            uploadOpen ? 'border border-line bg-white text-slate-700 hover:bg-slate-100' : 'bg-blue-600 text-white hover:bg-blue-700',
          )}
          onClick={() => setUploadOpen((current) => !current)}
        >
          <Upload className="h-4 w-4" />
          {uploadOpen ? 'Ẩn tải lên' : 'Tải minh chứng'}
        </button>
      </div>

      <div className="grid gap-2 sm:grid-cols-4">
        <EvidenceStat icon={<ImageIcon className="h-3.5 w-3.5" />} label="Ảnh" value={stats.photos} />
        <EvidenceStat icon={<FileText className="h-3.5 w-3.5" />} label="Tài liệu" value={stats.documents} />
        <EvidenceStat icon={<CheckCircle2 className="h-3.5 w-3.5" />} label="Đã chấp nhận" value={stats.accepted} tone="ok" />
        <EvidenceStat icon={<Clock3 className="h-3.5 w-3.5" />} label="Đang chờ" value={stats.pending} tone={stats.pending ? 'warn' : 'neutral'} />
      </div>

      {uploadOpen ? (
        <form className="rounded-md border border-blue-100 bg-blue-50/40 p-3" onSubmit={submitUpload}>
          <div className="grid gap-2 lg:grid-cols-[minmax(0,1.4fr)_9rem_9rem_minmax(0,1fr)_minmax(0,1fr)]">
            <label className="min-w-0">
              <span className="text-xs font-semibold text-slate-600">Tệp</span>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/png,image/jpeg,image/webp,application/pdf"
                className="mt-1 block w-full rounded-md border border-line bg-white px-3 py-2 text-sm file:mr-3 file:rounded-md file:border-0 file:bg-blue-50 file:px-3 file:py-1.5 file:text-sm file:font-semibold file:text-blue-700 hover:file:bg-blue-100"
                onChange={(event) => onFileChange(event.target.files?.[0] ?? null)}
              />
            </label>
            <label>
              <span className="text-xs font-semibold text-slate-600">Loại</span>
              <select
                className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                value={evidenceType}
                onChange={(event) => setEvidenceType(event.target.value as EvidenceUploadType)}
              >
                <option value="photo">Ảnh</option>
                <option value="document">Tài liệu PDF</option>
              </select>
            </label>
            <label>
              <span className="text-xs font-semibold text-slate-600">Phạm vi hiển thị</span>
              <select
                className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                value={visibility}
                onChange={(event) => setVisibility(event.target.value as EvidenceVisibility)}
              >
                <option value="internal">Nội bộ</option>
                <option value="project">Công trình</option>
                <option value="client">Chủ đầu tư</option>
              </select>
            </label>
            <label>
              <span className="text-xs font-semibold text-slate-600">Tên minh chứng</span>
              <input
                className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="Tên file / hạng mục"
              />
            </label>
            <label>
              <span className="text-xs font-semibold text-slate-600">Ghi chú</span>
              <input
                className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                value={caption}
                onChange={(event) => setCaption(event.target.value)}
                placeholder="Ghi chú hiện trường"
              />
            </label>
          </div>
          <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
            <span className="text-xs text-slate-500">{selectedFile ? `${selectedFile.name} · ${formatEvidenceFileSize(selectedFile.size)}` : 'PNG/JPG/WebP/PDF <= 10MB'}</span>
            <button
              type="submit"
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={uploadEvidence.isPending}
            >
              {uploadEvidence.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
              Tải lên
            </button>
          </div>
        </form>
      ) : null}

      <div>
        <EvidenceGallery
          evidence={evidence}
          emptyTitle="Chưa có minh chứng cho đầu việc này"
          variant="list"
          onReview={(record, action, comment) => reviewEvidence.mutateAsync({ record, action, comment }).then(() => undefined)}
        />
      </div>
    </section>
  );
}

function evidenceStats(evidence: EvidenceRecord[]) {
  return {
    photos: evidence.filter(isImageEvidence).length,
    documents: evidence.filter((record) => record.evidence_type === 'document' || String(record.mime_type ?? '').includes('pdf')).length,
    accepted: evidence.filter((record) => record.status === 'accepted').length,
    pending: evidence.filter((record) => ['draft', 'submitted', 'pending'].includes(record.status)).length,
  };
}

function EvidenceStat({
  icon,
  label,
  value,
  tone = 'neutral',
}: {
  icon: ReactNode;
  label: string;
  value: number;
  tone?: 'neutral' | 'ok' | 'warn';
}) {
  return (
    <div className={clsx(
      'flex min-w-0 items-center gap-2 rounded-md px-2.5 py-2 text-xs font-semibold ring-1 ring-inset',
      tone === 'ok' ? 'bg-emerald-50 text-emerald-700 ring-emerald-200' : tone === 'warn' ? 'bg-amber-50 text-amber-800 ring-amber-200' : 'bg-white text-slate-600 ring-line',
    )}>
      <span className="shrink-0">{icon}</span>
      <span className="min-w-0">
        <span className="block text-[11px] uppercase tracking-wide opacity-75">{label}</span>
        <span className="block">{value}</span>
      </span>
    </div>
  );
}
