import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
  type KeyboardEvent,
  type MouseEvent,
  type PointerEvent,
} from 'react';
import clsx from 'clsx';
import { Minus, Plus } from 'lucide-react';
import { EmptyState } from '../../../../components/EmptyState';
import type { PipelineNode, PipelineTemplateEdge } from '../../../../lib/types/domain';
import { GRID_SIZE, initialNodePosition, nodePosition } from './diagramConstants';
import type {
  ConnectorEndpoint,
  ConnectorEndpointRole,
  ConnectorModel,
  DiagramNodeRect,
  DragState,
  NodePosition,
  NodeSize,
  PanState,
  PortRef,
  PortSide,
} from './diagramTypes';
import { clientPointToWorld, nodeRectsFromNodes, samePoint, visibleConnectorEndpoint } from './geometry';
import { hitTestNode, nearestPortHit } from './hitTesting';
import { AdornerLayer } from './AdornerLayer';
import { ConnectorLayer } from './ConnectorLayer';
import { DragPreviewLayer } from './DragPreviewLayer';
import { buildConnectorModels, templateEdgeKey, type ConnectorCreateInput, type ConnectorReconnectInput } from './edgePersistence';
import {
  diagramInteractionReducer,
  initialDiagramInteractionState,
  type ConnectorDraft,
  type ConnectorInvalidReason,
  type ConnectorReconnectDraft,
} from './interactionReducer';
import { NodeLayer } from './NodeLayer';

const ADORNER_HIT_PADDING = 32;
const CONNECTOR_SNAP_RADIUS = 34;
const MIN_ZOOM = 0.5;
const MAX_ZOOM = 2;
const ZOOM_STEP = 0.1;
const CONTEXT_MENU_WIDTH = 168;
const CONTEXT_MENU_HEIGHT = 44;

type PendingNodePosition = {
  nodeId: string;
  position: NodePosition;
};

type CanvasContextMenu =
  | {
      type: 'node';
      id: string;
      x: number;
      y: number;
    }
  | {
      type: 'connector';
      id: string;
      x: number;
      y: number;
    };

type DiagramCanvasProps = {
  nodes: PipelineNode[];
  edges: PipelineTemplateEdge[];
  selectedNodeId: string;
  connectorMutationPending: boolean;
  onSelectNode: (nodeId: string) => void;
  onSelectedConnectorChange: (connectorId: string) => void;
  onToggleNodeDetails: (nodeId: string) => void;
  onSaveNodePosition: (nodeId: string, position: NodePosition) => void;
  onCreateConnector: (input: ConnectorCreateInput) => void;
  onReconnectConnector: (input: ConnectorReconnectInput) => void;
  onDeleteConnector: (connectorId: string) => void;
  onDeleteNode: (nodeId: string) => void;
};

