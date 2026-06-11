import type { ConnectorEndpoint, ConnectorEndpointRole, ConnectorModel, DiagramNodeRect, NodePosition, PortRef, PortSide } from './diagramTypes';
import {
  CONNECTOR_ENDPOINT_CLEARANCE,
  PORT_SIDES,
  expandNodeRect,
  manhattanDistance,
  offsetPointBySide,
  orthogonalSegmentIntersectsRect,
  pointInNodeRect,
  portPoint,
  rectCenter,
} from './geometry';

const TERMINAL_NODE_PADDING = 4;
const OBSTACLE_PADDING = 24;
const GUIDE_PADDING = 36;
const PORT_EXIT_DISTANCE = 34;
const PORT_SLOT_SPACING = 18;
const PORT_SIDE_PADDING = 24;
const BEND_PENALTY = 24;
const ROUTED_CROSSING_PENALTY = 700;
const ROUTED_OVERLAP_PENALTY = 3600;
const LABEL_HEIGHT = 18;
const LABEL_MIN_WIDTH = 54;
const LABEL_MAX_WIDTH = 128;

type RouteAxis = 'horizontal' | 'vertical';
type SearchDirection = RouteAxis | 'none';

type RouteSegment = {
  from: NodePosition;
  to: NodePosition;
  axis: RouteAxis;
};

type EndpointSlot = {
  connectorId: string;
  role: ConnectorEndpointRole;
  endpoint: ConnectorEndpoint;
  oppositeEndpoint: ConnectorEndpoint;
  projection: number;
};

type LabelBox = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type ConnectorRouteLayout = {
  connectorId: string;
  source: ConnectorEndpoint;
  target: ConnectorEndpoint;
  route: NodePosition[];
  labelPoint: NodePosition | null;
};

export function routeConnectors({
  connectors,
  nodeRects,
}: {
  connectors: ConnectorModel[];
  nodeRects: DiagramNodeRect[];
}): Map<string, ConnectorRouteLayout> {
  const rectById = new Map(nodeRects.map((rect) => [rect.nodeId, rect]));
  const normalizedConnectors = connectors.map((connector) => normalizeConnectorPorts(connector, rectById, nodeRects));
  const slotOffsets = allocateEndpointSlots(normalizedConnectors, rectById);
  const layouts = new Map<string, ConnectorRouteLayout>();
  const routedSegments: RouteSegment[] = [];
  const labelBoxes: LabelBox[] = [];

  sortedConnectorsForRouting(normalizedConnectors).forEach((connector) => {
    const source = endpointWithAllocatedSlot(connector.source.port, rectById, slotOffsets.get(endpointSlotKey(connector.id, 'source')) ?? 0);
    const target = endpointWithAllocatedSlot(connector.target.port, rectById, slotOffsets.get(endpointSlotKey(connector.id, 'target')) ?? 0);

    if (!source.point || !target.point) return;

    const route = routeBetweenEndpoints(source, target, nodeRects, routedSegments);
    const labelPoint = connector.label ? placeConnectorLabel(connector.label, route, nodeRects, labelBoxes) : null;

    layouts.set(connector.id, {
      connectorId: connector.id,
      source,
      target,
      route,
      labelPoint,
    });
    routedSegments.push(...routeSegments(route));
  });

  return layouts;
}

export function routePreviewConnector(source: ConnectorEndpoint, target: ConnectorEndpoint, nodeRects: DiagramNodeRect[]) {
  if (!source.point || !target.point) return [];
  return routeBetweenEndpoints(source, target, nodeRects, []);
}

function normalizeConnectorPorts(
  connector: ConnectorModel,
  rectById: Map<string, DiagramNodeRect>,
  nodeRects: DiagramNodeRect[],
): ConnectorModel {
  if (connector.portMode === 'manual') return connector;

  const sourceRect = rectById.get(connector.source.port.nodeId);
  const targetRect = rectById.get(connector.target.port.nodeId);
  if (!sourceRect || !targetRect) return connector;

  const pair = chooseAutoPortPair(sourceRect, targetRect, nodeRects);
  return {
    ...connector,
    source: {
      ...connector.source,
      port: { ...connector.source.port, side: pair.sourceSide },
    },
    target: {
      ...connector.target,
      port: { ...connector.target.port, side: pair.targetSide },
    },
  };
}

