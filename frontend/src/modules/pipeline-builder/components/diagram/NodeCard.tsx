import type { PointerEvent } from 'react';
import clsx from 'clsx';
import { MousePointer2, PanelRightOpen } from 'lucide-react';
import { EnumChip } from '../../../../components/EnumChip';
import type { PipelineNode } from '../../../../lib/types/domain';
import { NODE_HEIGHT, NODE_WIDTH } from './diagramConstants';
import type { NodePosition } from './diagramTypes';

type NodeCardProps = {
  node: PipelineNode;
  index: number;
  position: NodePosition;
  selected: boolean;
  onSelectNode: (nodeId: string) => void;
  onToggleDetails: (nodeId: string) => void;
  onPointerDown: (event: PointerEvent<HTMLDivElement>, node: PipelineNode, index: number) => void;
  onPointerMove: (event: PointerEvent<HTMLDivElement>) => void;
  onPointerUp: (event: PointerEvent<HTMLDivElement>) => void;
  onPointerCancel: (event: PointerEvent<HTMLDivElement>) => void;
};

export function NodeCard({
  node,
  index,
  position,
  selected,
  onSelectNode,
  onToggleDetails,
  onPointerDown,
  onPointerMove,
  onPointerUp,
  onPointerCancel,
}: NodeCardProps) {
  const roles = Array.isArray(node.responsible_role_ids) ? node.responsible_role_ids.join(', ') : 'PM';

  return (
    <div
      role="button"
      tabIndex={0}
      className={clsx(
        'pointer-events-auto absolute z-10 select-none rounded-lg border bg-white p-3 text-left shadow-sm transition hover:-translate-y-0.5 hover:shadow-panel focus:outline-none',
        'cursor-grab active:cursor-grabbing',
        selected && 'border-blue-400 bg-blue-50 shadow-panel ring-4 ring-blue-100',
        !selected && 'border-line',
      )}
      style={{ left: position.x, top: position.y, width: NODE_WIDTH, minHeight: NODE_HEIGHT }}
      onFocus={() => onSelectNode(node.id)}
      onPointerDown={(event) => onPointerDown(event, node, index)}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerCancel}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          onSelectNode(node.id);
        }
      }}
    >
      <div className="mb-3 flex items-start justify-between gap-3">
        <div className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-slate-100 text-xs font-black text-slate-700">{index + 1}</div>
        <div className="flex items-center gap-1">
          <MousePointer2 className="h-4 w-4 shrink-0 text-slate-300" />
          <button
            title="Chi tiết node"
            aria-label={`Chi tiết node ${node.name}`}
            className="rounded-md p-1 text-slate-400 hover:bg-white hover:text-blue-600"
            onPointerDown={(event) => event.stopPropagation()}
            onClick={(event) => {
              event.stopPropagation();
              onToggleDetails(node.id);
            }}
          >
            <PanelRightOpen className="h-4 w-4" />
          </button>
        </div>
      </div>
      <h3 className="line-clamp-2 text-sm font-semibold text-ink">{node.name}</h3>
      <p className="mt-2 line-clamp-3 text-xs leading-5 text-slate-600">{node.description}</p>
      <div className="mt-3 flex flex-wrap gap-1">
        <span className="max-w-full truncate rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{roles}</span>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{node.steps?.length ?? 0} step</span>
        <EnumChip group="node_type" value={node.type} />
      </div>
    </div>
  );
}
