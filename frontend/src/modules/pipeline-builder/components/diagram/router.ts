import type { NodePosition } from './diagramTypes';

export function connectorPath(points: NodePosition[]) {
  if (points.length < 2) return '';
  return points.map((point, index) => `${index === 0 ? 'M' : 'L'} ${formatPoint(point)}`).join(' ');
}

function formatPoint(point: NodePosition) {
  return `${formatNumber(point.x)} ${formatNumber(point.y)}`;
}

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}
