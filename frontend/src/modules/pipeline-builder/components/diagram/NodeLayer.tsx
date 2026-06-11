import type { PointerEvent } from 'react';
import type { PipelineNode } from '../../../../lib/types/domain';
import { nodePosition } from './diagramConstants';
import type { NodePosition } from './diagramTypes';
import { NodeCard } from './NodeCard';

type NodeLayerProps = {
  nodes: PipelineNode[];
  positions: Record<string, NodePosition>;
  selectedNodeId: string;
  onSelectNode: (nodeId: string) => void;
  onToggleNodeDetails: (nodeId: string) => void;
  onNodePointerDown: (event: PointerEvent<HTMLDivElement>, node: PipelineNode, index: number) => void;
  onNodePointerMove: (event: PointerEvent<HTMLDivElement>) => void;
  onNodePointerUp: (event: PointerEvent<HTMLDivElement>) => void;
  onNodePointerCancel: (event: PointerEvent<HTMLDivElement>) => void;
  setNodeElementRef: (nodeId: string, element: HTMLDivElement | null) => void;
};

export function NodeLayer({
  nodes,
  positions,
  selectedNodeId,
  onSelectNode,
  onToggleNodeDetails,
  onNodePointerDown,
  onNodePointerMove,
  onNodePointerUp,
  onNodePointerCancel,
  setNodeElementRef,
}: NodeLayerProps) {
  return (
    <>
      {nodes.map((node, index) => (
        <NodeCard
          key={node.id}
          node={node}
          index={index}
          position={nodePosition(node, index, positions)}
          selected={selectedNodeId === node.id}
          onSelectNode={onSelectNode}
          onToggleDetails={onToggleNodeDetails}
          onPointerDown={onNodePointerDown}
          onPointerMove={onNodePointerMove}
          onPointerUp={onNodePointerUp}
          onPointerCancel={onNodePointerCancel}
          setNodeElementRef={setNodeElementRef}
        />
      ))}
    </>
  );
}
