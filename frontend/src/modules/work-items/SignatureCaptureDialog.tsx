import { PointerEvent, useEffect, useRef, useState } from 'react';
import { AlertTriangle, Eraser, Loader2, PenLine, ShieldCheck } from 'lucide-react';
import { Dialog } from '../../components/Dialog';

export type SignaturePoint = { x: number; y: number };
export type SignatureStroke = { points: SignaturePoint[] };

type SignatureCapturePayload = {
  purpose: string;
  comment: string;
  signature_image_data_url: string;
  signature_strokes: SignatureStroke[];
};

type SignatureCaptureDialogProps = {
  open: boolean;
  signerName?: string;
  submitting: boolean;
  signedPurposes?: string[];
  formRevision?: number;
  onClose: () => void;
  onSubmit: (payload: SignatureCapturePayload) => void;
};

const canvasWidth = 640;
const canvasHeight = 220;
const signaturePurposeOptions = [
  { value: 'internal_ack', label: 'Internal ack' },
  { value: 'prepared_by', label: 'Prepared by' },
  { value: 'reviewed_by', label: 'Reviewed by' },
  { value: 'approved_by', label: 'Approved by' },
  { value: 'received_by', label: 'Received by' },
];

export function SignatureCaptureDialog({
  open,
  signerName,
  submitting,
  signedPurposes = [],
  formRevision,
  onClose,
  onSubmit,
}: SignatureCaptureDialogProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [purpose, setPurpose] = useState('internal_ack');
  const [comment, setComment] = useState('');
  const [strokes, setStrokes] = useState<SignatureStroke[]>([]);
  const [activeStroke, setActiveStroke] = useState<SignaturePoint[] | null>(null);
  const [acknowledgedImmutable, setAcknowledgedImmutable] = useState(false);
  const signedPurposeSet = new Set(signedPurposes);
  const purposeAlreadySigned = signedPurposeSet.has(purpose);

  useEffect(() => {
    if (!open) return;
    drawCanvas(strokes, activeStroke ?? []);
  }, [open, strokes, activeStroke]);

  useEffect(() => {
    if (!open) {
      setPurpose('internal_ack');
      setComment('');
      setStrokes([]);
      setActiveStroke(null);
      setAcknowledgedImmutable(false);
    }
  }, [open]);

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
    setActiveStroke((current) => (current ? [...current, pointFromEvent(event)] : current));
  }

  function finishStroke() {
    if (!activeStroke) return;
    if (activeStroke.length >= 2) {
      setStrokes((current) => [...current, { points: activeStroke }]);
    }
    setActiveStroke(null);
  }

  function clear() {
    setStrokes([]);
    setActiveStroke(null);
    drawCanvas([], []);
  }

  function submit() {
    const allStrokes = activeStroke && activeStroke.length >= 2 ? [...strokes, { points: activeStroke }] : strokes;
    if (!allStrokes.length || !acknowledgedImmutable || purposeAlreadySigned) return;
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

  const hasSignature = strokes.length > 0 || Boolean(activeStroke && activeStroke.length >= 2);
  const description = [signerName ? `Signer: ${signerName}` : undefined, formRevision ? `Revision: ${formRevision}` : undefined].filter(Boolean).join(' · ');

  return (
    <Dialog
      open={open}
      title="Ký / Đóng dấu PDF"
      description={description || undefined}
      onClose={onClose}
      size="lg"
      footer={
        <div className="flex flex-wrap items-center justify-between gap-2">
          <button
            type="button"
            className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            onClick={clear}
            disabled={submitting || !hasSignature}
          >
            <Eraser className="h-4 w-4" />
            Clear
          </button>
          <div className="flex flex-wrap justify-end gap-2">
            <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose} disabled={submitting}>
              Hủy
            </button>
            <button
              className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
              onClick={submit}
              disabled={submitting || !hasSignature || !acknowledgedImmutable || purposeAlreadySigned}
            >
              {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <PenLine className="h-4 w-4" />}
              Ký & lưu
            </button>
          </div>
        </div>
      }
    >
      <div className="space-y-3">
        <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900">
          <div className="flex items-start gap-2">
            <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0 text-emerald-700" />
            <div>
              <p className="font-semibold">Chữ ký được lưu bất biến</p>
              <p className="mt-1 text-xs leading-5 text-emerald-800">
                Record ký sẽ gắn với actor, thời điểm ký, purpose, revision hiện tại và payload hash của form đã submit.
              </p>
            </div>
          </div>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <label>
            <span className="text-xs font-semibold text-slate-600">Purpose</span>
            <select
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={purpose}
              disabled={submitting}
              onChange={(event) => {
                setPurpose(event.target.value);
                setAcknowledgedImmutable(false);
              }}
            >
              {signaturePurposeOptions.map((option) => (
                <option key={option.value} value={option.value} disabled={signedPurposeSet.has(option.value)}>
                  {option.label}
                  {signedPurposeSet.has(option.value) ? ' (đã ký)' : ''}
                </option>
              ))}
            </select>
          </label>
          <label>
            <span className="text-xs font-semibold text-slate-600">Comment</span>
            <input
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={comment}
              maxLength={1000}
              disabled={submitting}
              onChange={(event) => setComment(event.target.value)}
              placeholder="Ghi chú chữ ký"
            />
          </label>
        </div>
        {purposeAlreadySigned ? (
          <div className="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-700" />
            <p className="text-xs leading-5">Bạn đã ký purpose này. Mỗi user chỉ được ký một lần cho cùng purpose trên form instance này.</p>
          </div>
        ) : null}
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
          <p className="font-semibold text-ink">SIGNED BY {signerName ?? 'Current user'}</p>
          <p className="mt-1 text-xs text-slate-500">Stamp sẽ được gắn vào PDF export cùng hash và thời điểm ký.</p>
        </div>
        <label className="flex items-start gap-2 rounded-md border border-line bg-white p-3 text-sm text-slate-700">
          <input
            type="checkbox"
            className="mt-0.5 h-4 w-4 rounded border-line"
            checked={acknowledgedImmutable}
            disabled={submitting || purposeAlreadySigned}
            onChange={(event) => setAcknowledgedImmutable(event.target.checked)}
          />
          <span>
            Tôi xác nhận chữ ký này là append-only và dùng cho đúng purpose/revision đang hiển thị.
          </span>
        </label>
      </div>
    </Dialog>
  );
}

function clamp(value: number) {
  if (Number.isNaN(value)) return 0;
  return Math.max(0, Math.min(1, value));
}
