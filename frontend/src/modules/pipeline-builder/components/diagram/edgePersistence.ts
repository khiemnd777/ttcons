import type { PipelineTemplateEdge } from '../../../../lib/types/domain';
import type { ConnectorModel, DiagramNodeRect, NodePosition, PortRef, PortSide } from './diagramTypes';
import { connectorEndpoint } from './geometry';

type DiagramEdgeEndpointMetadata = {
  nodeId?: string;
  side?: PortSide;
};

type DiagramEdgeMetadata = {
  source?: DiagramEdgeEndpointMetadata;
  target?: DiagramEdgeEndpointMetadata;
  route?: NodePosition[];
};

const PORT_SIDES = new Set<PortSide>(['top', 'right', 'bottom', 'left']);

export type ConnectorCreateInput = {
  source: PortRef;
  target: PortRef;
};

export type ConnectorReconnectInput = ConnectorCreateInput & {
  connectorId: string;
};

export function templateEdgeKey(sourceNodeId: string, targetNodeId: string) {
  return `${sourceNodeId}->${targetNodeId}`;
}

export function buildCreateEdgePayload(input: ConnectorCreateInput) {
  return {
    from_node_id: input.source.nodeId,
    to_node_id: input.target.nodeId,
    condition_type: 'always',
    condition_config: {
      diagram: {
        version: 1,
        source: {
          nodeId: input.source.nodeId,
          side: input.source.side,
        },
        target: {
          nodeId: input.target.nodeId,
          side: input.target.side,
        },
      },
    },
    label: 'Hoàn tất',
  };
}

export function buildUpdateEdgePayload(edge: PipelineTemplateEdge | undefined, input: ConnectorReconnectInput) {
  const conditionConfig = isRecord(edge?.condition_config) ? edge.condition_config : {};

  return {
    from_node_id: input.source.nodeId,
    to_node_id: input.target.nodeId,
    condition_config: {
      ...conditionConfig,
      diagram: {
        version: 1,
        source: {
          nodeId: input.source.nodeId,
          side: input.source.side,
        },
        target: {
          nodeId: input.target.nodeId,
          side: input.target.side,
        },
      },
    },
  };
}

export function buildConnectorModels(edges: PipelineTemplateEdge[], nodeRects: DiagramNodeRect[]): ConnectorModel[] {
  const rectById = new Map(nodeRects.map((rect) => [rect.nodeId, rect]));

  return edges.flatMap((edge, index) => {
    const connector = edgeToConnector(edge, rectById, index);
    return connector ? [connector] : [];
  });
}

export function edgeToConnector(edge: PipelineTemplateEdge, rectById: Map<string, DiagramNodeRect>, index = 0): ConnectorModel | null {
  const sourceNodeId = String(edge.from_node_id ?? '');
  const targetNodeId = String(edge.to_node_id ?? '');
  if (!sourceNodeId || !targetNodeId) return null;

  const metadata = readDiagramMetadata(edge);
  const sourceRect = rectById.get(resolveNodeId(metadata?.source?.nodeId, sourceNodeId, rectById));
  const targetRect = rectById.get(resolveNodeId(metadata?.target?.nodeId, targetNodeId, rectById));
  if (!sourceRect || !targetRect) return null;

  const source = connectorEndpoint(sourceRect, metadata?.source?.side ?? 'right');
  const target = connectorEndpoint(targetRect, metadata?.target?.side ?? 'left');
  const route = metadata?.route && source.point && target.point ? [source.point, ...metadata.route.slice(1, -1), target.point] : undefined;

  return {
    id: String(edge.id || `${sourceNodeId}-${targetNodeId}-${index}`),
    source,
    target,
    route,
    label: typeof edge.label === 'string' && edge.label.trim() ? edge.label : undefined,
  };
}

function resolveNodeId(candidate: string | undefined, fallback: string, rectById: Map<string, DiagramNodeRect>) {
  return candidate && rectById.has(candidate) ? candidate : fallback;
}

function readDiagramMetadata(edge: PipelineTemplateEdge): DiagramEdgeMetadata | null {
  const config = edge.condition_config;
  if (!isRecord(config)) return null;

  const diagram = config.diagram;
  if (!isRecord(diagram)) return null;

  return {
    source: readEndpointMetadata(diagram.source ?? diagram.sourcePort),
    target: readEndpointMetadata(diagram.target ?? diagram.targetPort),
    route: readRouteMetadata(diagram.route ?? diagram.points),
  };
}

function readEndpointMetadata(value: unknown): DiagramEdgeEndpointMetadata | undefined {
  if (!isRecord(value)) return undefined;

  const nodeId = readString(value.nodeId ?? value.node_id);
  const side = readPortSide(value.side ?? value.portSide ?? value.port_side);
  if (!nodeId && !side) return undefined;

  return { nodeId, side };
}

function readRouteMetadata(value: unknown): NodePosition[] | undefined {
  if (!Array.isArray(value)) return undefined;

  const route = value.flatMap((point) => {
    if (!isRecord(point)) return [];
    const x = Number(point.x);
    const y = Number(point.y);
    return Number.isFinite(x) && Number.isFinite(y) ? [{ x, y }] : [];
  });

  return route.length >= 2 ? route : undefined;
}

function readPortSide(value: unknown): PortSide | undefined {
  return typeof value === 'string' && PORT_SIDES.has(value as PortSide) ? (value as PortSide) : undefined;
}

function readString(value: unknown) {
  return typeof value === 'string' && value.trim() ? value : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}
