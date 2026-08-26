import { Eye, FileImage } from 'lucide-react';
import { useState } from 'react';
import { EvidenceReferenceViewer } from '../_shared/EvidenceReferenceViewer';

export function ConstructionEvidenceButton({ evidenceId, name, label }: { evidenceId?: string; name?: string; label: string }) {
  const [viewerOpen, setViewerOpen] = useState(false);

  if (!evidenceId) return <div className="rounded-md border border-dashed border-slate-300 px-3 py-4 text-sm text-slate-500">Chưa có {label.toLocaleLowerCase('vi-VN')}</div>;

  return <>
    <button type="button" className="flex w-full items-center gap-3 rounded-md border border-line bg-slate-50 px-3 py-3 text-left hover:border-blue-200 hover:bg-blue-50" onClick={() => setViewerOpen(true)}>
      <span className="grid h-10 w-10 shrink-0 place-items-center rounded-md bg-white text-blue-600 ring-1 ring-inset ring-line"><FileImage className="h-5 w-5" /></span>
      <span className="min-w-0 flex-1"><span className="block text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</span><span className="mt-0.5 block truncate text-sm font-medium text-ink">{name || 'Minh chứng'}</span></span>
      <span className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-blue-700"><Eye className="h-4 w-4" /> Xem</span>
    </button>
    <EvidenceReferenceViewer reference={viewerOpen ? { id: evidenceId, name: name || label } : null} onClose={() => setViewerOpen(false)} />
  </>;
}
