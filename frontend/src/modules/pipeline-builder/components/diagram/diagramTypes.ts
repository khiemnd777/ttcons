export type NodePosition = { x: number; y: number };

export type NodeSize = { width: number; height: number };

export type DiagramNodeRect = {
  nodeId: string;
  x: number;
  y: number;
  width: number;
  height: number;
};

export type PortSide = 'top' | 'right' | 'bottom' | 'left';

export type PortRef = {
  nodeId: string;
  side: PortSide;
};

export type DiagramPortMode = 'auto' | 'manual';

export type DiagramEditingMode = 'full' | 'layout' | 'view';

export type ConnectorEndpoint = {
  port: PortRef;
  point?: NodePosition;
};

export type ConnectorEndpointRole = 'source' | 'target';

export type ConnectorModel = {
  id: string;
  source: ConnectorEndpoint;
  target: ConnectorEndpoint;
  route?: NodePosition[];
  label?: string;
  portMode?: DiagramPortMode;
  selected?: boolean;
};

export type DragState = {
  nodeId: string;
  pointerId: number;
  offsetX: number;
  offsetY: number;
  startClientX: number;
  startClientY: number;
  lastX: number;
  lastY: number;
  moved: boolean;
};

export type PanState = {
  pointerId: number;
  startClientX: number;
  startClientY: number;
  startOffset: NodePosition;
};