function chooseAutoPortPair(sourceRect: DiagramNodeRect, targetRect: DiagramNodeRect, nodeRects: DiagramNodeRect[]) {
  const sourceCenter = rectCenter(sourceRect);
  const targetCenter = rectCenter(targetRect);
  const dx = targetCenter.x - sourceCenter.x;
  const dy = targetCenter.y - sourceCenter.y;

  return PORT_SIDES.flatMap((sourceSide) =>
    PORT_SIDES.map((targetSide) => {
      const source = endpointWithSlot({ nodeId: sourceRect.nodeId, side: sourceSide }, sourceRect, 0);
      const target = endpointWithSlot({ nodeId: targetRect.nodeId, side: targetSide }, targetRect, 0);
      const route = source.point && target.point ? routeBetweenEndpoints(source, target, nodeRects, []) : [];
      const score = scoreAutoPortRoute(route, sourceSide, targetSide, dx, dy);
      return { sourceSide, targetSide, score };
    }),
  ).reduce((best, candidate) => (candidate.score < best.score ? candidate : best));
}

function scoreAutoPortRoute(route: NodePosition[], sourceSide: PortSide, targetSide: PortSide, dx: number, dy: number) {
  if (route.length < 2) return Number.MAX_SAFE_INTEGER;

  const length = routeLength(route);
  const bends = countBends(route);
  const facingPenalty = sourceFacingPenalty(sourceSide, dx, dy) + targetFacingPenalty(targetSide, dx, dy);
  const sameSidePenalty = sourceSide === targetSide ? 80 : 0;

  return length + bends * 30 + facingPenalty + sameSidePenalty + sideTieBreaker(sourceSide, targetSide);
}

function sourceFacingPenalty(side: PortSide, dx: number, dy: number) {
  switch (side) {
    case 'top':
      return dy <= 0 ? 0 : Math.abs(dy) * 1.4;
    case 'right':
      return dx >= 0 ? 0 : Math.abs(dx) * 1.4;
    case 'bottom':
      return dy >= 0 ? 0 : Math.abs(dy) * 1.4;
    case 'left':
      return dx <= 0 ? 0 : Math.abs(dx) * 1.4;
  }
}

function targetFacingPenalty(side: PortSide, dx: number, dy: number) {
  switch (side) {
    case 'top':
      return dy >= 0 ? 0 : Math.abs(dy) * 1.4;
    case 'right':
      return dx <= 0 ? 0 : Math.abs(dx) * 1.4;
    case 'bottom':
      return dy <= 0 ? 0 : Math.abs(dy) * 1.4;
    case 'left':
      return dx >= 0 ? 0 : Math.abs(dx) * 1.4;
  }
}

function sideTieBreaker(sourceSide: PortSide, targetSide: PortSide) {
  return PORT_SIDES.indexOf(sourceSide) * 0.1 + PORT_SIDES.indexOf(targetSide) * 0.01;
}

function allocateEndpointSlots(connectors: ConnectorModel[], rectById: Map<string, DiagramNodeRect>) {
  const groups = connectors.reduce((map, connector) => {
    addEndpointSlot(map, connector, 'source', rectById);
    addEndpointSlot(map, connector, 'target', rectById);
    return map;
  }, new Map<string, EndpointSlot[]>());

  const offsets = new Map<string, number>();
  groups.forEach((items) => {
    [...items]
      .sort((left, right) => left.projection - right.projection || left.connectorId.localeCompare(right.connectorId) || left.role.localeCompare(right.role))
      .forEach((item, index, sorted) => {
        offsets.set(endpointSlotKey(item.connectorId, item.role), (index - (sorted.length - 1) / 2) * PORT_SLOT_SPACING);
      });
  });

  return offsets;
}

