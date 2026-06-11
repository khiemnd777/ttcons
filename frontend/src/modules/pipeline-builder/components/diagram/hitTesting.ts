import type { DiagramNodeRect, NodePosition, PortRef } from './diagramTypes';
import { PORT_SIDES, distanceSquared, portPoint } from './geometry';

export type NodeHit = {
  rect: DiagramNodeRect;
  distanceSquared: number;
};

export type PortHit = {
  rect: DiagramNodeRect;
  port: PortRef;
  point: NodePosition;
  distanceSquared: number;
};

export function pointInRect(point: NodePosition, rect: DiagramNodeRect, padding = 0) {
  return (
    point.x >= rect.x - padding &&
    point.x <= rect.x + rect.width + padding &&
    point.y >= rect.y - padding &&
    point.y <= rect.y + rect.height + padding
  );
}

export function expandRect(rect: DiagramNodeRect, padding: number): DiagramNodeRect {
  return {
    nodeId: rect.nodeId,
    x: rect.x - padding,
    y: rect.y - padding,
    width: rect.width + padding * 2,
    height: rect.height + padding * 2,
  };
}

export function hitTestNode(point: NodePosition, rects: DiagramNodeRect[], padding = 0): DiagramNodeRect | null {
  for (let index = rects.length - 1; index >= 0; index -= 1) {
    const rect = rects[index];
    if (pointInRect(point, rect, padding)) return rect;
  }
  return null;
}

export function nearestNode(point: NodePosition, rects: DiagramNodeRect[], maxDistance = Infinity): NodeHit | null {
  let nearest: NodeHit | null = null;
  const maxDistanceSquared = maxDistance * maxDistance;

  rects.forEach((rect) => {
    const distance = distanceToRectSquared(point, rect);
    if (distance > maxDistanceSquared) return;
    if (!nearest || distance < nearest.distanceSquared) {
      nearest = { rect, distanceSquared: distance };
    }
  });

  return nearest;
}

export function pointNearPort(point: NodePosition, rect: DiagramNodeRect, side: PortRef['side'], radius: number) {
  return distanceSquared(point, portPoint(rect, side)) <= radius * radius;
}

export function hitTestPort(point: NodePosition, rects: DiagramNodeRect[], radius = 12): PortHit | null {
  return nearestPortHit(point, rects, radius);
}

export function nearestPortHit(point: NodePosition, rects: DiagramNodeRect[], maxDistance = Infinity): PortHit | null {
  let nearest: PortHit | null = null;
  const maxDistanceSquared = maxDistance * maxDistance;

  rects.forEach((rect) => {
    PORT_SIDES.forEach((side) => {
      const candidatePoint = portPoint(rect, side);
      const distance = distanceSquared(point, candidatePoint);
      if (distance > maxDistanceSquared) return;
      if (!nearest || distance < nearest.distanceSquared) {
        nearest = {
          rect,
          port: { nodeId: rect.nodeId, side },
          point: candidatePoint,
          distanceSquared: distance,
        };
      }
    });
  });

  return nearest;
}

function distanceToRectSquared(point: NodePosition, rect: DiagramNodeRect) {
  const closestX = clamp(point.x, rect.x, rect.x + rect.width);
  const closestY = clamp(point.y, rect.y, rect.y + rect.height);
  return distanceSquared(point, { x: closestX, y: closestY });
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}
