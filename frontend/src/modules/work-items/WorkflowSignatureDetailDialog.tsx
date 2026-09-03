import { useEffect, useRef, useState } from 'react';
import { ArrowLeft, Check, ChevronDown, Copy, FileText, LockKeyhole } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';
import type { WorkflowFormSignature } from '../../lib/types/domain';
import { confirmationRolesText, fileSizeText, signatureDateTime, signatureIsHistorical, signaturePurposeLabel } from './workflowConfirmationPresentation';
import { WorkflowSignatureVisual, useWorkflowSignaturePreview } from './WorkflowSignatureVisual';

export type WorkflowSignatureViewMode = 'detail' | 'visual';

export function WorkflowSignatureDetailDialog({ signature, formTitle, currentRevision, initialMode, onClose }: {
  signature: WorkflowFormSignature;
  formTitle: string;
  currentRevision: number;
  initialMode: WorkflowSignatureViewMode;
  onClose: () => void;
}) {
  const [mode, setMode] = useState(initialMode);
  const [auditOpen, setAuditOpen] = useState(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const previousMode = useRef(initialMode);
  useEffect(() => {
    if (previousMode.current === mode) return;
    previousMode.current = mode;
    const frame = window.requestAnimationFrame(() => {
      const target = contentRef.current?.querySelector<HTMLElement>('[data-signature-mode-focus]:not(:disabled)') ?? contentRef.current;
      target?.focus();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [mode]);
  const preview = useWorkflowSignaturePreview(signature);
  const visual = mode === 'visual';
  const historical = signatureIsHistorical(signature, currentRevision);
  const name = signature.signer_name || 'Chưa có thông tin người ký';
  const fileDescription = [
    signature.visual_mime_type,
    signature.visual_file_size != null ? fileSizeText(signature.visual_file_size) : undefined,
  ].filter(Boolean).join(' · ') || 'Chưa có thông tin tệp';

  return (
    <Dialog
      open
      manageFocus
      title={visual ? 'Dấu ký' : 'Chi tiết chữ ký'}
      description={visual
        ? `${name} · ${signaturePurposeLabel(signature.purpose)} · Hồ sơ phiên bản ${signature.form_revision_no}${historical ? ' · Phiên bản trước' : ''}`
        : `${formTitle} · Hồ sơ phiên bản ${signature.form_revision_no}`}
      size={visual ? 'xl' : 'lg'}
      onClose={onClose}
      footer={(
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-xs text-slate-500">{visual ? `Ký lúc ${signatureDateTime(signature.signed_at)}` : historical ? 'Chữ ký thuộc phiên bản hồ sơ trước.' : 'Chữ ký gắn với phiên bản hồ sơ đã ký.'}</p>
          <button type="button" className="ml-auto rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>
        </div>
      )}
    >
      <div ref={contentRef} tabIndex={-1} className="outline-none">
        {visual ? (
          <div className="space-y-3">
            <button type="button" data-signature-mode-focus className="inline-flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={() => setMode('detail')}>
              {initialMode === 'detail' ? <ArrowLeft className="h-4 w-4" /> : <FileText className="h-4 w-4" />}
              {initialMode === 'detail' ? 'Quay lại chi tiết' : 'Chi tiết chữ ký'}
            </button>
            <WorkflowSignatureVisual preview={preview} signerName={name} variant="full" />
          </div>
        ) : (
          <div className="space-y-5">
            <div className="grid gap-5 sm:grid-cols-2">
              <WorkflowSignatureVisual preview={preview} signerName={name} onEnlarge={() => setMode('visual')} />
              <div className="min-w-0">
                <div className="flex flex-wrap items-start gap-2">
                  <p className="w-full break-all text-base font-semibold text-ink">{name}</p>
                  <span className="inline-flex items-center gap-1 rounded bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700"><Check className="h-3.5 w-3.5" />Đã ký</span>
                  {historical ? <span className="rounded bg-amber-50 px-2 py-0.5 text-xs font-semibold text-amber-800">Phiên bản trước</span> : null}
                </div>
                <p className="mt-2 break-words text-sm text-slate-600">{confirmationRolesText(signature.signer_roles_json)}</p>
                <dl className="mt-5 space-y-4">
                  <DetailField label="Mục đích" value={signaturePurposeLabel(signature.purpose)} />
                  <DetailField label="Thời điểm ký" value={signatureDateTime(signature.signed_at)} />
                  <DetailField label="Phiên bản hồ sơ" value={`Phiên bản ${signature.form_revision_no}`} />
                </dl>
              </div>
            </div>
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Ghi chú</p>
              <p className="mt-1 whitespace-pre-wrap break-words text-sm leading-6 text-slate-700">{signature.comment || 'Không có ghi chú.'}</p>
            </div>
            <p className="flex items-start gap-2 text-xs leading-5 text-slate-500"><LockKeyhole className="mt-0.5 h-4 w-4 shrink-0" />Bản ghi chữ ký được lưu cố định theo phiên bản hồ sơ.</p>
            <details className="group border-t border-line" open={auditOpen} onToggle={(event) => setAuditOpen(event.currentTarget.open)}>
              <summary className="flex cursor-pointer list-none items-center justify-between gap-2 py-3 text-sm font-semibold text-slate-700">Thông tin đối chiếu<ChevronDown className="h-4 w-4 transition group-open:rotate-180" /></summary>
              <div className="space-y-3 pb-1">
                <AuditValue label="ID người ký" value={signature.signer_user_id} />
                <AuditValue label="Mã nội dung" value={signature.payload_hash} />
                <AuditValue label="SHA-256 dấu ký" value={signature.visual_checksum_sha256} />
                <dl className="grid gap-4 pt-1 sm:grid-cols-2">
                  <DetailField label="Tệp dấu ký" value={fileDescription} />
                  <DetailField label="Nơi lưu" value={signature.visual_storage_provider || 'Chưa có thông tin'} />
                </dl>
              </div>
            </details>
          </div>
        )}
      </div>
    </Dialog>
  );
}

function DetailField({ label, value }: { label: string; value: string }) {
  return <div className="min-w-0"><dt className="text-xs text-slate-500">{label}</dt><dd className="mt-1 break-words text-sm font-medium text-slate-800">{value}</dd></div>;
}

function AuditValue({ label, value }: { label: string; value?: string }) {
  const toast = useToast();
  async function copy() {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      toast.success(`Đã sao chép ${label.toLocaleLowerCase('vi-VN')}`);
    } catch (error) {
      toast.error('Không sao chép được thông tin đối chiếu', error);
    }
  }
  return (
    <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs font-medium text-slate-600">{label}</p>
        {value ? <button type="button" aria-label={`Sao chép ${label.toLocaleLowerCase('vi-VN')}`} className="inline-flex items-center gap-1 rounded px-2 py-1 text-xs font-semibold text-blue-700 hover:bg-blue-100" onClick={() => void copy()}><Copy className="h-3.5 w-3.5" />Sao chép</button> : null}
      </div>
      <p className="mt-1 select-text break-all font-mono text-xs leading-5 text-slate-700">{value || 'Chưa có thông tin'}</p>
    </div>
  );
}