function addEndpointSlot(
  groups: Map<string, EndpointSlot[]>,
  connector: ConnectorModel,
  role: ConnectorEndpointRole,
  rectById: Map<string, DiagramNodeRect>,
) {
  const endpoint = role === 'source' ? connector.source : connector.target;
  const oppositeEndpoint = role === 'source' ? connector.target : connector.source;
  const rect = rectById.get(endpoint.port.nodeId);
  if (!rect) return;

  const oppositeRect = rectById.get(oppositeEndpoint.port.nodeId);
  const oppositeCenter = oppositeRect ? rectCenter(oppositeRect) : endpoint.point;
  const projection = oppositeCenter ? endpointProjection(endpoint.port.side, oppositeCenter) : 0;
  const groupKey = `${endpoint.port.nodeId}:${endpoint.port.side}`;
  const group = groups.get(groupKey) ?? [];
  group.push({
    connectorId: connector.id,
    role,
    endpoint,
    oppositeEndpoint,
    projection,
  });
  groups.set(groupKey, group);
}

function endpointProjection(side: PortSide, point: NodePosition) {
  return side === 'top' || side === 'bottom' ? point.x : point.y;
}

function endpointWithAllocatedSlot(port: PortRef, rectById: Map<string, DiagramNodeRect>, slotOffset: number): ConnectorEndpoint {
  const rect = rectById.get(port.nodeId);
  return rect ? endpointWithSlot(port, rect, slotOffset) : { port };
}

function endpointWithSlot(port: PortRef, rect: DiagramNodeRect, slotOffset: number): ConnectorEndpoint {
  const boundaryPoint = clampPortPointToSide(portPoint(rect, port.side), rect, port.side, slotOffset);
  return {
    port,
    point: offsetPointBySide(boundaryPoint, port.side, CONNECTOR_ENDPOINT_CLEARANCE),
  };
}

function clampPortPointToSide(point: NodePosition, rect: DiagramNodeRect, side: PortSide, slotOffset: number) {
  if (side === 'top' || side === 'bottom') {
    return {
      x: clamp(point.x + slotOffset, rect.x + PORT_SIDE_PADDING, rect.x + rect.width - PORT_SIDE_PADDING),
      y: point.y,
    };
  }

  return {
    x: point.x,
    y: clamp(point.y + slotOffset, rect.y + PORT_SIDE_PADDING, rect.y + rect.height - PORT_SIDE_PADDING),
  };
}

function routeBetweenEndpoints(
  source: ConnectorEndpoint,
  target: ConnectorEndpoint,
  nodeRects: DiagramNodeRect[],
  routedSegments: RouteSegment[],
) {
  if (!source.point || !target.point) return [];

  const sourceExit = offsetPointBySide(source.point, source.port.side, PORT_EXIT_DISTANCE);
  const targetEntry = offsetPointBySide(target.point, target.port.side, PORT_EXIT_DISTANCE);
  const avoidRects = buildAvoidRects(nodeRects, source.port.nodeId, target.port.nodeId);
  const midRoute = findOrthogonalPath(sourceExit, targetEntry, avoidRects, routedSegments);
  const route = midRoute.length ? [source.point, ...midRoute, target.point] : fallbackOuterRoute(source, target, sourceExit, targetEntry, avoidRects);

  return compactRoute(route);
}

