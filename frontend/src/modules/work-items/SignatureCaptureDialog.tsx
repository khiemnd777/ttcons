import { PointerEvent, type ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { AlertTriangle, Eraser, Loader2, PenLine, ShieldCheck, Stamp, Upload } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import type { SealAsset } from '../../lib/types/domain';
import { SealAssetPreview } from './SealAssetPreview';

export type SignaturePoint = { x: number; y: number };
export type SignatureStroke = { points: SignaturePoint[] };
export type ConfirmationKind = 'signature' | 'seal';

export type SignatureCapturePayload = {
  purpose: string;
  comment: string;
  signature_image_data_url: string;
  signature_strokes: SignatureStroke[];
};

export type WorkflowSealApplicationPayload = {
  purpose: string;
  comment: string;
  seal_asset_id: string;
};

type SignatureCaptureDialogProps = {
  open: boolean;
  signerName?: string;
  submitting: boolean;
  signedPurposes?: string[];
  allowedPurposes?: string[];
  defaultPurpose?: string;
  purposeLocked?: boolean;
  formRevision?: number;
  allowedKinds?: ConfirmationKind[];
  sealAssets?: SealAsset[];
  sealAssetsLoading?: boolean;
  appliedSealKeys?: string[];
  canManageSeals?: boolean;
  uploadingSeal?: boolean;
  onClose: () => void;
  onSubmit: (payload: SignatureCapturePayload) => void;
  onApplySeal?: (payload: WorkflowSealApplicationPayload) => void;
  onUploadSeal?: (name: string, file: File) => Promise<SealAsset>;
  onValidationWarning?: (message: string) => void;
};

const canvasWidth = 640;
const canvasHeight = 220;
const maxSealFileSize = 10 * 1024 * 1024;
const acceptedSealTypes = new Set(['image/png', 'image/jpeg', 'image/webp']);
const signaturePurposeOptions = [
  { value: 'internal_ack', label: 'Xác nhận nội bộ' },
  { value: 'prepared_by', label: 'Người lập' },
  { value: 'reviewed_by', label: 'Người rà soát' },
  { value: 'approved_by', label: 'Người phê duyệt' },
  { value: 'received_by', label: 'Người nhận' },
];

export function SignatureCaptureDialog({
  open,
  signerName,
  submitting,
  signedPurposes = [],
  allowedPurposes,
  defaultPurpose = 'internal_ack',
  purposeLocked = false,
  formRevision,
  allowedKinds = ['signature'],
  sealAssets = [],
  sealAssetsLoading = false,
  appliedSealKeys = [],
  canManageSeals = false,
  uploadingSeal = false,
  onClose,
  onSubmit,
  onApplySeal,
  onUploadSeal,
  onValidationWarning,
}: SignatureCaptureDialogProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const signatureAllowed = allowedKinds.includes('signature');
  const sealAllowed = allowedKinds.includes('seal') && Boolean(onApplySeal);
  const enabledKinds = useMemo(
    () => [signatureAllowed ? 'signature' : null, sealAllowed ? 'seal' : null].filter(Boolean) as ConfirmationKind[],
    [sealAllowed, signatureAllowed],
  );
  const [kind, setKind] = useState<ConfirmationKind>(enabledKinds[0] ?? 'signature');
  const [purpose, setPurpose] = useState(defaultPurpose);
  const [comment, setComment] = useState('');
  const [strokes, setStrokes] = useState<SignatureStroke[]>([]);
  const [activeStroke, setActiveStroke] = useState<SignaturePoint[] | null>(null);
  const [selectedSealAssetId, setSelectedSealAssetId] = useState('');
  const [sealName, setSealName] = useState('');
  const [sealFile, setSealFile] = useState<File | null>(null);
  const [uploadError, setUploadError] = useState('');
  const [acknowledgedImmutable, setAcknowledgedImmutable] = useState(false);
  const signedPurposeSet = useMemo(() => new Set(signedPurposes), [signedPurposes]);
  const appliedSealKeySet = useMemo(() => new Set(appliedSealKeys), [appliedSealKeys]);
  const purposeAlreadySigned = kind === 'signature' && signedPurposeSet.has(purpose);
  const sealAlreadyApplied = kind === 'seal' && Boolean(selectedSealAssetId) && appliedSealKeySet.has(`${purpose}:${selectedSealAssetId}`);
  const activeSealAssets = sealAssets.filter((asset) => asset.status === 'active');

  const availablePurposeOptions = useMemo(
    () => (allowedPurposes?.length
      ? signaturePurposeOptions.filter((option) => allowedPurposes.includes(option.value))
      : signaturePurposeOptions),
    [allowedPurposes],
  );

  useEffect(() => {
    if (!open || kind !== 'signature') return;
    drawCanvas(strokes, activeStroke ?? []);
  }, [activeStroke, kind, open, strokes]);

  useEffect(() => {
    if (open) return;
    setKind(enabledKinds[0] ?? 'signature');
    setPurpose(defaultPurpose);
    setComment('');
    setStrokes([]);
    setActiveStroke(null);
    setSelectedSealAssetId('');
    setSealName('');
    setSealFile(null);
    setUploadError('');
    setAcknowledgedImmutable(false);
  }, [defaultPurpose, enabledKinds, open]);

  useEffect(() => {
    if (!open) return;
    if (!enabledKinds.includes(kind)) setKind(enabledKinds[0] ?? 'signature');
    if (!availablePurposeOptions.some((option) => option.value === purpose)) {
      setPurpose(availablePurposeOptions[0]?.value ?? defaultPurpose);
    }
  }, [availablePurposeOptions, defaultPurpose, enabledKinds, kind, open, purpose]);

  function drawCanvas(savedStrokes: SignatureStroke[], currentStroke: SignaturePoint[]) {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.clearRect(0, 0, canvasWidth, canvasHeight);
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, canvasWidth, canvasHeight);
    ctx.strokeStyle = '#0f172a';
    ctx.lineWidth = 2.5;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    [...savedStrokes.map((stroke) => stroke.points), currentStroke].forEach((points) => {
      if (points.length < 2) return;
      ctx.beginPath();
      ctx.moveTo(points[0].x * canvasWidth, points[0].y * canvasHeight);
      points.slice(1).forEach((point) => ctx.lineTo(point.x * canvasWidth, point.y * canvasHeight));
      ctx.stroke();
    });
    ctx.strokeStyle = '#cbd5e1';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(32, canvasHeight - 42);
    ctx.lineTo(canvasWidth - 32, canvasHeight - 42);
    ctx.stroke();
  }

  function pointFromEvent(event: PointerEvent<HTMLCanvasElement>): SignaturePoint {
    const rect = event.currentTarget.getBoundingClientRect();
    return {
      x: clamp((event.clientX - rect.left) / rect.width),
      y: clamp((event.clientY - rect.top) / rect.height),
    };
  }

  function onPointerDown(event: PointerEvent<HTMLCanvasElement>) {
    if (submitting) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    setActiveStroke([pointFromEvent(event)]);
  }

  function onPointerMove(event: PointerEvent<HTMLCanvasElement>) {
    if (!activeStroke || submitting) return;
    const nextPoint = pointFromEvent(event);
    setActiveStroke((current) => (current ? [...current, nextPoint] : current));
  }

  function finishStroke() {
    if (!activeStroke) return;
    if (activeStroke.length >= 2) setStrokes((current) => [...current, { points: activeStroke }]);
    setActiveStroke(null);
  }

  function clear() {
    setStrokes([]);
    setActiveStroke(null);
    drawCanvas([], []);
  }

  function submit() {
    if (!acknowledgedImmutable) return;
    if (kind === 'seal') {
      if (!selectedSealAssetId || sealAlreadyApplied || !onApplySeal) return;
      onApplySeal({ purpose, comment, seal_asset_id: selectedSealAssetId });
      return;
    }
    const allStrokes = activeStroke && activeStroke.length >= 2 ? [...strokes, { points: activeStroke }] : strokes;
    if (!allStrokes.length || purposeAlreadySigned) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    drawCanvas(allStrokes, []);
    onSubmit({
      purpose,
      comment,
      signature_image_data_url: canvas.toDataURL('image/png'),
      signature_strokes: allStrokes,
    });
  }

  async function uploadSeal() {
    const normalizedName = sealName.trim();
    const message = validateSealUpload(normalizedName, sealFile);
    if (message || !sealFile || !onUploadSeal) {
      setUploadError(message || 'Chưa thể tải con dấu.');
      onValidationWarning?.(message || 'Chưa thể tải con dấu.');
      return;
    }
    setUploadError('');
    try {
      const asset = await onUploadSeal(normalizedName, sealFile);
      setSelectedSealAssetId(asset.id);
      setSealName('');
      setSealFile(null);
    } catch {
      // Parent mutation reports the API error through the shared toast.
    }
  }

  const hasSignature = strokes.length > 0 || Boolean(activeStroke && activeStroke.length >= 2);
  const invalidCurrentConfirmation = kind === 'signature'
    ? !hasSignature || purposeAlreadySigned
    : !selectedSealAssetId || sealAlreadyApplied;
  const description = [signerName ? `Người thao tác: ${signerName}` : undefined, formRevision ? `Phiên bản: ${formRevision}` : undefined].filter(Boolean).join(' · ');

  return (
    <Dialog
      open={open}
      title="Thêm xác nhận hồ sơ"
      description={description || undefined}
      onClose={onClose}
      size="lg"
      footer={(
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            {kind === 'signature' ? (
              <button
                type="button"
                className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60"
                onClick={clear}
                disabled={submitting || !hasSignature}
              >
                <Eraser className="h-4 w-4" />
                Xóa chữ ký
              </button>
            ) : null}
          </div>
          <div className="flex flex-wrap justify-end gap-2">
            <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose} disabled={submitting}>
              Hủy
            </button>
            <button
              className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={submit}
              disabled={submitting || uploadingSeal || invalidCurrentConfirmation || !acknowledgedImmutable}
            >
              {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : kind === 'signature' ? <PenLine className="h-4 w-4" /> : <Stamp className="h-4 w-4" />}
              {kind === 'signature' ? 'Ký và lưu' : 'Đóng dấu và lưu'}
            </button>
          </div>
        </div>
      )}
    >
      <div className="space-y-3">
        <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900">
          <div className="flex items-start gap-2">
            <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-emerald-700" />
            <div>
              <p className="font-semibold">Xác nhận được lưu bất biến</p>
              <p className="mt-1 text-xs leading-5 text-emerald-800">
                Bản ghi gắn với người thao tác, thời điểm, mục đích, phiên bản hiện tại và mã kiểm tra của hồ sơ đã chốt.
              </p>
            </div>
          </div>
        </div>

        {enabledKinds.length > 1 ? (
          <div className="grid grid-cols-2 gap-1 rounded-md bg-slate-100 p-1" aria-label="Loại xác nhận">
            <ConfirmationKindButton active={kind === 'signature'} label="Chữ ký cá nhân" icon={<PenLine className="h-4 w-4" />} onClick={() => changeKind('signature')} />
            <ConfirmationKindButton active={kind === 'seal'} label="Con dấu công ty" icon={<Stamp className="h-4 w-4" />} onClick={() => changeKind('seal')} />
          </div>
        ) : null}

        <div className="grid gap-3 sm:grid-cols-2">
          <label>
            <span className="text-xs font-semibold text-slate-600">Mục đích xác nhận</span>
            <select
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={purpose}
              disabled={submitting || purposeLocked || availablePurposeOptions.length === 1}
              onChange={(event) => {
                setPurpose(event.target.value);
                setAcknowledgedImmutable(false);
              }}
            >
              {availablePurposeOptions.map((option) => {
                const used = kind === 'signature' && signedPurposeSet.has(option.value);
                return <option key={option.value} value={option.value} disabled={used}>{option.label}{used ? ' (đã ký)' : ''}</option>;
              })}
            </select>
          </label>
          <label>
            <span className="text-xs font-semibold text-slate-600">Ghi chú</span>
            <input
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={comment}
              maxLength={1000}
              disabled={submitting}
              onChange={(event) => setComment(event.target.value)}
              placeholder={kind === 'signature' ? 'Ghi chú chữ ký' : 'Ghi chú đóng dấu'}
            />
          </label>
        </div>

        {purposeAlreadySigned ? <DuplicateWarning>Người dùng hiện tại đã ký mục đích này trên hồ sơ.</DuplicateWarning> : null}
        {sealAlreadyApplied ? <DuplicateWarning>Con dấu này đã được dùng cho cùng mục đích trên hồ sơ.</DuplicateWarning> : null}

        {kind === 'signature' ? (
          <>
            <canvas
              ref={canvasRef}
              width={canvasWidth}
              height={canvasHeight}
              className="block w-full touch-none rounded-md border border-line bg-white"
              onPointerDown={onPointerDown}
              onPointerMove={onPointerMove}
              onPointerUp={finishStroke}
              onPointerCancel={finishStroke}
              onPointerLeave={finishStroke}
            />
            <div className="rounded-md border border-line bg-slate-50 p-3 text-sm">
              <p className="font-semibold text-ink">KÝ BỞI {signerName ?? 'Người dùng hiện tại'}</p>
              <p className="mt-1 text-xs text-slate-500">Chữ ký cá nhân sẽ xuất hiện tại vị trí cố định trong PDF cùng mã kiểm tra và thời điểm ký.</p>
            </div>
          </>
        ) : (
          <SealSelection
            assets={activeSealAssets}
            loading={sealAssetsLoading}
            selectedId={selectedSealAssetId}
            disabled={submitting || uploadingSeal}
            canManage={canManageSeals && Boolean(onUploadSeal)}
            sealName={sealName}
            sealFile={sealFile}
            uploadError={uploadError}
            onSelect={(id) => {
              setSelectedSealAssetId(id);
              setAcknowledgedImmutable(false);
            }}
            onNameChange={(name) => {
              setSealName(name);
              setUploadError('');
            }}
            onFileChange={(file) => {
              setSealFile(file);
              setUploadError('');
              if (file && !sealName.trim()) setSealName(file.name.replace(/\.[^.]+$/, ''));
            }}
            onUpload={() => void uploadSeal()}
          />
        )}

        <label className="flex items-start gap-2 rounded-md border border-line bg-white p-3 text-sm text-slate-700">
          <input
            type="checkbox"
            className="mt-0.5 h-4 w-4 rounded border-line"
            checked={acknowledgedImmutable}
            disabled={submitting || uploadingSeal || purposeAlreadySigned || sealAlreadyApplied}
            onChange={(event) => setAcknowledgedImmutable(event.target.checked)}
          />
          <span>Tôi xác nhận {kind === 'signature' ? 'chữ ký cá nhân' : 'con dấu công ty'} này chỉ được ghi thêm và dùng đúng mục đích/phiên bản đang hiển thị.</span>
        </label>
      </div>
    </Dialog>
  );

  function changeKind(nextKind: ConfirmationKind) {
    setKind(nextKind);
    setAcknowledgedImmutable(false);
  }
}

