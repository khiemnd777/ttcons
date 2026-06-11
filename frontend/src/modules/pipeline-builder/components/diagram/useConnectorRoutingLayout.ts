import { useMemo, useRef } from 'react';
import type { ConnectorModel, DiagramNodeRect } from './diagramTypes';
import { routeConnectors, type ConnectorRouteLayout } from './routingEngine';

type RoutingCache = {
  key: string;
  layouts: Map<string, ConnectorRouteLayout>;
};

type ConnectorRoutingLayoutOptions = {
  draggingNodeId?: string;
  liveNodeRects?: DiagramNodeRect[];
};

export function useConnectorRoutingLayout(
  connectors: ConnectorModel[],
  nodeRects: DiagramNodeRect[],
  options: ConnectorRoutingLayoutOptions = {},
) {
  const cacheRef = useRef<RoutingCache | null>(null);
  const key = useMemo(() => routingLayoutKey(connectors, nodeRects), [connectors, nodeRects]);

  const baseLayouts = useMemo(() => {
    if (cacheRef.current?.key === key) return cacheRef.current.layouts;

    const layouts = routeConnectors({ connectors, nodeRects });
    cacheRef.current = { key, layouts };
    return layouts;
  }, [connectors, key, nodeRects]);

  return useMemo(() => {
    const draggingNodeId = options.draggingNodeId;
    const liveNodeRects = options.liveNodeRects;
    if (!draggingNodeId || !liveNodeRects) return baseLayouts;

    const incidentConnectors = connectors.filter((connector) => connectorTouchesNode(connector, draggingNodeId));
    if (!incidentConnectors.length) return baseLayouts;

    const liveLayouts = routeConnectors({ connectors: incidentConnectors, nodeRects: liveNodeRects });
    if (!liveLayouts.size) return baseLayouts;

    const layouts = new Map(baseLayouts);
    liveLayouts.forEach((layout, connectorId) => {
      layouts.set(connectorId, layout);
    });
    return layouts;
  }, [baseLayouts, connectors, options.draggingNodeId, options.liveNodeRects]);
}

function routingLayoutKey(connectors: ConnectorModel[], nodeRects: DiagramNodeRect[]) {
  return [connectorLayoutKey(connectors), nodeRectLayoutKey(nodeRects)].join('::');
}

function connectorLayoutKey(connectors: ConnectorModel[]) {
  return connectors
    .map((connector) =>
      [
        connector.id,
        connector.source.port.nodeId,
        connector.source.port.side,
        connector.target.port.nodeId,
        connector.target.port.side,
        connector.portMode ?? 'auto',
        connector.label ?? '',
      ].join(':'),
    )
    .sort()
    .join('|');
}

function nodeRectLayoutKey(nodeRects: DiagramNodeRect[]) {
  return nodeRects
    .map((rect) => [rect.nodeId, formatNumber(rect.x), formatNumber(rect.y), formatNumber(rect.width), formatNumber(rect.height)].join(':'))
    .sort()
    .join('|');
}

function connectorTouchesNode(connector: ConnectorModel, nodeId: string) {
  return connector.source.port.nodeId === nodeId || connector.target.port.nodeId === nodeId;
}

function formatNumber(value: number) {
  return Number.isInteger(value) ? String(value) : value.toFixed(2);
}
