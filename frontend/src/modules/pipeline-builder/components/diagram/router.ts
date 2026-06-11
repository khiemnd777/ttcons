import type { NodePosition } from './diagramTypes';

const CONNECTOR_CORNER_RADIUS = 12;

export function connectorPath(points: NodePosition[]) {
  if (points.length < 2) return '';

  const commands = [`M ${formatPoint(points[0])}`];

  for (let index = 1; index < points.length - 1; index += 1) {
    const previous = points[index - 1];
    const current = points[index];
    const next = points[index + 1];
    const radius = cornerRadius(previous, current, next);

    if (!radius) {
      commands.push(`L ${formatPoint(current)}`);
      continue;
    }

    const curveStart = pointToward(current, previous, radius);
    const curveEnd = pointToward(current, next, radius);
    commands.push(`L ${formatPoint(curveStart)}`, `Q ${formatPoint(current)} ${formatPoint(curveEnd)}`);
  }

  commands.push(`L ${formatPoint(points[points.length - 1])}`);
  return commands.join(' ');
}

function cornerRadius(previous: NodePosition, current: NodePosition, next: NodePosition) {
  if (isCollinear(previous, current, next)) return 0;

  const previousDistance = distance(previous, current);
  const nextDistance = distance(current, next);
  return Math.min(CONNECTOR_CORNER_RADIUS, previousDistance / 2, nextDistance / 2);
}

function pointToward(from: NodePosition, to: NodePosition, distanceFromStart: number): NodePosition {
  const segmentDistance = distance(from, to);
  if (!segmentDistance) return from;

  const ratio = distanceFromStart / segmentDistance;
  return {
    x: from.x + (to.x - from.x) * ratio,
    y: from.y + (to.y - from.y) * ratio,
  };
}

function distance(left: NodePosition, right: NodePosition) {
  return Math.hypot(right.x - left.x, right.y - left.y);
}

function isCollinear(previous: NodePosition, current: NodePosition, next: NodePosition) {
  return (previous.x === current.x && current.x === next.x) || (previous.y === current.y && current.y === next.y);
}

function formatPoint(point: NodePosition) {
  return `${formatNumber(point.x)} ${formatNumber(point.y)}`;
}

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}
