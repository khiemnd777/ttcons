import { PointerEvent, useEffect, useRef, useState } from 'react';
import { Eraser, Loader2, PenLine } from 'lucide-react';
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
  onClose: () => void;
  onSubmit: (payload: SignatureCapturePayload) => void;
};

const canvasWidth = 640;
const canvasHeight = 220;

export function SignatureCaptureDialog({ open, signerName, submitting, onClose, onSubmit }: SignatureCaptureDialogProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [purpose, setPurpose] = useState('internal_ack');
  const [comment, setComment] = useState('');
  const [strokes, setStrokes] = useState<SignatureStroke[]>([]);
  const [activeStroke, setActiveStroke] = useState<SignaturePoint[] | null>(null);

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
    if (!allStrokes.length) return;
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

  const hasSignature = strokes.length > 0 || Boolean(activeStroke?.length);

  return (
    <Dialog
      open={open}
      title="Ký / Đóng dấu PDF"
      description={signerName ? `Signer: ${signerName}` : undefined}
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
              disabled={submitting || !hasSignature}
            >
              {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <PenLine className="h-4 w-4" />}
              Ký & lưu
            </button>
          </div>
        </div>
      }
    >
      <div className="space-y-3">
        <div className="grid gap-3 sm:grid-cols-2">
          <label>
            <span className="text-xs font-semibold text-slate-600">Purpose</span>
            <select
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={purpose}
              disabled={submitting}
              onChange={(event) => setPurpose(event.target.value)}
            >
              <option value="internal_ack">Internal ack</option>
              <option value="prepared_by">Prepared by</option>
              <option value="reviewed_by">Reviewed by</option>
              <option value="approved_by">Approved by</option>
              <option value="received_by">Received by</option>
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
      </div>
    </Dialog>
  );
}

function clamp(value: number) {
  if (Number.isNaN(value)) return 0;
  return Math.max(0, Math.min(1, value));
}