function findOrthogonalPath(
  start: NodePosition,
  end: NodePosition,
  obstacles: DiagramNodeRect[],
  routedSegments: RouteSegment[],
): NodePosition[] {
  const graph = buildVisibilityGraph(start, end, obstacles);
  const startIndex = graph.pointIndex.get(pointKey(start));
  const endIndex = graph.pointIndex.get(pointKey(end));
  if (startIndex === undefined || endIndex === undefined) return [];

  const startState = stateKey(startIndex, 'none');
  const open: Array<{ key: string; pointIndex: number; direction: SearchDirection; cost: number; priority: number }> = [
    {
      key: startState,
      pointIndex: startIndex,
      direction: 'none',
      cost: 0,
      priority: manhattanDistance(start, end),
    },
  ];
  const costs = new Map<string, number>([[startState, 0]]);
  const previous = new Map<string, string>();
  const statePoints = new Map<string, number>([[startState, startIndex]]);

  while (open.length) {
    open.sort((left, right) => left.priority - right.priority || left.cost - right.cost);
    const current = open.shift();
    if (!current) break;
    if ((costs.get(current.key) ?? Number.POSITIVE_INFINITY) < current.cost) continue;

    if (current.pointIndex === endIndex) {
      return reconstructRoute(current.key, graph.points, statePoints, previous);
    }

    graph.neighbors(current.pointIndex).forEach((neighbor) => {
      const from = graph.points[current.pointIndex];
      const to = graph.points[neighbor.pointIndex];
      if (!from || !to) return;

      const axis = segmentAxis(from, to);
      if (!axis) return;
      if (segmentIntersectsAnyRect(from, to, obstacles)) return;

      const bendCost = current.direction !== 'none' && current.direction !== axis ? BEND_PENALTY : 0;
      const routedCost = routedSegmentPenalty({ from, to, axis }, routedSegments);
      const nextCost = current.cost + manhattanDistance(from, to) + bendCost + routedCost;
      const nextKey = stateKey(neighbor.pointIndex, axis);
      if (nextCost >= (costs.get(nextKey) ?? Number.POSITIVE_INFINITY)) return;

      costs.set(nextKey, nextCost);
      previous.set(nextKey, current.key);
      statePoints.set(nextKey, neighbor.pointIndex);
      open.push({
        key: nextKey,
        pointIndex: neighbor.pointIndex,
        direction: axis,
        cost: nextCost,
        priority: nextCost + manhattanDistance(to, end),
      });
    });
  }

  return [];
}

function buildVisibilityGraph(start: NodePosition, end: NodePosition, obstacles: DiagramNodeRect[]) {
  const xs = stableUniqueNumbers([
    start.x,
    end.x,
    start.x - GUIDE_PADDING,
    start.x + GUIDE_PADDING,
    end.x - GUIDE_PADDING,
    end.x + GUIDE_PADDING,
    ...obstacles.flatMap((rect) => [rect.x - GUIDE_PADDING, rect.x + rect.width + GUIDE_PADDING]),
  ]);
  const ys = stableUniqueNumbers([
    start.y,
    end.y,
    start.y - GUIDE_PADDING,
    start.y + GUIDE_PADDING,
    end.y - GUIDE_PADDING,
    end.y + GUIDE_PADDING,
    ...obstacles.flatMap((rect) => [rect.y - GUIDE_PADDING, rect.y + rect.height + GUIDE_PADDING]),
  ]);
  const points: NodePosition[] = [];
  const pointIndex = new Map<string, number>();

  xs.forEach((x) => {
    ys.forEach((y) => {
      const point = { x, y };
      if (!samePoint(point, start) && !samePoint(point, end) && pointInsideAnyRect(point, obstacles)) return;
      addGraphPoint(point, points, pointIndex);
    });
  });
  addGraphPoint(start, points, pointIndex);
  addGraphPoint(end, points, pointIndex);

  const rows = groupPointIndexes(points, 'y', 'x');
  const columns = groupPointIndexes(points, 'x', 'y');

  return {
    points,
    pointIndex,
    neighbors(point: number) {
      return [
        ...adjacentPointIndexes(point, rows.get(formatNumber(points[point].y)) ?? []),
        ...adjacentPointIndexes(point, columns.get(formatNumber(points[point].x)) ?? []),
      ].map((pointIndex) => ({ pointIndex }));
    },
  };
}

function addGraphPoint(point: NodePosition, points: NodePosition[], pointIndex: Map<string, number>) {
  const key = pointKey(point);
  if (pointIndex.has(key)) return;
  pointIndex.set(key, points.length);
  points.push({ x: round(point.x), y: round(point.y) });
}

function groupPointIndexes(points: NodePosition[], keyAxis: 'x' | 'y', sortAxis: 'x' | 'y') {
  const groups = points.reduce((map, point, index) => {
    const key = formatNumber(point[keyAxis]);
    const group = map.get(key) ?? [];
    group.push(index);
    map.set(key, group);
    return map;
  }, new Map<string, number[]>());

  groups.forEach((group) => {
    group.sort((left, right) => points[left][sortAxis] - points[right][sortAxis]);
  });

  return groups;
}