export function DiagramCanvas({
  nodes,
  edges,
  selectedNodeId,
  connectorMutationPending,
  onSelectNode,
  onSelectedConnectorChange,
  onToggleNodeDetails,
  onSaveNodePosition,
  onCreateConnector,
  onReconnectConnector,
  onDeleteConnector,
  onDeleteNode,
}: DiagramCanvasProps) {
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const dragRef = useRef<DragState | null>(null);
  const panRef = useRef<PanState | null>(null);
  const connectorDraftRef = useRef<ConnectorDraft | null>(null);
  const reconnectDraftRef = useRef<ConnectorReconnectDraft | null>(null);
  const interactionRef = useRef(initialDiagramInteractionState);
  const persistedPositionsRef = useRef<Record<string, NodePosition>>({});
  const resizeObserversRef = useRef<Record<string, ResizeObserver>>({});
  const settledNodeRectsRef = useRef<DiagramNodeRect[]>([]);
  const nodePositionsRef = useRef<Record<string, NodePosition>>({});
  const nodeRectsRef = useRef<DiagramNodeRect[]>([]);
  const edgeIdsByKeyRef = useRef<Map<string, Set<string>>>(new Map());
  const nodePositionFrameRef = useRef<number | null>(null);
  const pendingNodePositionRef = useRef<PendingNodePosition | null>(null);
  const [nodePositions, setNodePositions] = useState<Record<string, NodePosition>>({});
  const [nodeSizes, setNodeSizes] = useState<Record<string, NodeSize>>({});
  const [canvasOffset, setCanvasOffset] = useState<NodePosition>({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(1);
  const [isPanning, setIsPanning] = useState(false);
  const [draggingNodeId, setDraggingNodeId] = useState('');
  const [contextMenu, setContextMenu] = useState<CanvasContextMenu | null>(null);
  const [interaction, dispatchInteraction] = useReducer(diagramInteractionReducer, initialDiagramInteractionState);
  const nodeRects = useMemo(() => nodeRectsFromNodes(nodes, nodePositions, nodeSizes), [nodes, nodePositions, nodeSizes]);
  const routingNodeRects = draggingNodeId ? settledNodeRectsRef.current : nodeRects;
  const connectors = useMemo(() => buildConnectorModels(edges, routingNodeRects), [edges, routingNodeRects]);
  const edgeIdsByKey = useMemo(
    () =>
      edges.reduce((map, edge) => {
        const sourceNodeId = String(edge.from_node_id ?? '');
        const targetNodeId = String(edge.to_node_id ?? '');
        if (!sourceNodeId || !targetNodeId) return map;
        const key = templateEdgeKey(sourceNodeId, targetNodeId);
        const ids = map.get(key) ?? new Set<string>();
        ids.add(String(edge.id));
        map.set(key, ids);
        return map;
      }, new Map<string, Set<string>>()),
    [edges],
  );
  nodePositionsRef.current = nodePositions;
  nodeRectsRef.current = nodeRects;
  edgeIdsByKeyRef.current = edgeIdsByKey;
  interactionRef.current = interaction;
  const connectorDraft = interaction.connectorDraft;
  const reconnectDraft = interaction.reconnectDraft;
  const activeTargetPort = connectorDraft?.target ?? reconnectDraft?.target ?? null;
  const activeTargetValid = connectorDraft?.validTarget ?? reconnectDraft?.validTarget ?? false;
  const adornerNodeId = connectorDraft?.source.nodeId || (reconnectDraft ? '' : interaction.hoveredNodeId || selectedNodeId);
  const adornerRect = nodeRects.find((rect) => rect.nodeId === adornerNodeId) ?? null;
  const adornerMode = connectorDraft ? 'intent' : adornerNodeId === selectedNodeId ? 'selected' : 'hover';
  const targetAdornerRect =
    activeTargetPort && activeTargetPort.nodeId !== adornerRect?.nodeId
      ? nodeRects.find((rect) => rect.nodeId === activeTargetPort.nodeId) ?? null
      : null;
  const targetAdornerMode = activeTargetValid ? 'target' : 'invalid';
  const previewSource = connectorDraft
    ? endpointFromPort(connectorDraft.source, nodeRects)
    : reconnectDraft
      ? reconnectPreviewSource(reconnectDraft, nodeRects)
      : null;
  const previewTarget = connectorDraft
    ? previewTargetEndpoint(connectorDraft, nodeRects)
    : reconnectDraft
      ? reconnectPreviewTarget(reconnectDraft, nodeRects)
      : null;

  useEffect(() => {
    onSelectedConnectorChange(interaction.selectedConnectorId);
  }, [interaction.selectedConnectorId, onSelectedConnectorChange]);

  useEffect(() => {
    if (interaction.selectedConnectorId && !connectors.some((connector) => connector.id === interaction.selectedConnectorId)) {
      dispatchInteraction({ type: 'clear-selected-connector', connectorId: interaction.selectedConnectorId });
    }
  }, [connectors, interaction.selectedConnectorId]);

  useEffect(() => {
    const nodeIds = new Set(nodes.map((node) => node.id));
    const connectorDraft = connectorDraftRef.current;
    if (connectorDraft && !nodeIds.has(connectorDraft.source.nodeId)) {
      connectorDraftRef.current = null;
      dispatchInteraction({ type: 'cancel-connector-drag' });
    }

    const reconnectDraft = reconnectDraftRef.current;
    if (
      reconnectDraft &&
      (!nodeIds.has(reconnectDraft.fixed.nodeId) ||
        !nodeIds.has(reconnectDraft.originalSource.nodeId) ||
        !nodeIds.has(reconnectDraft.originalTarget.nodeId))
    ) {
      reconnectDraftRef.current = null;
      dispatchInteraction({ type: 'cancel-connector-reconnect' });
    }
  }, [nodes]);

  useEffect(() => {
    const previousPersisted = persistedPositionsRef.current;
    const nextPersisted: Record<string, NodePosition> = {};

    nodes.forEach((node, index) => {
      nextPersisted[node.id] = initialNodePosition(node, index);
    });

    setNodePositions((current) => {
      const next: Record<string, NodePosition> = {};
      let changed = Object.keys(current).length !== nodes.length;

      nodes.forEach((node, index) => {
        const persisted = nextPersisted[node.id];
        const persistedChanged = !samePoint(previousPersisted[node.id], persisted);
        const currentPosition = current[node.id];
        const nextPosition = !currentPosition || persistedChanged ? persisted : currentPosition;
        next[node.id] = nextPosition;
        if (!samePoint(currentPosition, nextPosition)) changed = true;
      });

      return changed ? next : current;
    });
    persistedPositionsRef.current = nextPersisted;
  }, [nodes]);

  useEffect(() => {
    const nodeIds = new Set(nodes.map((node) => node.id));
    Object.keys(resizeObserversRef.current).forEach((nodeId) => {
      if (nodeIds.has(nodeId)) return;
      resizeObserversRef.current[nodeId].disconnect();
      delete resizeObserversRef.current[nodeId];
    });
    setNodeSizes((current) => {
      let changed = false;
      const next: Record<string, NodeSize> = {};
      Object.entries(current).forEach(([nodeId, size]) => {
        if (!nodeIds.has(nodeId)) {
          changed = true;
          return;
        }
        next[nodeId] = size;
      });
      return changed ? next : current;
    });
  }, [nodes]);

  useEffect(
    () => () => {
      Object.values(resizeObserversRef.current).forEach((observer) => observer.disconnect());
      resizeObserversRef.current = {};
      if (nodePositionFrameRef.current !== null) {
        window.cancelAnimationFrame(nodePositionFrameRef.current);
        nodePositionFrameRef.current = null;
      }
    },
    [],
  );

  const setNodeElementRef = useCallback((nodeId: string, element: HTMLDivElement | null) => {
    resizeObserversRef.current[nodeId]?.disconnect();
    delete resizeObserversRef.current[nodeId];
    if (!element) return;

    const updateSize = () => {
      const nextSize = {
        width: element.offsetWidth,
        height: element.offsetHeight,
      };
      setNodeSizes((current) => {
        const currentSize = current[nodeId];
        if (sameSize(currentSize, nextSize)) return current;
        return { ...current, [nodeId]: nextSize };
      });
    };

    updateSize();
    const observer = new ResizeObserver(updateSize);
    observer.observe(element);
    resizeObserversRef.current[nodeId] = observer;
  }, []);

  const applyNodePosition = useCallback((nodeId: string, position: NodePosition) => {
    setNodePositions((current) => (samePoint(current[nodeId], position) ? current : { ...current, [nodeId]: position }));
  }, []);

  const scheduleNodePositionUpdate = useCallback((nodeId: string, position: NodePosition) => {
    pendingNodePositionRef.current = { nodeId, position };
    if (nodePositionFrameRef.current !== null) return;

    nodePositionFrameRef.current = window.requestAnimationFrame(() => {
      nodePositionFrameRef.current = null;
      const pending = pendingNodePositionRef.current;
      pendingNodePositionRef.current = null;
      if (!pending) return;
      applyNodePosition(pending.nodeId, pending.position);
    });
  }, [applyNodePosition]);

  const cancelScheduledNodePositionUpdate = useCallback(() => {
    if (nodePositionFrameRef.current !== null) {
      window.cancelAnimationFrame(nodePositionFrameRef.current);
      nodePositionFrameRef.current = null;
    }
    pendingNodePositionRef.current = null;
  }, []);

  const focusCanvas = useCallback(() => {
    canvasRef.current?.focus({ preventScroll: true });
  }, []);

  const canvasPointFromEvent = useCallback((event: PointerEvent<Element>) => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    return clientPointToWorld(event, canvas.getBoundingClientRect(), canvasOffset, zoom);
  }, [canvasOffset, zoom]);

  const closeContextMenu = useCallback(() => {
    setContextMenu(null);
  }, []);

  const contextMenuPointFromEvent = useCallback((event: MouseEvent<Element>) => {
    const canvas = canvasRef.current;
    if (!canvas) return null;
    const bounds = canvas.getBoundingClientRect();
    return {
      x: clamp(event.clientX - bounds.left, 8, Math.max(8, bounds.width - CONTEXT_MENU_WIDTH - 8)),
      y: clamp(event.clientY - bounds.top, 8, Math.max(8, bounds.height - CONTEXT_MENU_HEIGHT - 8)),
    };
  }, []);

  const zoomTo = useCallback((nextZoom: number, anchor?: NodePosition) => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const clampedZoom = clamp(roundZoom(nextZoom), MIN_ZOOM, MAX_ZOOM);
    if (clampedZoom === zoom) return;

    const bounds = canvas.getBoundingClientRect();
    const viewportAnchor = anchor ?? { x: bounds.width / 2, y: bounds.height / 2 };
    const worldAnchor = {
      x: (viewportAnchor.x - canvasOffset.x) / zoom,
      y: (viewportAnchor.y - canvasOffset.y) / zoom,
    };

    setZoom(clampedZoom);
    setCanvasOffset({
      x: viewportAnchor.x - worldAnchor.x * clampedZoom,
      y: viewportAnchor.y - worldAnchor.y * clampedZoom,
    });
    closeContextMenu();
  }, [canvasOffset, closeContextMenu, zoom]);

  const changeZoom = useCallback((direction: 1 | -1) => {
    zoomTo(zoom + direction * ZOOM_STEP);
  }, [zoom, zoomTo]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return undefined;

    const handleNativeWheel = (event: globalThis.WheelEvent) => {
      if (!event.ctrlKey && !event.metaKey) return;
      event.preventDefault();
      event.stopPropagation();
      const bounds = canvas.getBoundingClientRect();
      const direction = event.deltaY > 0 ? -1 : 1;
      zoomTo(zoom + direction * ZOOM_STEP, {
        x: event.clientX - bounds.left,
        y: event.clientY - bounds.top,
      });
    };

    canvas.addEventListener('wheel', handleNativeWheel, { passive: false });
    return () => canvas.removeEventListener('wheel', handleNativeWheel);
  }, [zoom, zoomTo]);

  const selectNode = useCallback((nodeId: string) => {
    dispatchInteraction({ type: 'clear-selected-connector' });
    onSelectNode(nodeId);
  }, [onSelectNode]);

  const startCanvasPan = (event: PointerEvent<HTMLDivElement>) => {
    if (connectorDraftRef.current || reconnectDraftRef.current) return;
    if (event.button !== 0 || event.target !== event.currentTarget) return;
    event.preventDefault();
    closeContextMenu();
    focusCanvas();
    event.currentTarget.setPointerCapture(event.pointerId);
    selectNode('');
    panRef.current = {
      pointerId: event.pointerId,
      startClientX: event.clientX,
      startClientY: event.clientY,
      startOffset: canvasOffset,
    };
    setIsPanning(true);
  };

  const moveCanvasPan = (event: PointerEvent<HTMLDivElement>) => {
    const pan = panRef.current;
    if (!pan || pan.pointerId !== event.pointerId) return;
    setCanvasOffset({
      x: pan.startOffset.x + event.clientX - pan.startClientX,
      y: pan.startOffset.y + event.clientY - pan.startClientY,
    });
  };

  const updateHoveredNode = (event: PointerEvent<Element>) => {
    if (panRef.current || dragRef.current || connectorDraftRef.current || reconnectDraftRef.current) return;
    const point = canvasPointFromEvent(event);
    if (!point) return;
    const hitRect = hitTestNode(point, nodeRects, ADORNER_HIT_PADDING);
    if (hitRect) {
      dispatchInteraction({ type: 'hover-node', nodeId: hitRect.nodeId });
      return;
    }
    dispatchInteraction({ type: 'clear-hover-node' });
  };

  const handleCanvasPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    moveCanvasPan(event);
    updateHoveredNode(event);
  };

  const startConnectorDrag = (port: PortRef, event: PointerEvent<HTMLButtonElement>) => {
    if (connectorMutationPending) return;
    const pointer = canvasPointFromEvent(event);
    if (!pointer) return;

    focusCanvas();
    closeContextMenu();
    onSelectNode(port.nodeId);
    const draft: ConnectorDraft = {
      source: port,
      pointer,
      target: null,
      validTarget: false,
    };
    connectorDraftRef.current = draft;
    dispatchInteraction({ type: 'start-connector-drag', source: port, pointer });
  };

  const moveConnectorDrag = (event: PointerEvent<HTMLButtonElement>) => {
    const draft = connectorDraftRef.current;
    if (!draft) return;

    const nextDraft = nextConnectorDraftFromEvent(event, draft);
    if (!nextDraft) return;

    connectorDraftRef.current = nextDraft;
    dispatchInteraction({
      type: 'move-connector-drag',
      pointer: nextDraft.pointer,
      target: nextDraft.target,
      validTarget: nextDraft.validTarget,
      invalidReason: nextDraft.invalidReason,
    });
  };

  const finishConnectorDrag = (event: PointerEvent<HTMLButtonElement>) => {
    const draft = connectorDraftRef.current;
    if (!draft) return;

    const nextDraft = nextConnectorDraftFromEvent(event, draft) ?? draft;
    connectorDraftRef.current = null;
    dispatchInteraction({ type: 'finish-connector-drag' });

    if (!connectorMutationPending && nextDraft.target && nextDraft.validTarget) {
      onCreateConnector({ source: nextDraft.source, target: nextDraft.target });
    }
  };

  const cancelConnectorDrag = () => {
    connectorDraftRef.current = null;
    dispatchInteraction({ type: 'cancel-connector-drag' });
  };

  const nextConnectorDraftFromEvent = useCallback((event: PointerEvent<Element>, draft: ConnectorDraft): ConnectorDraft | null => {
    const pointer = canvasPointFromEvent(event);
    if (!pointer) return null;
    const target = connectorTargetFromPoint(draft.source, pointer, nodeRectsRef.current, edgeIdsByKeyRef.current);
    return {
      ...draft,
      pointer,
      target: target.port,
      validTarget: target.validTarget,
      invalidReason: target.invalidReason,
    };
  }, [canvasPointFromEvent]);

  const selectConnector = useCallback((connectorId: string) => {
    focusCanvas();
    closeContextMenu();
    onSelectNode('');
    dispatchInteraction({ type: 'select-connector', connectorId });
  }, [closeContextMenu, focusCanvas, onSelectNode]);

  const nextReconnectDraftFromEvent = useCallback((event: PointerEvent<Element>, draft: ConnectorReconnectDraft): ConnectorReconnectDraft | null => {
    const pointer = canvasPointFromEvent(event);
    if (!pointer) return null;
    const target = reconnectTargetFromPoint(draft, pointer, nodeRectsRef.current, edgeIdsByKeyRef.current);
    return {
      ...draft,
      pointer,
      target: target.port,
      validTarget: target.validTarget,
      invalidReason: target.invalidReason,
    };
  }, [canvasPointFromEvent]);

  const startConnectorReconnect = useCallback((
    connector: ConnectorModel,
    endpoint: ConnectorEndpointRole,
    event: PointerEvent<SVGCircleElement>,
  ) => {
    if (connectorMutationPending) return;
    const pointer = canvasPointFromEvent(event);
    if (!pointer) return;

    focusCanvas();
    closeContextMenu();
    const originalSource = connector.source.port;
    const originalTarget = connector.target.port;
    const fixed = endpoint === 'source' ? originalTarget : originalSource;
    const draft: ConnectorReconnectDraft = {
      connectorId: connector.id,
      endpoint,
      fixed,
      originalSource,
      originalTarget,
      pointer,
      target: null,
      validTarget: false,
    };
    reconnectDraftRef.current = draft;
    onSelectNode('');
    dispatchInteraction({
      type: 'start-connector-reconnect',
      connectorId: connector.id,
      endpoint,
      fixed,
      originalSource,
      originalTarget,
      pointer,
    });
  }, [canvasPointFromEvent, closeContextMenu, connectorMutationPending, focusCanvas, onSelectNode]);

  const moveConnectorReconnect = useCallback((event: PointerEvent<SVGCircleElement>) => {
    const draft = reconnectDraftRef.current;
    if (!draft) return;

    const nextDraft = nextReconnectDraftFromEvent(event, draft);
    if (!nextDraft) return;

    reconnectDraftRef.current = nextDraft;
    dispatchInteraction({
      type: 'move-connector-reconnect',
      pointer: nextDraft.pointer,
      target: nextDraft.target,
      validTarget: nextDraft.validTarget,
      invalidReason: nextDraft.invalidReason,
    });
  }, [nextReconnectDraftFromEvent]);

  const finishConnectorReconnect = useCallback((event: PointerEvent<SVGCircleElement>) => {
    const draft = reconnectDraftRef.current;
    if (!draft) return;

    const nextDraft = nextReconnectDraftFromEvent(event, draft) ?? draft;
    reconnectDraftRef.current = null;
    dispatchInteraction({ type: 'finish-connector-reconnect' });

    const reconnectInput = reconnectInputFromDraft(nextDraft);
    if (!connectorMutationPending && reconnectInput) {
      onReconnectConnector(reconnectInput);
    }
  }, [connectorMutationPending, nextReconnectDraftFromEvent, onReconnectConnector]);

  const cancelConnectorReconnect = useCallback(() => {
    reconnectDraftRef.current = null;
    dispatchInteraction({ type: 'cancel-connector-reconnect' });
  }, []);

  const finishCanvasPan = (event: PointerEvent<HTMLDivElement>) => {
    const pan = panRef.current;
    if (!pan || pan.pointerId !== event.pointerId) return;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    panRef.current = null;
    setIsPanning(false);
  };

  const handleNodePointerDown = useCallback((event: PointerEvent<HTMLDivElement>, node: PipelineNode, index: number) => {
    if (event.button !== 0) return;
    const point = canvasPointFromEvent(event);
    if (!point) return;
    event.preventDefault();
    event.stopPropagation();
    closeContextMenu();
    focusCanvas();
    dispatchInteraction({ type: 'clear-selected-connector' });
    const position = nodePosition(node, index, nodePositionsRef.current);
    settledNodeRectsRef.current = nodeRectsRef.current;
    setDraggingNodeId(node.id);
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = {
      nodeId: node.id,
      pointerId: event.pointerId,
      offsetX: point.x - position.x,
      offsetY: point.y - position.y,
      startClientX: event.clientX,
      startClientY: event.clientY,
      lastX: position.x,
      lastY: position.y,
      moved: false,
    };
  }, [canvasPointFromEvent, closeContextMenu, focusCanvas]);

  const handleNodePointerMove = useCallback((event: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    const point = canvasPointFromEvent(event);
    if (!drag || drag.pointerId !== event.pointerId || !point) return;
    const x = point.x - drag.offsetX;
    const y = point.y - drag.offsetY;
    const moved = drag.moved || Math.hypot(event.clientX - drag.startClientX, event.clientY - drag.startClientY) > 4;
    dragRef.current = { ...drag, lastX: x, lastY: y, moved };
    if (moved) {
      scheduleNodePositionUpdate(drag.nodeId, { x, y });
    }
  }, [canvasPointFromEvent, scheduleNodePositionUpdate]);

  const finishNodePointer = useCallback((event: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    event.preventDefault();
    event.stopPropagation();
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    dragRef.current = null;
    cancelScheduledNodePositionUpdate();
    setDraggingNodeId('');
    if (drag.moved) {
      const finalPosition = { x: drag.lastX, y: drag.lastY };
      applyNodePosition(drag.nodeId, finalPosition);
      onSaveNodePosition(drag.nodeId, finalPosition);
      dispatchInteraction({ type: 'hover-node', nodeId: drag.nodeId });
      return;
    }
    onSelectNode(drag.nodeId);
    dispatchInteraction({ type: 'hover-node', nodeId: drag.nodeId });
  }, [applyNodePosition, cancelScheduledNodePositionUpdate, onSaveNodePosition, onSelectNode]);

  const handleNodeContextMenu = useCallback((event: MouseEvent<HTMLDivElement>, node: PipelineNode) => {
    event.preventDefault();
    event.stopPropagation();
    const point = contextMenuPointFromEvent(event);
    if (!point) return;
    focusCanvas();
    dispatchInteraction({ type: 'clear-selected-connector' });
    onSelectNode(node.id);
    setContextMenu({ type: 'node', id: node.id, ...point });
  }, [contextMenuPointFromEvent, focusCanvas, onSelectNode]);

  const handleConnectorContextMenu = useCallback((connectorId: string, event: MouseEvent<SVGPathElement>) => {
    event.preventDefault();
    event.stopPropagation();
    const point = contextMenuPointFromEvent(event);
    if (!point) return;
    focusCanvas();
    onSelectNode('');
    dispatchInteraction({ type: 'select-connector', connectorId });
    setContextMenu({ type: 'connector', id: connectorId, ...point });
  }, [contextMenuPointFromEvent, focusCanvas, onSelectNode]);

  const handleContextMenuAction = useCallback(() => {
    if (!contextMenu) return;
    const { type, id } = contextMenu;
    closeContextMenu();
    if (type === 'node') {
      onDeleteNode(id);
      return;
    }
    if (!connectorMutationPending) onDeleteConnector(id);
  }, [closeContextMenu, connectorMutationPending, contextMenu, onDeleteConnector, onDeleteNode]);

  const handleCanvasContextMenu = useCallback((event: MouseEvent<HTMLDivElement>) => {
    event.preventDefault();
  }, []);

  const handleCanvasKeyDown = useCallback(
    (event: KeyboardEvent<HTMLDivElement>) => {
      if (isEditableKeyboardTarget(event.target)) return;

      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopPropagation();

        if (contextMenu) {
          closeContextMenu();
          return;
        }

        if (connectorDraftRef.current) {
          connectorDraftRef.current = null;
          dispatchInteraction({ type: 'cancel-connector-drag' });
          return;
        }

        if (reconnectDraftRef.current) {
          reconnectDraftRef.current = null;
          dispatchInteraction({ type: 'cancel-connector-reconnect' });
          return;
        }

        if (interactionRef.current.selectedConnectorId) {
          dispatchInteraction({ type: 'clear-selected-connector' });
          return;
        }

        if (selectedNodeId) onSelectNode('');
        return;
      }

      if (event.key !== 'Delete' && event.key !== 'Backspace') return;
      event.preventDefault();
      event.stopPropagation();

      const selectedConnectorId = interactionRef.current.selectedConnectorId;
      if (connectorMutationPending || connectorDraftRef.current || reconnectDraftRef.current || dragRef.current) return;
      if (selectedConnectorId) {
        closeContextMenu();
        onDeleteConnector(selectedConnectorId);
        return;
      }
      if (selectedNodeId) {
        closeContextMenu();
        onDeleteNode(selectedNodeId);
      }
    },
    [closeContextMenu, connectorMutationPending, contextMenu, onDeleteConnector, onDeleteNode, onSelectNode, selectedNodeId],
  );

  const handleCanvasKeyDownCapture = useCallback((event: KeyboardEvent<HTMLDivElement>) => {
    if (!event.ctrlKey && !event.metaKey) return;

    const key = event.key;
    if (key !== '+' && key !== '=' && key !== '-' && key !== '_' && key !== '0') return;

    event.preventDefault();
    event.stopPropagation();

    if (key === '0') {
      zoomTo(1);
      return;
    }

    changeZoom(key === '-' || key === '_' ? -1 : 1);
  }, [changeZoom, zoomTo]);

  return (
    <div
      ref={canvasRef}
      tabIndex={0}
      aria-label="Pipeline builder canvas"
      className={clsx('relative min-h-0 flex-1 touch-none overflow-hidden bg-slate-50 focus:outline-none', isPanning ? 'cursor-grabbing' : 'cursor-grab')}
      style={{
        backgroundImage: 'radial-gradient(circle at 1px 1px, #cbd5e1 1px, transparent 0)',
        backgroundPosition: `${scaledGridOffset(canvasOffset.x, zoom)} ${scaledGridOffset(canvasOffset.y, zoom)}`,
        backgroundSize: `${GRID_SIZE * zoom}px ${GRID_SIZE * zoom}px`,
      }}
      onPointerDown={startCanvasPan}
      onPointerMove={handleCanvasPointerMove}
      onPointerUp={finishCanvasPan}
      onPointerCancel={finishCanvasPan}
      onKeyDownCapture={handleCanvasKeyDownCapture}
      onKeyDown={handleCanvasKeyDown}
      onContextMenu={handleCanvasContextMenu}
      onPointerLeave={() => {
        if (!connectorDraftRef.current && !reconnectDraftRef.current) dispatchInteraction({ type: 'clear-hover-node' });
      }}
    >
      <div
        className="pointer-events-none absolute left-0 top-0 overflow-visible"
        style={{
          transform: `translate3d(${canvasOffset.x}px, ${canvasOffset.y}px, 0) scale(${zoom})`,
          transformOrigin: '0 0',
          willChange: isPanning ? 'transform' : undefined,
        }}
      >
        <ConnectorLayer
          connectors={connectors}
          nodeRects={routingNodeRects}
          liveNodeRects={nodeRects}
          draggingNodeId={draggingNodeId}
          selectedConnectorId={interaction.selectedConnectorId}
          reconnectDisabled={connectorMutationPending}
          onSelectConnector={selectConnector}
          onConnectorContextMenu={handleConnectorContextMenu}
          onReconnectEndpointStart={startConnectorReconnect}
          onReconnectEndpointMove={moveConnectorReconnect}
          onReconnectEndpointEnd={finishConnectorReconnect}
          onReconnectEndpointCancel={cancelConnectorReconnect}
        />
        <DragPreviewLayer
          source={previewSource}
          target={previewTarget}
          nodeRects={nodeRects}
          snapped={Boolean(activeTargetPort)}
          validTarget={Boolean(activeTargetValid)}
        />
        <NodeLayer
          nodes={nodes}
          positions={nodePositions}
          selectedNodeId={selectedNodeId}
          onSelectNode={selectNode}
          onToggleNodeDetails={onToggleNodeDetails}
          onNodePointerDown={handleNodePointerDown}
          onNodePointerMove={handleNodePointerMove}
          onNodePointerUp={finishNodePointer}
          onNodePointerCancel={finishNodePointer}
          onNodeContextMenu={handleNodeContextMenu}
          setNodeElementRef={setNodeElementRef}
        />
        <AdornerLayer
          rect={adornerRect}
          mode={adornerMode}
          activePort={connectorDraft?.source ?? null}
          connectorDragDisabled={connectorMutationPending}
          onConnectorDragStart={startConnectorDrag}
          onConnectorDragMove={moveConnectorDrag}
          onConnectorDragEnd={finishConnectorDrag}
          onConnectorDragCancel={cancelConnectorDrag}
        />
        <AdornerLayer
          rect={targetAdornerRect}
          mode={targetAdornerMode}
          activePort={activeTargetPort}
          interactive={false}
          onConnectorDragStart={startConnectorDrag}
          onConnectorDragMove={moveConnectorDrag}
          onConnectorDragEnd={finishConnectorDrag}
          onConnectorDragCancel={cancelConnectorDrag}
        />
      </div>
      {!nodes.length ? (
        <div className="pointer-events-none absolute inset-0 grid place-items-center p-6">
          <div className="w-[380px] max-w-full">
            <EmptyState title="Template chưa có node. Chọn node trong Thư viện node để bắt đầu." />
          </div>
        </div>
      ) : null}
      <div className="absolute right-4 top-4 z-40 inline-flex items-center overflow-hidden rounded-md border border-line bg-white shadow-sm">
        <button
          type="button"
          title="Thu nhỏ canvas"
          aria-label="Thu nhỏ canvas"
          className="grid h-9 w-9 place-items-center text-slate-600 hover:bg-slate-50 disabled:text-slate-300"
          disabled={zoom <= MIN_ZOOM}
          onClick={() => changeZoom(-1)}
        >
          <Minus className="h-4 w-4" />
        </button>
        <div className="min-w-[56px] border-x border-line px-2 text-center text-xs font-semibold text-slate-600">{Math.round(zoom * 100)}%</div>
        <button
          type="button"
          title="Phóng to canvas"
          aria-label="Phóng to canvas"
          className="grid h-9 w-9 place-items-center text-slate-600 hover:bg-slate-50 disabled:text-slate-300"
          disabled={zoom >= MAX_ZOOM}
          onClick={() => changeZoom(1)}
        >
          <Plus className="h-4 w-4" />
        </button>
      </div>
      {contextMenu ? (
        <div
          className="absolute z-50 overflow-hidden rounded-md border border-line bg-white py-1 shadow-panel"
          style={{ left: contextMenu.x, top: contextMenu.y, width: CONTEXT_MENU_WIDTH }}
          onPointerDown={(event) => event.stopPropagation()}
          onContextMenu={(event) => event.preventDefault()}
        >
          <button
            type="button"
            className="block w-full px-3 py-2 text-left text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-50"
            disabled={contextMenu.type === 'connector' && connectorMutationPending}
            onClick={handleContextMenuAction}
          >
            {contextMenu.type === 'node' ? 'Xoá node' : 'Xoá connector'}
          </button>
        </div>
      ) : null}
    </div>
  );
}

