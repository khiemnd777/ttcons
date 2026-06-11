import type { ConnectorEndpoint, DiagramNodeRect } from './diagramTypes';
import { connectorPath } from './router';
import { routePreviewConnector } from './routingEngine';

const PREVIEW_MARKER_ID = 'pipeline-builder-drag-preview-arrow';

type DragPreviewLayerProps = {
  source: ConnectorEndpoint | null;
  target: ConnectorEndpoint | null;
  nodeRects: DiagramNodeRect[];
  snapped: boolean;
  validTarget: boolean;
};

export function DragPreviewLayer({ source, target, nodeRects, snapped, validTarget }: DragPreviewLayerProps) {
  if (!source || !target) return null;

  const route = routePreviewConnector(source, target, nodeRects);
  const path = connectorPath(route);
  if (!path) return null;

  const stroke = snapped && validTarget ? '#2563eb' : snapped ? '#f43f5e' : '#64748b';
  const halo = snapped && validTarget ? '#dbeafe' : snapped ? '#ffe4e6' : '#e2e8f0';

  return (
    <svg className="pointer-events-none absolute left-0 top-0 z-[5] overflow-visible" width={1} height={1} aria-hidden="true">
      <defs>
        <marker id={PREVIEW_MARKER_ID} viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto">
          <path d="M 0 0 L 10 5 L 0 10 z" fill={stroke} />
        </marker>
      </defs>
      <path d={path} fill="none" stroke={halo} strokeWidth={7} strokeLinecap="round" strokeLinejoin="round" />
      <path
        d={path}
        fill="none"
        stroke={stroke}
        strokeWidth={2.5}
        strokeDasharray={snapped && validTarget ? undefined : '7 6'}
        strokeLinecap="round"
        strokeLinejoin="round"
        markerEnd={`url(#${PREVIEW_MARKER_ID})`}
      />
    </svg>
  );
}