function adjacentPointIndexes(pointIndex: number, group: number[]) {
  const index = group.indexOf(pointIndex);
  if (index === -1) return [];

  return [group[index - 1], group[index + 1]].filter((candidate): candidate is number => candidate !== undefined);
}

function reconstructRoute(
  endState: string,
  points: NodePosition[],
  statePoints: Map<string, number>,
  previous: Map<string, string>,
) {
  const route: NodePosition[] = [];
  let current: string | undefined = endState;

  while (current) {
    const pointIndex = statePoints.get(current);
    if (pointIndex !== undefined) route.push(points[pointIndex]);
    current = previous.get(current);
  }

  return compactRoute(route.reverse());
}

function fallbackOuterRoute(
  source: ConnectorEndpoint,
  target: ConnectorEndpoint,
  sourceExit: NodePosition,
  targetEntry: NodePosition,
  obstacles: DiagramNodeRect[],
) {
  if (!source.point || !target.point) return [];

  const sourcePoint = source.point;
  const targetPoint = target.point;
  const bounds = obstacleBounds(obstacles, sourceExit, targetEntry);
  const lanes = [
    { x: bounds.x - GUIDE_PADDING, y: null },
    { x: bounds.x + bounds.width + GUIDE_PADDING, y: null },
    { x: null, y: bounds.y - GUIDE_PADDING },
    { x: null, y: bounds.y + bounds.height + GUIDE_PADDING },
  ];

  const candidates = lanes.map((lane) => {
    if (lane.x !== null) {
      return compactRoute([sourcePoint, sourceExit, { x: lane.x, y: sourceExit.y }, { x: lane.x, y: targetEntry.y }, targetEntry, targetPoint]);
    }
    const y = lane.y ?? sourceExit.y;
    return compactRoute([sourcePoint, sourceExit, { x: sourceExit.x, y }, { x: targetEntry.x, y }, targetEntry, targetPoint]);
  });

  return candidates.reduce((best, route) => {
    const score = scoreFallbackRoute(route, obstacles);
    return score < scoreFallbackRoute(best, obstacles) ? route : best;
  }, candidates[0]);
}

function scoreFallbackRoute(route: NodePosition[], obstacles: DiagramNodeRect[]) {
  const collisions = routeSegments(route).reduce(
    (total, segment) => total + obstacles.filter((rect) => orthogonalSegmentIntersectsRect(segment.from, segment.to, rect)).length,
    0,
  );
  return collisions * 100_000 + routeLength(route) + countBends(route) * 30;
}

function buildAvoidRects(nodeRects: DiagramNodeRect[], sourceNodeId: string, targetNodeId: string) {
  return nodeRects.map((rect) =>
    rect.nodeId === sourceNodeId || rect.nodeId === targetNodeId ? expandNodeRect(rect, TERMINAL_NODE_PADDING) : expandNodeRect(rect, OBSTACLE_PADDING),
  );
}

function sortedConnectorsForRouting(connectors: ConnectorModel[]) {
  return [...connectors].sort((left, right) => connectorPairKey(left).localeCompare(connectorPairKey(right)) || left.id.localeCompare(right.id));
}

function connectorPairKey(connector: ConnectorModel) {
  return [connector.source.port.nodeId, connector.target.port.nodeId].sort().join('|');
}

function placeConnectorLabel(label: string, route: NodePosition[], nodeRects: DiagramNodeRect[], labelBoxes: LabelBox[]) {
  const width = clamp(label.length * 7 + 18, LABEL_MIN_WIDTH, LABEL_MAX_WIDTH);
  const avoidRects = nodeRects.map((rect) => expandNodeRect(rect, 6));
  const candidates = labelCandidates(route, width);

  for (const point of candidates) {
    const box = labelBox(point, width);
    if (avoidRects.some((rect) => labelBoxIntersectsRect(box, rect))) continue;
    if (labelBoxes.some((candidate) => labelBoxesIntersect(box, candidate))) continue;
    labelBoxes.push(box);
    return point;
  }

  const fallback = route[Math.floor(route.length / 2)] ?? null;
  if (!fallback) return null;
  labelBoxes.push(labelBox(fallback, width));
  return fallback;
}

