import type { PipelineNode } from '../../../../lib/types/domain';
import type { NodePosition } from './diagramTypes';

export const NODE_WIDTH = 236;
export const NODE_HEIGHT = 136;
export const GRID_SIZE = 24;

export function initialNodePosition(node: PipelineNode, index: number): NodePosition {
  const x = Number(node.position_x);
  const y = Number(node.position_y);
  if (Number.isFinite(x) && Number.isFinite(y)) return { x, y };
  return autoLayoutPosition(index);
}

export function nodePosition(node: PipelineNode, index: number, positions: Record<string, NodePosition>): NodePosition {
  return positions[node.id] ?? initialNodePosition(node, index);
}

export function gridOffset(value: number) {
  return `${((value % GRID_SIZE) + GRID_SIZE) % GRID_SIZE}px`;
}

export function autoLayoutPosition(index: number): NodePosition {
  const column = index % 4;
  const row = Math.floor(index / 4);
  return { x: 72 + column * 292, y: 80 + row * 190 };
}

export function nextNodePosition(index: number): NodePosition {
  const column = index % 4;
  const row = Math.floor(index / 4);
  return { x: 96 + column * 280, y: 112 + row * 180 };
}