function sameSize(left: NodeSize | undefined, right: NodeSize | undefined) {
  return left?.width === right?.width && left?.height === right?.height;
}

function scaledGridOffset(value: number, scale: number) {
  const gridSize = GRID_SIZE * scale;
  return `${((value % gridSize) + gridSize) % gridSize}px`;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

function roundZoom(value: number) {
  return Math.round(value * 100) / 100;
}

function endpointFromPort(port: PortRef, nodeRects: DiagramNodeRect[]): ConnectorEndpoint | null {
  const rect = nodeRects.find((candidate) => candidate.nodeId === port.nodeId);
  return rect ? visibleConnectorEndpoint(rect, port.side) : null;
}

function previewTargetEndpoint(draft: ConnectorDraft, nodeRects: DiagramNodeRect[]): ConnectorEndpoint {
  const targetEndpoint = draft.target ? endpointFromPort(draft.target, nodeRects) : null;
  if (targetEndpoint) return targetEndpoint;

  return {
    port: {
      nodeId: '__pointer__',
      side: oppositePortSide(draft.source.side),
    },
    point: draft.pointer,
  };
}

function reconnectPreviewSource(draft: ConnectorReconnectDraft, nodeRects: DiagramNodeRect[]): ConnectorEndpoint | null {
  if (draft.endpoint === 'target') return endpointFromPort(draft.fixed, nodeRects);
  const targetEndpoint = draft.target ? endpointFromPort(draft.target, nodeRects) : null;
  return (
    targetEndpoint ?? {
      port: {
        nodeId: '__pointer__',
        side: oppositePortSide(draft.fixed.side),
      },
      point: draft.pointer,
    }
  );
}

function reconnectPreviewTarget(draft: ConnectorReconnectDraft, nodeRects: DiagramNodeRect[]): ConnectorEndpoint | null {
  if (draft.endpoint === 'source') return endpointFromPort(draft.fixed, nodeRects);
  const targetEndpoint = draft.target ? endpointFromPort(draft.target, nodeRects) : null;
  return (
    targetEndpoint ?? {
      port: {
        nodeId: '__pointer__',
        side: oppositePortSide(draft.fixed.side),
      },
      point: draft.pointer,
    }
  );
}

function connectorTargetFromPoint(
  source: PortRef,
  pointer: NodePosition,
  nodeRects: DiagramNodeRect[],
  edgeIdsByKey: Map<string, Set<string>>,
): { port: PortRef | null; validTarget: boolean; invalidReason?: ConnectorInvalidReason } {
  const hit = nearestPortHit(pointer, nodeRects, CONNECTOR_SNAP_RADIUS);
  if (!hit) return { port: null, validTarget: false };

  return validateConnectorTarget(source, hit.port, hit.port, edgeIdsByKey);
}

function reconnectTargetFromPoint(
  draft: ConnectorReconnectDraft,
  pointer: NodePosition,
  nodeRects: DiagramNodeRect[],
  edgeIdsByKey: Map<string, Set<string>>,
): { port: PortRef | null; validTarget: boolean; invalidReason?: ConnectorInvalidReason } {
  const hit = nearestPortHit(pointer, nodeRects, CONNECTOR_SNAP_RADIUS);
  if (!hit) return { port: null, validTarget: false };

  const source = draft.endpoint === 'source' ? hit.port : draft.fixed;
  const target = draft.endpoint === 'target' ? hit.port : draft.fixed;
  return validateConnectorTarget(source, target, hit.port, edgeIdsByKey, draft.connectorId);
}

function validateConnectorTarget(
  source: PortRef,
  target: PortRef,
  candidatePort: PortRef,
  edgeIdsByKey: Map<string, Set<string>>,
  excludedConnectorId = '',
): { port: PortRef | null; validTarget: boolean; invalidReason?: ConnectorInvalidReason } {
  if (source.nodeId === target.nodeId) {
    return { port: candidatePort, validTarget: false, invalidReason: 'self' };
  }

  if (hasDuplicateConnector(source.nodeId, target.nodeId, edgeIdsByKey, excludedConnectorId)) {
    return { port: candidatePort, validTarget: false, invalidReason: 'duplicate' };
  }

  return { port: candidatePort, validTarget: true };
}

function hasDuplicateConnector(
  sourceNodeId: string,
  targetNodeId: string,
  edgeIdsByKey: Map<string, Set<string>>,
  excludedConnectorId = '',
) {
  const connectorIds = edgeIdsByKey.get(templateEdgeKey(sourceNodeId, targetNodeId));
  if (!connectorIds) return false;
  return Array.from(connectorIds).some((connectorId) => connectorId !== excludedConnectorId);
}

function reconnectInputFromDraft(draft: ConnectorReconnectDraft): ConnectorReconnectInput | null {
  if (!draft.target || !draft.validTarget) return null;

  const source = draft.endpoint === 'source' ? draft.target : draft.fixed;
  const target = draft.endpoint === 'target' ? draft.target : draft.fixed;
  if (samePort(source, draft.originalSource) && samePort(target, draft.originalTarget)) return null;

  return {
    connectorId: draft.connectorId,
    source,
    target,
  };
}

function samePort(left: PortRef, right: PortRef) {
  return left.nodeId === right.nodeId && left.side === right.side;
}

function isEditableKeyboardTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  const tagName = target.tagName.toLowerCase();
  return target.isContentEditable || tagName === 'input' || tagName === 'textarea' || tagName === 'select';
}

function oppositePortSide(side: PortSide): PortSide {
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