function labelCandidates(route: NodePosition[], width: number) {
  return routeSegments(route)
    .filter((segment) => segmentLength(segment) >= width * 0.65)
    .sort((left, right) => {
      const axisBias = left.axis === right.axis ? 0 : left.axis === 'horizontal' ? -1 : 1;
      return axisBias || segmentLength(right) - segmentLength(left);
    })
    .flatMap((segment) => labelPointsForSegment(segment));
}

function labelPointsForSegment(segment: RouteSegment) {
  const midpoint = {
    x: (segment.from.x + segment.to.x) / 2,
    y: (segment.from.y + segment.to.y) / 2,
  };
  const offsets = [0, -14, 14, -28, 28];

  return offsets.map((offset) =>
    segment.axis === 'horizontal'
      ? { x: midpoint.x, y: midpoint.y - 12 + offset }
      : { x: midpoint.x + 12 + offset, y: midpoint.y },
  );
}

function labelBox(point: NodePosition, width: number): LabelBox {
  return {
    x: point.x - width / 2,
    y: point.y - LABEL_HEIGHT / 2,
    width,
    height: LABEL_HEIGHT,
  };
}

function labelBoxIntersectsRect(box: LabelBox, rect: DiagramNodeRect) {
  return box.x <= rect.x + rect.width && box.x + box.width >= rect.x && box.y <= rect.y + rect.height && box.y + box.height >= rect.y;
}

function labelBoxesIntersect(left: LabelBox, right: LabelBox) {
  return left.x <= right.x + right.width && left.x + left.width >= right.x && left.y <= right.y + right.height && left.y + left.height >= right.y;
}

function routeSegments(route: NodePosition[]) {
  const segments: RouteSegment[] = [];

  for (let index = 1; index < route.length; index += 1) {
    const from = route[index - 1];
    const to = route[index];
    const axis = segmentAxis(from, to);
    if (!axis || samePoint(from, to)) continue;
    segments.push({ from, to, axis });
  }

  return segments;
}

function segmentAxis(from: NodePosition, to: NodePosition): RouteAxis | null {
  if (from.y === to.y) return 'horizontal';
  if (from.x === to.x) return 'vertical';
  return null;
}

function segmentIntersectsAnyRect(from: NodePosition, to: NodePosition, rects: DiagramNodeRect[]) {
  return rects.some((rect) => orthogonalSegmentIntersectsRect(from, to, rect));
}

function pointInsideAnyRect(point: NodePosition, rects: DiagramNodeRect[]) {
  return rects.some((rect) => pointInNodeRect(point, rect));
}

function routedSegmentPenalty(segment: RouteSegment, routedSegments: RouteSegment[]) {
  return routedSegments.reduce((penalty, routedSegment) => {
    if (segmentsOverlap(segment, routedSegment)) return penalty + ROUTED_OVERLAP_PENALTY + overlapLength(segment, routedSegment) * 8;
    if (segmentsCross(segment, routedSegment)) return penalty + ROUTED_CROSSING_PENALTY;
    return penalty;
  }, 0);
}

function segmentsOverlap(left: RouteSegment, right: RouteSegment) {
  if (left.axis !== right.axis) return false;

  if (left.axis === 'horizontal') {
    return left.from.y === right.from.y && intervalsOverlap(normalizeInterval(left.from.x, left.to.x), normalizeInterval(right.from.x, right.to.x));
  }

  return left.from.x === right.from.x && intervalsOverlap(normalizeInterval(left.from.y, left.to.y), normalizeInterval(right.from.y, right.to.y));
}

