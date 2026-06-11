import { useEffect, useRef, useState, type PointerEvent } from 'react';
import clsx from 'clsx';
import { EmptyState } from '../../../../components/EmptyState';
import type { PipelineNode } from '../../../../lib/types/domain';
import { GRID_SIZE, gridOffset, initialNodePosition, nodePosition } from './diagramConstants';
import type { DragState, NodePosition, PanState } from './diagramTypes';
import { clientPointToWorld, samePoint } from './geometry';
import { NodeLayer } from './NodeLayer';

type DiagramCanvasProps = {
  nodes: PipelineNode[];
  selectedNodeId: string;
  onSelectNode: (nodeId: string) => void;
  onToggleNodeDetails: (nodeId: string) => void;
  onSaveNodePosition: (nodeId: string, position: NodePosition) => void;
};

export function DiagramCanvas({
  nodes,
  selectedNodeId,
  onSelectNode,
  onToggleNodeDetails,
  onSaveNodePosition,
}: DiagramCanvasProps) {
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const dragRef = useRef<DragState | null>(null);
  const panRef = useRef<PanState | null>(null);
  const persistedPositionsRef = useRef<Record<string, NodePosition>>({});
  const [nodePositions, setNodePositions] = useState<Record<string, NodePosition>>({});
  const [canvasOffset, setCanvasOffset] = useState<NodePosition>({ x: 0, y: 0 });
  const [isPanning, setIsPanning] = useState(false);

  useEffect(() => {
    const previousPersisted = persistedPositionsRef.current;
    const nextPersisted: Record<string, NodePosition> = {};

    nodes.forEach((node, index) => {
      nextPersisted[node.id] = initialNodePosition(node, index);
    });

    setNodePositions((current) => {
      const next: Record<string, NodePosition> = {};
      let changed = Object.keys(current).length !== nodes.length;

      nodes.forEach((node, index) => {
        const persisted = nextPersisted[node.id];
        const persistedChanged = !samePoint(previousPersisted[node.id], persisted);
        const currentPosition = current[node.id];
        const nextPosition = !currentPosition || persistedChanged ? persisted : currentPosition;
        next[node.id] = nextPosition;
        if (!samePoint(currentPosition, nextPosition)) changed = true;
      });

      return changed ? next : current;
    });
    persistedPositionsRef.current = nextPersisted;
  }, [nodes]);

  const canvasPointFromEvent = (event: PointerEvent<Element>) => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    return clientPointToWorld(event, canvas.getBoundingClientRect(), canvasOffset);
  };

  const startCanvasPan = (event: PointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || event.target !== event.currentTarget) return;
    event.preventDefault();
    event.currentTarget.setPointerCapture(event.pointerId);
    onSelectNode('');
    panRef.current = {
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startClientY: event.clientY,
      startOffset: canvasOffset,
    };
    setIsPanning(true);
  };

  const moveCanvasPan = (event: PointerEvent<HTMLDivElement>) => {
    const pan = panRef.current;
    if (!pan || pan.pointerId !== event.pointerId) return;
    setCanvasOffset({
      x: pan.startOffset.x + event.clientX - pan.startClientX,
      y: pan.startOffset.y + event.clientY - pan.startClientY,
    });
  };

  const finishCanvasPan = (event: PointerEvent<HTMLDivElement>) => {
    const pan = panRef.current;
    if (!pan || pan.pointerId !== event.pointerId) return;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    panRef.current = null;
    setIsPanning(false);
  };

  const handleNodePointerDown = (event: PointerEvent<HTMLDivElement>, node: PipelineNode, index: number) => {
    if (event.button !== 0) return;
    const point = canvasPointFromEvent(event);
    if (!point) return;
    event.preventDefault();
    event.stopPropagation();
    const position = nodePosition(node, index, nodePositions);
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = {
      nodeId: node.id,
      pointerId: event.pointerId,
      offsetX: point.x - position.x,
      offsetY: point.y - position.y,
      startClientX: event.clientX,
      startClientY: event.clientY,
      lastX: position.x,
      lastY: position.y,
      moved: false,
    };
  };

  const handleNodePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    const point = canvasPointFromEvent(event);
    if (!drag || drag.pointerId !== event.pointerId || !point) return;
    const x = point.x - drag.offsetX;
    const y = point.y - drag.offsetY;
    const moved = drag.moved || Math.hypot(event.clientX - drag.startClientX, event.clientY - drag.startClientY) > 4;
    dragRef.current = { ...drag, lastX: x, lastY: y, moved };
    if (moved) {
      setNodePositions((current) => ({ ...current, [drag.nodeId]: { x, y } }));
    }
  };

  const finishNodePointer = (event: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    event.preventDefault();
    event.stopPropagation();
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    dragRef.current = null;
    if (drag.moved) {
      onSaveNodePosition(drag.nodeId, { x: drag.lastX, y: drag.lastY });
      return;
    }
    onSelectNode(drag.nodeId);
  };

  return (
    <div
      ref={canvasRef}
      className={clsx('relative min-h-0 flex-1 touch-none overflow-hidden bg-slate-50', isPanning ? 'cursor-grabbing' : 'cursor-grab')}
      style={{
        backgroundImage: 'radial-gradient(circle at 1px 1px, #cbd5e1 1px, transparent 0)',
        backgroundPosition: `${gridOffset(canvasOffset.x)} ${gridOffset(canvasOffset.y)}`,
        backgroundSize: `${GRID_SIZE}px ${GRID_SIZE}px`,
      }}
      onPointerDown={startCanvasPan}
      onPointerMove={moveCanvasPan}
      onPointerUp={finishCanvasPan}
      onPointerCancel={finishCanvasPan}
    >
      <div
        className="pointer-events-none absolute left-0 top-0 overflow-visible"
        style={{
          transform: `translate3d(${canvasOffset.x}px, ${canvasOffset.y}px, 0)`,
          transformOrigin: '0 0',
          willChange: isPanning ? 'transform' : undefined,
        }}
      >
        <NodeLayer
          nodes={nodes}
          positions={nodePositions}
          selectedNodeId={selectedNodeId}
          onSelectNode={onSelectNode}
          onToggleNodeDetails={onToggleNodeDetails}
          onNodePointerDown={handleNodePointerDown}
          onNodePointerMove={handleNodePointerMove}
          onNodePointerUp={finishNodePointer}
          onNodePointerCancel={finishNodePointer}
        />
      </div>
      {!nodes.length ? (
        <div className="pointer-events-none absolute inset-0 grid place-items-center p-6">
          <div className="w-[380px] max-w-full">
            <EmptyState title="Template chưa có node. Chọn node trong Thư viện node để bắt đầu." />
          </div>
        </div>
      ) : null}
    </div>
  );
}
