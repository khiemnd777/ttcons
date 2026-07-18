import type { ConnectorEndpointRole, NodePosition, PortRef } from './diagramTypes';

export type ConnectorInvalidReason = 'self' | 'duplicate' | 'locked-node';

export type ConnectorDraft = {
  source: PortRef;
  pointer: NodePosition;
  target: PortRef | null;
  validTarget: boolean;
  invalidReason?: ConnectorInvalidReason;
};

export type ConnectorReconnectDraft = {
  connectorId: string;
  endpoint: ConnectorEndpointRole;
  fixed: PortRef;
  originalSource: PortRef;
  originalTarget: PortRef;
  pointer: NodePosition;
  target: PortRef | null;
  validTarget: boolean;
  invalidReason?: ConnectorInvalidReason;
};

export type DiagramInteractionState = {
  hoveredNodeId: string;
  selectedConnectorId: string;
  connectorDraft: ConnectorDraft | null;
  reconnectDraft: ConnectorReconnectDraft | null;
};

export type DiagramInteractionAction =
  | { type: 'hover-node'; nodeId: string }
  | { type: 'clear-hover-node'; nodeId?: string }
  | { type: 'select-connector'; connectorId: string }
  | { type: 'clear-selected-connector'; connectorId?: string }
  | { type: 'start-connector-drag'; source: PortRef; pointer: NodePosition }
  | {
      type: 'move-connector-drag';
      pointer: NodePosition;
      target: PortRef | null;
      validTarget: boolean;
      invalidReason?: ConnectorInvalidReason;
    }
  | { type: 'finish-connector-drag' }
  | { type: 'cancel-connector-drag' }
  | {
      type: 'start-connector-reconnect';
      connectorId: string;
      endpoint: ConnectorEndpointRole;
      fixed: PortRef;
      originalSource: PortRef;
      originalTarget: PortRef;
      pointer: NodePosition;
    }
  | {
      type: 'move-connector-reconnect';
      pointer: NodePosition;
      target: PortRef | null;
      validTarget: boolean;
      invalidReason?: ConnectorInvalidReason;
    }
  | { type: 'finish-connector-reconnect' }
  | { type: 'cancel-connector-reconnect' };

export const initialDiagramInteractionState: DiagramInteractionState = {
  hoveredNodeId: '',
  selectedConnectorId: '',
  connectorDraft: null,
  reconnectDraft: null,
};

export function diagramInteractionReducer(state: DiagramInteractionState, action: DiagramInteractionAction): DiagramInteractionState {
  switch (action.type) {
    case 'hover-node':
      return state.hoveredNodeId === action.nodeId ? state : { ...state, hoveredNodeId: action.nodeId };
    case 'clear-hover-node':
      if (action.nodeId && state.hoveredNodeId !== action.nodeId) return state;
      return state.hoveredNodeId ? { ...state, hoveredNodeId: '' } : state;
    case 'select-connector':
      return state.selectedConnectorId === action.connectorId
        ? state
        : { ...state, hoveredNodeId: '', selectedConnectorId: action.connectorId };
    case 'clear-selected-connector':
      if (action.connectorId && state.selectedConnectorId !== action.connectorId) return state;
      return state.selectedConnectorId ? { ...state, selectedConnectorId: '' } : state;
    case 'start-connector-drag':
      return {
        ...state,
        selectedConnectorId: '',
        connectorDraft: {
          source: action.source,
          pointer: action.pointer,
          target: null,
          validTarget: false,
        },
      };
    case 'move-connector-drag':
      if (!state.connectorDraft) return state;
      return {
        ...state,
        connectorDraft: {
          ...state.connectorDraft,
          pointer: action.pointer,
          target: action.target,
          validTarget: action.validTarget,
          invalidReason: action.invalidReason,
        },
      };
    case 'finish-connector-drag':
    case 'cancel-connector-drag':
      return state.connectorDraft ? { ...state, connectorDraft: null } : state;
    case 'start-connector-reconnect':
      return {
        ...state,
        hoveredNodeId: '',
        selectedConnectorId: action.connectorId,
        reconnectDraft: {
          connectorId: action.connectorId,
          endpoint: action.endpoint,
          fixed: action.fixed,
          originalSource: action.originalSource,
          originalTarget: action.originalTarget,
          pointer: action.pointer,
          target: null,
          validTarget: false,
        },
      };
    case 'move-connector-reconnect':
      if (!state.reconnectDraft) return state;
      return {
        ...state,
        reconnectDraft: {
          ...state.reconnectDraft,
          pointer: action.pointer,
          target: action.target,
          validTarget: action.validTarget,
          invalidReason: action.invalidReason,
        },
      };
    case 'finish-connector-reconnect':
    case 'cancel-connector-reconnect':
      return state.reconnectDraft ? { ...state, reconnectDraft: null } : state;
  }
}
