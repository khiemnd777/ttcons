import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { ExternalLink, Eye } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';
import { downloadBlob } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import type { EvidenceMediaType } from './evidenceMedia';
import { EvidenceViewerDialog } from './EvidenceViewerDialog';

export type EvidenceFileReference = {
  id?: string;
  name?: string;
  filePath?: string;
  fileUrl?: string;
  mimeType?: string;
  evidenceType?: string;
  metadata?: string[];
  headerStatus?: ReactNode;
};

type EvidenceReferenceViewerProps = {
  reference: EvidenceFileReference | null;
  onClose: () => void;
};

type PreviewState = {
  blob?: Blob;
  objectUrl?: string;
  loading: boolean;
  error?: string;
};

export function EvidenceReferenceViewer({ reference, onClose }: EvidenceReferenceViewerProps) {
  const [retryKey, setRetryKey] = useState(0);
  const [state, setState] = useState<PreviewState>({ loading: false });
  const filePath = reference?.filePath || (reference?.id ? `/evidence/${reference.id}/file` : '');
  const externalUrl = safeEvidenceFileUrl(reference?.fileUrl);
  const initialMediaType = useMemo(
    () => evidenceReferenceMediaType(reference?.name, reference?.mimeType, reference?.evidenceType),
    [reference?.evidenceType, reference?.mimeType, reference?.name],
  );

  useEffect(() => {
    if (!reference || !filePath) {
      setState({ loading: false });
      return undefined;
    }

    let cancelled = false;
    let objectUrl = '';
    setState({ loading: true });
    void downloadBlob(filePath)
      .then((blob) => {
        if (cancelled) return;
        objectUrl = URL.createObjectURL(blob);
        setState({ blob, objectUrl, loading: false });
      })
      .catch((error) => {
        if (cancelled) return;
        const message = error instanceof Error ? error.message : 'Vui lòng thử lại.';
        setState({ loading: false, error: `Không tải được minh chứng. ${message}` });
      });

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [filePath, reference?.id, retryKey]);

  if (!reference) return null;
  const mediaType = state.blob
    ? evidenceReferenceMediaType(reference.name, state.blob.type || reference.mimeType, reference.evidenceType)
    : initialMediaType;
  const previewUrl = state.objectUrl || (!filePath ? externalUrl : undefined);
  const originalUrl = state.objectUrl || externalUrl;
  const legacyOffice = isLegacyOffice(reference.name, reference.mimeType);
  const mediaAction = originalUrl ? (
    <a
      href={originalUrl}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
    >
      <ExternalLink className="h-4 w-4" />
      Mở tệp gốc
    </a>
  ) : undefined;

  return (
    <EvidenceViewerDialog
      open
      name={reference.name || 'Minh chứng'}
      previewUrl={mediaType === 'photo' || mediaType === 'pdf' ? previewUrl : undefined}
      fileBlob={mediaType === 'word' || mediaType === 'spreadsheet' ? state.blob : undefined}
      mediaType={mediaType}
      loading={state.loading}
      error={state.error}
      metadata={[
        evidenceMediaLabel(mediaType, legacyOffice),
        state.blob?.type || reference.mimeType || '',
        ...(reference.metadata ?? []),
      ].filter(Boolean)}
      description={filePath ? 'Tệp được tải qua API có kiểm tra quyền truy cập hiện tại.' : 'Minh chứng dùng liên kết ngoài được lưu trong dữ liệu lịch sử.'}
      headerStatus={reference.headerStatus}
      mediaAction={mediaAction}
      documentUnavailableText={legacyOffice ? 'Định dạng Office đời cũ chưa hỗ trợ đọc trực tiếp. Hãy mở tệp gốc bằng ứng dụng tương thích.' : undefined}
      onRetry={filePath ? () => setRetryKey((current) => current + 1) : undefined}
      onClose={onClose}
    />
  );
}

export function SelectedEvidenceViewButton({ evidenceId, evidence, label = 'Xem' }: { evidenceId?: string; evidence: EvidenceRecord[]; label?: string }) {
  const record = evidence.find((item) => String(item.id) === String(evidenceId));
  if (!evidenceId) return null;
  const name = record?.name || record?.title || String(evidenceId);
  return <EvidenceReferenceViewButton label={label} reference={{
    id: String(evidenceId),
    name,
    mimeType: record?.mime_type,
    evidenceType: record?.evidence_type,
    metadata: record?.work_item_title ? [record.work_item_title] : undefined,
    headerStatus: record?.status ? <StatusBadge status={record.status} /> : undefined,
  }} />;
}

export function EvidenceReferenceViewButton({ reference, label = 'Xem' }: { reference: EvidenceFileReference; label?: string }) {
  const [open, setOpen] = useState(false);
  const name = reference.name || reference.id || 'minh chứng';
  return <>
    <button
      type="button"
      className="inline-flex items-center gap-1.5 rounded-md border border-line bg-white px-2.5 py-1.5 text-xs font-semibold text-blue-700 hover:border-blue-200 hover:bg-blue-50"
      onClick={() => setOpen(true)}
      aria-label={`Xem minh chứng ${name}`}
    >
      <Eye className="h-3.5 w-3.5" /> {label}
    </button>
    <EvidenceReferenceViewer
      reference={open ? reference : null}
      onClose={() => setOpen(false)}
    />
  </>;
}

export function evidenceReferenceMediaType(name?: string, mimeType?: string, evidenceType?: string): EvidenceMediaType {
  const mime = String(mimeType ?? '').trim().toLowerCase().split(';', 1)[0];
  const filename = String(name ?? '').trim().toLowerCase().split(/[?#]/, 1)[0];
  if (mime.includes('pdf') || filename.endsWith('.pdf')) return 'pdf';
  if (mime === 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' || filename.endsWith('.docx')) return 'word';
  if (mime === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' || filename.endsWith('.xlsx')) return 'spreadsheet';
  if (mime.startsWith('image/') || /\.(png|jpe?g|webp|gif)$/.test(filename) || evidenceType === 'photo') return 'photo';
  return 'document';
}

function isLegacyOffice(name?: string, mimeType?: string) {
  const mime = String(mimeType ?? '').trim().toLowerCase().split(';', 1)[0];
  const filename = String(name ?? '').trim().toLowerCase().split(/[?#]/, 1)[0];
  return mime === 'application/msword'
    || mime === 'application/vnd.ms-excel'
    || filename.endsWith('.doc')
    || filename.endsWith('.xls');
}

function evidenceMediaLabel(mediaType: EvidenceMediaType, legacyOffice: boolean) {
  if (mediaType === 'photo') return 'Ảnh';
  if (mediaType === 'pdf') return 'PDF';
  if (mediaType === 'word' || legacyOffice) return 'Word';
  if (mediaType === 'spreadsheet') return 'Excel';
  return 'Tài liệu';
}

function safeEvidenceFileUrl(value?: string) {
  const candidate = String(value ?? '').trim();
  if (!candidate) return undefined;
  if (candidate.startsWith('/') && !candidate.startsWith('//')) return candidate;
  try {
    const parsed = new URL(candidate, window.location.origin);
    if (parsed.protocol === 'http:' || parsed.protocol === 'https:' || parsed.protocol === 'blob:') return parsed.href;
    if (parsed.protocol === 'data:' && candidate.toLowerCase().startsWith('data:image/')) return candidate;
  } catch {
    return undefined;
  }
  return undefined;
}
