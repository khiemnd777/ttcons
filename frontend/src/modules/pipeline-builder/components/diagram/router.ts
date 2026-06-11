import type { ConnectorEndpoint, NodePosition, PortSide } from './diagramTypes';

const PORT_EXIT_DISTANCE = 36;

export function routeConnector(source: ConnectorEndpoint, target: ConnectorEndpoint): NodePosition[] {
  if (!source.point || !target.point) return [];

  const sourceExit = offsetBySide(source.point, source.port.side, PORT_EXIT_DISTANCE);
  const targetEntry = offsetBySide(target.point, target.port.side, PORT_EXIT_DISTANCE);

  if (isHorizontal(source.port.side) && isHorizontal(target.port.side)) {
    const midX = (sourceExit.x + targetEntry.x) / 2;
    return compactRoute([
      source.point,
      sourceExit,
      { x: midX, y: sourceExit.y },
      { x: midX, y: targetEntry.y },
      targetEntry,
      target.point,
    ]);
  }

  if (!isHorizontal(source.port.side) && !isHorizontal(target.port.side)) {
    const midY = (sourceExit.y + targetEntry.y) / 2;
    return compactRoute([
      source.point,
      sourceExit,
      { x: sourceExit.x, y: midY },
      { x: targetEntry.x, y: midY },
      targetEntry,
      target.point,
    ]);
  }

  if (isHorizontal(source.port.side)) {
    return compactRoute([source.point, sourceExit, { x: targetEntry.x, y: sourceExit.y }, targetEntry, target.point]);
  }

  return compactRoute([source.point, sourceExit, { x: sourceExit.x, y: targetEntry.y }, targetEntry, target.point]);
}

export function connectorPath(points: NodePosition[]) {
  if (points.length < 2) return '';
  return points.map((point, index) => `${index === 0 ? 'M' : 'L'} ${formatPoint(point)}`).join(' ');
}

function offsetBySide(point: NodePosition, side: PortSide, distance: number): NodePosition {
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

function isHorizontal(side: PortSide) {
  return side === 'left' || side === 'right';
}

function compactRoute(points: NodePosition[]) {
  return points.reduce<NodePosition[]>((route, point) => {
    const last = route[route.length - 1];
    if (last && last.x === point.x && last.y === point.y) return route;

    const previous = route[route.length - 2];
    if (previous && last && isCollinear(previous, last, point)) {
      return [...route.slice(0, -1), point];
    }

    return [...route, point];
  }, []);
}

function isCollinear(a: NodePosition, b: NodePosition, c: NodePosition) {
  return (a.x === b.x && b.x === c.x) || (a.y === b.y && b.y === c.y);
}

function formatPoint(point: NodePosition) {
  return `${formatNumber(point.x)} ${formatNumber(point.y)}`;
}

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}
