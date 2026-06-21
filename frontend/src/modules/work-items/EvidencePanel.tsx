import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ExternalLink, FileText, Image as ImageIcon, Loader2, Upload } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, postForm } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import { compactDateTime, evidenceLabel, visibilityLabel } from './workItemPresentation';

const maxEvidenceUploadBytes = 10 * 1024 * 1024;

type EvidencePanelProps = {
  workItemId: string;
  projectId?: string;
  evidence: EvidenceRecord[];
};

type EvidenceUploadType = 'photo' | 'document';
type EvidenceVisibility = 'internal' | 'project' | 'client';

export function EvidencePanel({ workItemId, projectId, evidence }: EvidencePanelProps) {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [evidenceType, setEvidenceType] = useState<EvidenceUploadType>('photo');
  const [visibility, setVisibility] = useState<EvidenceVisibility>('project');
  const [name, setName] = useState('');
  const [caption, setCaption] = useState('');
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const previewUrls = useEvidencePreviewUrls(evidence);
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
      if (fileInputRef.current) fileInputRef.current.value = '';
      queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
      queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      if (projectId) queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
      toast.success('Đã upload evidence');
    },
    onError: (error) => toast.error('Không upload được evidence', error),
  });

  function onFileChange(file: File | null) {
    setSelectedFile(file);
    if (file && !name.trim()) setName(file.name);
  }

  function submitUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedFile) {
      toast.warn({ title: 'Chưa chọn file', description: 'Evidence cần một file ảnh hoặc PDF.' });
      return;
    }
    if (selectedFile.size > maxEvidenceUploadBytes) {
      toast.warn({ title: 'File quá lớn', description: `Dung lượng tối đa ${formatFileSize(maxEvidenceUploadBytes)}.` });
      return;
    }
    if (selectedFile.type && evidenceType === 'photo' && !selectedFile.type.startsWith('image/')) {
      toast.warn({ title: 'Sai loại evidence', description: 'Photo evidence cần file ảnh PNG, JPG hoặc WebP.' });
      return;
    }
    if (selectedFile.type && evidenceType === 'document' && selectedFile.type !== 'application/pdf') {
      toast.warn({ title: 'Sai loại evidence', description: 'Document evidence hiện nhận file PDF.' });
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

  async function openEvidence(record: EvidenceRecord) {
    const previewUrl = previewUrls[record.id];
    if (previewUrl) {
      window.open(previewUrl, '_blank', 'noopener,noreferrer');
      return;
    }
    if (record.storage_provider === 'local' && record.storage_key) {
      const viewer = window.open('', '_blank', 'noopener,noreferrer');
      try {
        const blob = await downloadBlob(`/evidence/${record.id}/file`);
        const objectUrl = URL.createObjectURL(blob);
        if (viewer) viewer.location.href = objectUrl;
        else window.open(objectUrl, '_blank', 'noopener,noreferrer');
        window.setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
      } catch (error) {
        viewer?.close();
        toast.error('Không mở được file evidence', error);
      }
      return;
    }
    if (record.file_url) {
      window.open(record.file_url, '_blank', 'noopener,noreferrer');
      return;
    }
    toast.warn({ title: 'Evidence chưa có file', description: 'Record này không có file nội bộ hoặc URL để mở.' });
  }

  return (
    <section>
      <div className="mb-2 flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold text-ink">Evidence</h3>
        <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{evidence.length} file</span>
      </div>

      <form className="rounded-md border border-line bg-slate-50 p-3" onSubmit={submitUpload}>
        <div className="grid gap-2 sm:grid-cols-2">
          <label className="sm:col-span-2">
            <span className="text-xs font-semibold text-slate-600">File</span>
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
              <option value="photo">Photo</option>
              <option value="document">Document PDF</option>
            </select>
          </label>
          <label>
            <span className="text-xs font-semibold text-slate-600">Visibility</span>
            <select
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={visibility}
              onChange={(event) => setVisibility(event.target.value as EvidenceVisibility)}
            >
              <option value="internal">Internal</option>
              <option value="project">Project</option>
              <option value="client">Client</option>
            </select>
          </label>
          <label>
            <span className="text-xs font-semibold text-slate-600">Tên evidence</span>
            <input
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="Tên file / hạng mục"
            />
          </label>
          <label>
            <span className="text-xs font-semibold text-slate-600">Caption</span>
            <input
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={caption}
              onChange={(event) => setCaption(event.target.value)}
              placeholder="Ghi chú hiện trường"
            />
          </label>
        </div>
        <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
          <span className="text-xs text-slate-500">{selectedFile ? `${selectedFile.name} · ${formatFileSize(selectedFile.size)}` : 'PNG/JPG/WebP/PDF <= 10MB'}</span>
          <button
            type="submit"
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={uploadEvidence.isPending}
          >
            {uploadEvidence.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
            Upload
          </button>
        </div>
      </form>

      {evidence.length ? (
        <div className="mt-3 space-y-2">
          {evidence.map((record) => {
            const previewUrl = previewUrls[record.id];
            const title = evidenceTitle(record);
            return (
              <div key={record.id} className="rounded-md border border-line bg-white p-3">
                <div className="flex gap-3">
                  <div className="h-20 w-24 shrink-0 overflow-hidden rounded-md border border-line bg-slate-100">
                    {previewUrl ? (
                      <img className="h-full w-full object-cover" src={previewUrl} alt={title} />
                    ) : (
                      <div className="flex h-full w-full items-center justify-center text-slate-400">
                        {record.evidence_type === 'photo' ? <ImageIcon className="h-7 w-7" /> : <FileText className="h-7 w-7" />}
                      </div>
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-ink">{title}</p>
                        <p className="mt-1 text-xs text-slate-500">
                          {evidenceLabel(record.evidence_type)} · {visibilityLabel(record.visibility ?? 'project')} · {compactDateTime(record.created_at)}
                        </p>
                      </div>
                      <StatusBadge status={record.status} />
                    </div>
                    <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-slate-500">
                      {record.mime_type ? <span>{record.mime_type}</span> : null}
                      {record.file_size ? <span>{formatFileSize(record.file_size)}</span> : null}
                      {record.checksum_sha256 ? <span>sha256:{record.checksum_sha256.slice(0, 12)}</span> : null}
                    </div>
                    <div className="mt-3 flex items-center justify-between gap-2">
                      <span className={clsx('rounded-md px-2 py-0.5 text-xs font-semibold', record.storage_provider === 'local' ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600')}>
                        {record.storage_provider === 'local' ? 'Protected file' : 'External link'}
                      </span>
                      <button
                        type="button"
                        className="inline-flex items-center gap-1 rounded-md border border-line px-2 py-1 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
                        onClick={() => void openEvidence(record)}
                      >
                        <ExternalLink className="h-3.5 w-3.5" />
                        Mở
                      </button>
                    </div>
                  </div>
                </div>
                {record.caption ? <p className="mt-2 text-sm leading-5 text-slate-600">{record.caption}</p> : null}
              </div>
            );
          })}
        </div>
      ) : (
        <div className="mt-3">
          <EmptyState title="Chưa có evidence cho work item này" />
        </div>
      )}
    </section>
  );
}

function useEvidencePreviewUrls(evidence: EvidenceRecord[]) {
  const [urls, setUrls] = useState<Record<string, string>>({});
  const signature = useMemo(
    () => evidence.map((record) => `${record.id}:${record.storage_key ?? ''}:${record.updated_at ?? ''}:${record.mime_type ?? ''}`).join('|'),
    [evidence],
  );

  useEffect(() => {
    let cancelled = false;
    const objectUrls: string[] = [];
    setUrls({});

    async function loadPreviews() {
      const imageRecords = evidence.filter((record) => record.storage_provider === 'local' && record.storage_key && isImageEvidence(record));
      const entries = await Promise.all(imageRecords.map(async (record) => {
        try {
          const blob = await downloadBlob(`/evidence/${record.id}/file`);
          const objectUrl = URL.createObjectURL(blob);
          objectUrls.push(objectUrl);
          return [record.id, objectUrl] as const;
        } catch {
          return null;
        }
      }));
      if (!cancelled) {
        setUrls(Object.fromEntries(entries.filter(Boolean) as Array<readonly [string, string]>));
      } else {
        objectUrls.forEach((url) => URL.revokeObjectURL(url));
      }
    }

    void loadPreviews();
    return () => {
      cancelled = true;
      objectUrls.forEach((url) => URL.revokeObjectURL(url));
    };
  }, [signature]);

  return urls;
}

function isImageEvidence(record: EvidenceRecord) {
  return record.evidence_type === 'photo' || String(record.mime_type ?? '').startsWith('image/');
}

function evidenceTitle(record: EvidenceRecord) {
  return record.name ?? record.title ?? record.document_name ?? record.id;
}

function formatFileSize(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '-';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}
