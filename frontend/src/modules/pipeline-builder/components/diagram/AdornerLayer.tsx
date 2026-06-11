import type { PointerEvent } from 'react';
import clsx from 'clsx';
import { PORT_SIDES, portPoint } from './geometry';
import type { DiagramNodeRect, PortRef, PortSide } from './diagramTypes';

const OUTLINE_PADDING = 6;
const PORT_DOT_SIZE = 7;
const PORT_HIT_SIZE = 34;
const PORT_AFFORDANCE_OFFSET = 16;

type AdornerMode = 'hover' | 'selected' | 'intent' | 'target' | 'invalid';

type AdornerLayerProps = {
  rect: DiagramNodeRect | null;
  mode: AdornerMode;
  activePort?: PortRef | null;
  interactive?: boolean;
  connectorDragDisabled?: boolean;
  onConnectorDragStart: (port: PortRef, event: PointerEvent<HTMLButtonElement>) => void;
  onConnectorDragMove: (event: PointerEvent<HTMLButtonElement>) => void;
  onConnectorDragEnd: (event: PointerEvent<HTMLButtonElement>) => void;
  onConnectorDragCancel: (event: PointerEvent<HTMLButtonElement>) => void;
};

export function AdornerLayer({
  rect,
  mode,
  activePort,
  interactive = true,
  connectorDragDisabled = false,
  onConnectorDragStart,
  onConnectorDragMove,
  onConnectorDragEnd,
  onConnectorDragCancel,
}: AdornerLayerProps) {
  if (!rect) return null;

  return (
    <div className="pointer-events-none absolute left-0 top-0 z-20 overflow-visible">
      <div
        className={clsx(
          'absolute rounded-lg border bg-blue-500/5',
          mode === 'hover' && 'border-blue-300 shadow-[0_0_0_2px_rgba(59,130,246,0.08)]',
          mode === 'selected' && 'border-blue-400 shadow-[0_0_0_3px_rgba(59,130,246,0.12)]',
          mode === 'intent' && 'border-blue-500 shadow-[0_0_0_4px_rgba(59,130,246,0.16)]',
          mode === 'target' && 'border-emerald-500 bg-emerald-500/5 shadow-[0_0_0_4px_rgba(16,185,129,0.14)]',
          mode === 'invalid' && 'border-rose-400 bg-rose-500/5 shadow-[0_0_0_4px_rgba(244,63,94,0.12)]',
        )}
        style={{
          left: rect.x - OUTLINE_PADDING,
          top: rect.y - OUTLINE_PADDING,
          width: rect.width + OUTLINE_PADDING * 2,
          height: rect.height + OUTLINE_PADDING * 2,
        }}
      />

      {PORT_SIDES.map((side) => {
        const point = portPoint(rect, side);
        const affordancePoint = offsetPortPoint(point, side, PORT_AFFORDANCE_OFFSET);
        const active = activePort?.nodeId === rect.nodeId && activePort.side === side;
        const port: PortRef = { nodeId: rect.nodeId, side };
        const disabled = connectorDragDisabled || !interactive;

        return (
          <div key={side}>
            <div
              className={clsx(
                'absolute rounded-full border-2 bg-white transition',
                active && mode === 'target' && 'border-emerald-500 ring-4 ring-emerald-100',
                active && mode === 'invalid' && 'border-rose-400 ring-4 ring-rose-100',
                active && mode !== 'target' && mode !== 'invalid' && 'border-blue-600 ring-4 ring-blue-100',
                !active && mode === 'hover' && 'border-blue-300 opacity-70',
                !active && mode === 'selected' && 'border-blue-400 opacity-90',
                !active && mode === 'intent' && 'border-blue-500',
                !active && mode === 'target' && 'border-emerald-400',
                !active && mode === 'invalid' && 'border-rose-300',
              )}
              style={{
                left: point.x - PORT_DOT_SIZE / 2,
                top: point.y - PORT_DOT_SIZE / 2,
                width: PORT_DOT_SIZE,
                height: PORT_DOT_SIZE,
              }}
            />
            <button
              type="button"
              title={`Bắt đầu connector từ ${portSideLabel(side)}`}
              aria-label={`Bắt đầu connector từ ${portSideLabel(side)}`}
              disabled={disabled}
              className={clsx(
                'group pointer-events-auto absolute grid cursor-crosshair place-items-center rounded-full border bg-transparent text-blue-500/60 transition hover:scale-105 hover:border-blue-400 hover:bg-white/90 hover:text-blue-600 hover:shadow-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
                active && mode === 'target' && 'border-emerald-500 bg-white text-emerald-600 ring-4 ring-emerald-100',
                active && mode === 'invalid' && 'border-rose-400 bg-white text-rose-500 ring-4 ring-rose-100',
                active && mode !== 'target' && mode !== 'invalid' && 'border-blue-600 bg-white text-blue-700 ring-4 ring-blue-100',
                !active && 'border-transparent',
                disabled && 'pointer-events-none cursor-not-allowed opacity-45',
              )}
              style={{
                left: affordancePoint.x - PORT_HIT_SIZE / 2,
                top: affordancePoint.y - PORT_HIT_SIZE / 2,
                width: PORT_HIT_SIZE,
                height: PORT_HIT_SIZE,
              }}
              onPointerDown={(event) => {
                if (disabled) return;
                event.preventDefault();
                event.stopPropagation();
                event.currentTarget.setPointerCapture(event.pointerId);
                onConnectorDragStart(port, event);
              }}
              onPointerMove={(event) => {
                if (disabled) return;
                event.preventDefault();
                event.stopPropagation();
                onConnectorDragMove(event);
              }}
              onPointerUp={(event) => {
                if (disabled) return;
                event.preventDefault();
                event.stopPropagation();
                if (event.currentTarget.hasPointerCapture(event.pointerId)) {
                  event.currentTarget.releasePointerCapture(event.pointerId);
                }
                onConnectorDragEnd(event);
              }}
              onPointerCancel={(event) => {
                if (disabled) return;
                event.preventDefault();
                event.stopPropagation();
                if (event.currentTarget.hasPointerCapture(event.pointerId)) {
                  event.currentTarget.releasePointerCapture(event.pointerId);
                }
                onConnectorDragCancel(event);
              }}
            >
              <span
                className={clsx(
                  'h-2 w-2 rounded-full bg-current transition-opacity group-hover:opacity-100',
                  active && 'opacity-100',
                  !active && mode === 'hover' && 'opacity-20',
                  !active && mode === 'selected' && 'opacity-45',
                  !active && mode === 'intent' && 'opacity-70',
                )}
              />
            </button>
          </div>
        );
      })}
    </div>
  );
}

function offsetPortPoint(point: { x: number; y: number }, side: PortSide, offset: number) {
  switch (side) {
    case 'top':
      return { x: point.x, y: point.y - offset };
    case 'right':
      return { x: point.x + offset, y: point.y };
    case 'bottom':
      return { x: point.x, y: point.y + offset };
    case 'left':
      return { x: point.x - offset, y: point.y };
  }
}

function portSideLabel(side: PortSide) {
  switch (side) {
    case 'top':
      return 'cạnh trên';
    case 'right':
      return 'cạnh phải';
    case 'bottom':
      return 'cạnh dưới';
    case 'left':
      return 'cạnh trái';
  }
}
