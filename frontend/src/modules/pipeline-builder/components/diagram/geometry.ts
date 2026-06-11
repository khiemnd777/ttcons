import type { PipelineNode } from '../../../../lib/types/domain';
import { NODE_HEIGHT, NODE_WIDTH, nodePosition } from './diagramConstants';
import type { ConnectorEndpoint, DiagramNodeRect, NodePosition, NodeSize, PortRef, PortSide } from './diagramTypes';

export const PORT_SIDES: PortSide[] = ['top', 'right', 'bottom', 'left'];
export const CONNECTOR_ENDPOINT_CLEARANCE = 16;

type ClientPoint = {
  clientX: number;
  clientY: number;
};

type CanvasBounds = {
  left: number;
  top: number;
};

export function viewportPointToWorld(point: NodePosition, offset: NodePosition, scale = 1): NodePosition {
  return {
    x: (point.x - offset.x) / scale,
    y: (point.y - offset.y) / scale,
  };
}

export function worldPointToViewport(point: NodePosition, offset: NodePosition, scale = 1): NodePosition {
  return {
    x: point.x * scale + offset.x,
    y: point.y * scale + offset.y,
  };
}

export function clientPointToWorld(point: ClientPoint, bounds: CanvasBounds, offset: NodePosition, scale = 1): NodePosition {
  return viewportPointToWorld(
    {
      x: point.clientX - bounds.left,
      y: point.clientY - bounds.top,
    },
    offset,
    scale,
  );
}

export function worldPointToClient(point: NodePosition, bounds: CanvasBounds, offset: NodePosition, scale = 1): NodePosition {
  const viewportPoint = worldPointToViewport(point, offset, scale);
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

export function visibleConnectorEndpoint(rect: DiagramNodeRect, side: PortSide, clearance = CONNECTOR_ENDPOINT_CLEARANCE): ConnectorEndpoint {
  return {
    port: portRef(rect, side),
    point: offsetPointBySide(portPoint(rect, side), side, clearance),
  };
}

export function preferredPortPair(sourceRect: DiagramNodeRect, targetRect: DiagramNodeRect): { sourceSide: PortSide; targetSide: PortSide } {
  const sourceCenter = rectCenter(sourceRect);
  const targetCenter = rectCenter(targetRect);
  const dx = targetCenter.x - sourceCenter.x;
  const dy = targetCenter.y - sourceCenter.y;

  if (Math.abs(dx) >= Math.abs(dy)) {
    const sourceSide = dx >= 0 ? 'right' : 'left';
    return {
      sourceSide,
      targetSide: oppositePortSide(sourceSide),
    };
  }

  const sourceSide = dy >= 0 ? 'bottom' : 'top';
  return {
    sourceSide,
    targetSide: oppositePortSide(sourceSide),
  };
}

export function oppositePortSide(side: PortSide): PortSide {
  switch (side) {
    case 'top':
      return 'bottom';
    case 'right':
      return 'left';
    case 'bottom':
      return 'top';
    case 'left':
      return 'right';
  }
}

export function offsetPointBySide(point: NodePosition, side: PortSide, distance: number): NodePosition {
  switch (side) {
    case 'top':
      return { x: point.x, y: point.y - distance };
    case 'right':
      return { x: point.x + distance, y: point.y };
    case 'bottom':
      return { x: point.x, y: point.y + distance };
    case 'left':
      return { x: point.x - distance, y: point.y };
  }
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

export function expandNodeRect(rect: DiagramNodeRect, padding: number): DiagramNodeRect {
  return {
    nodeId: rect.nodeId,
    x: rect.x - padding,
    y: rect.y - padding,
    width: rect.width + padding * 2,
    height: rect.height + padding * 2,
  };
}

export function pointInNodeRect(point: NodePosition, rect: DiagramNodeRect) {
  return point.x >= rect.x && point.x <= rect.x + rect.width && point.y >= rect.y && point.y <= rect.y + rect.height;
}

export function orthogonalSegmentIntersectsRect(start: NodePosition, end: NodePosition, rect: DiagramNodeRect) {
  if (start.x === end.x) {
    const y1 = Math.min(start.y, end.y);
    const y2 = Math.max(start.y, end.y);
    return start.x >= rect.x && start.x <= rect.x + rect.width && y2 >= rect.y && y1 <= rect.y + rect.height;
  }

  if (start.y === end.y) {
    const x1 = Math.min(start.x, end.x);
    const x2 = Math.max(start.x, end.x);
    return start.y >= rect.y && start.y <= rect.y + rect.height && x2 >= rect.x && x1 <= rect.x + rect.width;
  }

  return false;
}

export function routeIntersectsRect(route: NodePosition[], rect: DiagramNodeRect) {
  for (let index = 1; index < route.length; index += 1) {
    if (orthogonalSegmentIntersectsRect(route[index - 1], route[index], rect)) return true;
  }
  return false;
}

export function manhattanDistance(left: NodePosition, right: NodePosition) {
  return Math.abs(left.x - right.x) + Math.abs(left.y - right.y);
}

export function samePoint(left: NodePosition | undefined, right: NodePosition | undefined) {
  return left?.x === right?.x && left?.y === right?.y;
}

export function distanceSquared(left: NodePosition, right: NodePosition) {
  const dx = left.x - right.x;
  const dy = left.y - right.y;
  return dx * dx + dy * dy;
}
