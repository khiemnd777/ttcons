import type { PipelineNode } from '../../../../lib/types/domain';
import { NODE_HEIGHT, NODE_WIDTH, nodePosition } from './diagramConstants';
import type { ConnectorEndpoint, DiagramNodeRect, NodePosition, NodeSize, PortRef, PortSide } from './diagramTypes';

export const PORT_SIDES: PortSide[] = ['top', 'right', 'bottom', 'left'];

type ClientPoint = {
  clientX: number;
  clientY: number;
};

type CanvasBounds = {
  left: number;
  top: number;
};

export function viewportPointToWorld(point: NodePosition, offset: NodePosition): NodePosition {
  return {
    x: point.x - offset.x,
    y: point.y - offset.y,
  };
}

export function worldPointToViewport(point: NodePosition, offset: NodePosition): NodePosition {
  return {
    x: point.x + offset.x,
    y: point.y + offset.y,
  };
}

export function clientPointToWorld(point: ClientPoint, bounds: CanvasBounds, offset: NodePosition): NodePosition {
  return viewportPointToWorld(
    {
      x: point.clientX - bounds.left,
      y: point.clientY - bounds.top,
    },
    offset,
  );
}

export function worldPointToClient(point: NodePosition, bounds: CanvasBounds, offset: NodePosition): NodePosition {
  const viewportPoint = worldPointToViewport(point, offset);
  return {
    x: viewportPoint.x + bounds.left,
    y: viewportPoint.y + bounds.top,
  };
}

export function nodeRect(nodeId: string, position: NodePosition, width = NODE_WIDTH, height = NODE_HEIGHT): DiagramNodeRect {
  return {
    nodeId,
    x: position.x,
    y: position.y,
    width,
    height,
  };
}

export function nodeRectFromNode(
  node: PipelineNode,
  index: number,
  positions: Record<string, NodePosition>,
  sizes: Record<string, NodeSize> = {},
): DiagramNodeRect {
  const size = sizes[node.id];
  return nodeRect(node.id, nodePosition(node, index, positions), size?.width ?? NODE_WIDTH, size?.height ?? NODE_HEIGHT);
}

export function nodeRectsFromNodes(
  nodes: PipelineNode[],
  positions: Record<string, NodePosition>,
  sizes: Record<string, NodeSize> = {},
): DiagramNodeRect[] {
  return nodes.map((node, index) => nodeRectFromNode(node, index, positions, sizes));
}

export function rectCenter(rect: DiagramNodeRect): NodePosition {
  return {
    x: rect.x + rect.width / 2,
    y: rect.y + rect.height / 2,
  };
}

export function portPoint(rect: DiagramNodeRect, side: PortSide): NodePosition {
  switch (side) {
    case 'top':
      return { x: rect.x + rect.width / 2, y: rect.y };
    case 'right':
      return { x: rect.x + rect.width, y: rect.y + rect.height / 2 };
    case 'bottom':
      return { x: rect.x + rect.width / 2, y: rect.y + rect.height };
    case 'left':
      return { x: rect.x, y: rect.y + rect.height / 2 };
  }
}

export function portRef(rect: DiagramNodeRect, side: PortSide): PortRef {
  return {
    nodeId: rect.nodeId,
    side,
  };
}

export function connectorEndpoint(rect: DiagramNodeRect, side: PortSide): ConnectorEndpoint {
  return {
    port: portRef(rect, side),
    point: portPoint(rect, side),
  };
}

export function nearestPortSide(rect: DiagramNodeRect, point: NodePosition): PortSide {
  return PORT_SIDES.reduce((nearest, side) => {
    const nearestDistance = distanceSquared(point, portPoint(rect, nearest));
    const sideDistance = distanceSquared(point, portPoint(rect, side));
    return sideDistance < nearestDistance ? side : nearest;
  }, PORT_SIDES[0]);
}

export function nearestPort(rect: DiagramNodeRect, point: NodePosition): ConnectorEndpoint {
  return connectorEndpoint(rect, nearestPortSide(rect, point));
}

export function samePoint(left: NodePosition | undefined, right: NodePosition | undefined) {
  return left?.x === right?.x && left?.y === right?.y;
}

export function distanceSquared(left: NodePosition, right: NodePosition) {
  const dx = left.x - right.x;
  const dy = left.y - right.y;
  return dx * dx + dy * dy;
}
