import { useState } from 'react';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import type { EvidenceRecord } from '../../lib/types/domain';
import { isProtectedStorageProvider } from '../../lib/utils/protectedStorage';
import { EvidenceReferenceViewer, useEvidenceReferenceRecord } from '../_shared/EvidenceReferenceViewer';
import { isImageEvidence, useProtectedEvidencePreview } from '../_shared/EvidenceRecordViewerDialog';
import { safeEvidenceFileUrl } from '../_shared/evidenceReferenceMedia';
import { EvidenceCard, canPreviewEvidenceInApp, useEvidenceFileOpener } from '../work-items/EvidenceGallery';

// Compatibility adapter for construction references; the gallery owns the card UI.
export function ConstructionEvidenceButton({ evidenceId, name, label, record: supplied }: { evidenceId?: string; name?: string; label: string; record?: EvidenceRecord }) {
  const [viewerId, setViewerId] = useState<string | null>(null);
  const reference = { id: evidenceId, record: supplied, name: name || label };
  const { id, record, loading, error, retry } = useEvidenceReferenceRecord(reference);
  const protectedPhoto = record && isImageEvidence(record) && isProtectedStorageProvider(record.storage_provider) && record.storage_key;
  const preview = useProtectedEvidencePreview(record ?? null, protectedPhoto ? 'photo' : undefined);
  const openEvidenceFile = useEvidenceFileOpener();

  if (!id) return <EmptyState title={`Chưa có ${label.toLocaleLowerCase('vi-VN')}`} />;

  return <div className="min-w-0 space-y-2">
    <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
    {record ? <EvidenceCard
      record={record}
      previewUrl={protectedPhoto ? preview.previewUrl : isImageEvidence(record) ? safeEvidenceFileUrl(record.file_url) : undefined}
      previewLoading={preview.loading}
      previewError={preview.error}
      onRetry={preview.retry}
      showWorkItem={false}
      onSelect={() => setViewerId(id)}
      onOpen={() => canPreviewEvidenceInApp(record) ? setViewerId(id) : void openEvidenceFile(record)}
    /> : loading ? <LoadingSpinner label="Đang tải thông tin minh chứng…" /> : <EmptyState
      title={error || 'Không tải được thông tin minh chứng.'}
      action={<button type="button" className="rounded-md border border-line px-3 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-50" onClick={retry}>Thử lại</button>}
    />}
    {viewerId === id ? <EvidenceReferenceViewer reference={{ ...reference, record }} onClose={() => setViewerId(null)} /> : null}
  </div>;
}