function overlapLength(left: RouteSegment, right: RouteSegment) {
  if (!segmentsOverlap(left, right)) return 0;

  const leftInterval = left.axis === 'horizontal' ? normalizeInterval(left.from.x, left.to.x) : normalizeInterval(left.from.y, left.to.y);
  const rightInterval = right.axis === 'horizontal' ? normalizeInterval(right.from.x, right.to.x) : normalizeInterval(right.from.y, right.to.y);
  return Math.max(0, Math.min(leftInterval[1], rightInterval[1]) - Math.max(leftInterval[0], rightInterval[0]));
}

function segmentsCross(left: RouteSegment, right: RouteSegment) {
  if (left.axis === right.axis) return false;

  const horizontal = left.axis === 'horizontal' ? left : right;
  const vertical = left.axis === 'vertical' ? left : right;
  const horizontalX = normalizeInterval(horizontal.from.x, horizontal.to.x);
  const verticalY = normalizeInterval(vertical.from.y, vertical.to.y);

  return (
    vertical.from.x > horizontalX[0] &&
    vertical.from.x < horizontalX[1] &&
    horizontal.from.y > verticalY[0] &&
    horizontal.from.y < verticalY[1]
  );
}

function intervalsOverlap(left: [number, number], right: [number, number]) {
  return Math.min(left[1], right[1]) > Math.max(left[0], right[0]);
}

function normalizeInterval(left: number, right: number): [number, number] {
  return left < right ? [left, right] : [right, left];
}

function routeLength(route: NodePosition[]) {
  return route.reduce((total, point, index) => (index === 0 ? 0 : total + manhattanDistance(route[index - 1], point)), 0);
}

function countBends(route: NodePosition[]) {
  let bends = 0;
  for (let index = 2; index < route.length; index += 1) {
    const previousAxis = segmentAxis(route[index - 2], route[index - 1]);
    const nextAxis = segmentAxis(route[index - 1], route[index]);
    if (previousAxis && nextAxis && previousAxis !== nextAxis) bends += 1;
  }
  return bends;
}

function segmentLength(segment: RouteSegment) {
  return manhattanDistance(segment.from, segment.to);
}

function compactRoute(points: NodePosition[]) {
  return points.reduce<NodePosition[]>((route, point) => {
    const rounded = { x: round(point.x), y: round(point.y) };
    const last = route[route.length - 1];
    if (last && samePoint(last, rounded)) return route;

    const previous = route[route.length - 2];
    if (previous && last && isCollinear(previous, last, rounded)) {
      return [...route.slice(0, -1), rounded];
    }

    return [...route, rounded];
  }, []);
}

function isCollinear(a: NodePosition, b: NodePosition, c: NodePosition) {
  return (a.x === b.x && b.x === c.x) || (a.y === b.y && b.y === c.y);
}

function obstacleBounds(rects: DiagramNodeRect[], ...points: NodePosition[]) {
  const xs = [...rects.flatMap((rect) => [rect.x, rect.x + rect.width]), ...points.map((point) => point.x)];
  const ys = [...rects.flatMap((rect) => [rect.y, rect.y + rect.height]), ...points.map((point) => point.y)];
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const minY = Math.min(...ys);
  const maxY = Math.max(...ys);

  return {
    x: minX,
    y: minY,
    width: maxX - minX,
    height: maxY - minY,
  };
}

function endpointSlotKey(connectorId: string, role: ConnectorEndpointRole) {
  return `${connectorId}:${role}`;
}

function stateKey(pointIndex: number, direction: SearchDirection) {
  return `${pointIndex}:${direction}`;
}

function pointKey(point: NodePosition) {
  return `${formatNumber(point.x)}:${formatNumber(point.y)}`;
}

function stableUniqueNumbers(values: number[]) {
  return Array.from(new Set(values.map(round))).sort((left, right) => left - right);
}

function samePoint(left: NodePosition, right: NodePosition) {
  return left.x === right.x && left.y === right.y;
}

function clamp(value: number, min: number, max: number) {
  if (min > max) return (min + max) / 2;
  return Math.min(Math.max(value, min), max);
}

function round(value: number) {
  return Number(value.toFixed(2));
}

function formatNumber(value: number) {
  return String(round(value));
}