function ConfirmationKindButton({ active, label, icon, onClick }: { active: boolean; label: string; icon: ReactNode; onClick: () => void }) {
  return (
    <button
      type="button"
      className={`inline-flex items-center justify-center gap-2 rounded px-3 py-2 text-sm font-semibold ${active ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-600 hover:text-ink'}`}
      aria-pressed={active}
      onClick={onClick}
    >
      {icon}
      {label}
    </button>
  );
}

function DuplicateWarning({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
      <p className="text-xs leading-5">{children} Mỗi xác nhận chỉ được ghi một lần theo quy tắc hiện tại.</p>
    </div>
  );
}

function SealSelection({
  assets,
  loading,
  selectedId,
  disabled,
  canManage,
  sealName,
  sealFile,
  uploadError,
  onSelect,
  onNameChange,
  onFileChange,
  onUpload,
}: {
  assets: SealAsset[];
  loading: boolean;
  selectedId: string;
  disabled: boolean;
  canManage: boolean;
  sealName: string;
  sealFile: File | null;
  uploadError: string;
  onSelect: (id: string) => void;
  onNameChange: (name: string) => void;
  onFileChange: (file: File | null) => void;
  onUpload: () => void;
}) {
  return (
    <div className="space-y-3">
      <div>
        <p className="text-xs font-semibold text-slate-600">Chọn con dấu đã lưu</p>
        {loading ? (
          <div className="mt-1 flex items-center gap-2 rounded-md border border-line p-4 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />Đang tải tài nguyên con dấu</div>
        ) : assets.length ? (
          <div className="mt-1 grid gap-2 sm:grid-cols-3">
            {assets.map((asset) => (
              <button
                key={asset.id}
                type="button"
                className={`rounded-md border p-2 text-left ${selectedId === asset.id ? 'border-blue-400 bg-blue-50 ring-2 ring-blue-100' : 'border-line bg-white hover:border-blue-200'}`}
                disabled={disabled}
                onClick={() => onSelect(asset.id)}
              >
                <SealAssetPreview path={`/seal-assets/${asset.id}/file`} alt={asset.name} className="h-24 w-full" />
                <span className="mt-2 block truncate text-xs font-semibold text-ink">{asset.name}</span>
              </button>
            ))}
          </div>
        ) : (
          <div className="mt-1 rounded-md border border-dashed border-line bg-slate-50 px-3 py-5 text-center text-sm text-slate-500">Chưa có con dấu đang hoạt động.</div>
        )}
      </div>

      {canManage ? (
        <div className="rounded-md border border-line bg-slate-50 p-3">
          <div className="flex items-center gap-2"><Upload className="h-4 w-4 text-blue-600" /><p className="text-sm font-semibold text-ink">Tải con dấu mới vào tài nguyên dùng chung</p></div>
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            <label>
              <span className="text-xs font-semibold text-slate-600">Tên con dấu *</span>
              <input className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm" value={sealName} disabled={disabled} onChange={(event) => onNameChange(event.target.value)} />
            </label>
            <label>
              <span className="text-xs font-semibold text-slate-600">Ảnh PNG/JPG/WebP *</span>
              <input className="mt-1 block w-full text-xs text-slate-600 file:mr-2 file:rounded-md file:border-0 file:bg-white file:px-3 file:py-2 file:font-semibold" type="file" accept="image/png,image/jpeg,image/webp" disabled={disabled} onChange={(event) => onFileChange(event.target.files?.[0] ?? null)} />
            </label>
          </div>
          <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
            <p className={uploadError ? 'text-xs font-medium text-rose-600' : 'text-xs text-slate-500'}>{uploadError || `${sealFile?.name ?? 'Tối đa 10 MB'} · hệ thống chuẩn hóa thành PNG.`}</p>
            <button type="button" className="inline-flex items-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-xs font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60" disabled={disabled} onClick={onUpload}>
              {disabled ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Upload className="h-3.5 w-3.5" />}
              Tải và chọn con dấu
            </button>
          </div>
        </div>
      ) : (
        <p className="text-xs text-slate-500">Con dấu do quản trị viên quản lý trong Cấu hình. Người có quyền chỉ chọn tài nguyên đang hoạt động.</p>
      )}
    </div>
  );
}

function validateSealUpload(name: string, file: File | null) {
  if (!name) return 'Vui lòng nhập tên con dấu.';
  if (!file) return 'Vui lòng chọn ảnh con dấu.';
  if (!acceptedSealTypes.has(file.type)) return 'Chỉ chấp nhận ảnh PNG, JPG hoặc WebP; không hỗ trợ SVG.';
  if (file.size > maxSealFileSize) return 'Ảnh con dấu không được vượt quá 10 MB.';
  return '';
}

function clamp(value: number) {
  if (Number.isNaN(value)) return 0;
  return Math.max(0, Math.min(1, value));
}
