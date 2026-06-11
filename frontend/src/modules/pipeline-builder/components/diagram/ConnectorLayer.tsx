import type { PointerEvent } from 'react';
import clsx from 'clsx';
import type { ConnectorModel } from './diagramTypes';
import type { ConnectorEndpointRole } from './diagramTypes';
import { connectorPath, routeConnector } from './router';

const ARROW_MARKER_ID = 'pipeline-builder-connector-arrow';
const SELECTED_ARROW_MARKER_ID = 'pipeline-builder-connector-arrow-selected';

type ConnectorLayerProps = {
  connectors: ConnectorModel[];
  selectedConnectorId: string;
  reconnectDisabled: boolean;
  onSelectConnector: (connectorId: string) => void;
  onReconnectEndpointStart: (connector: ConnectorModel, endpoint: ConnectorEndpointRole, event: PointerEvent<SVGCircleElement>) => void;
  onReconnectEndpointMove: (event: PointerEvent<SVGCircleElement>) => void;
  onReconnectEndpointEnd: (event: PointerEvent<SVGCircleElement>) => void;
  onReconnectEndpointCancel: (event: PointerEvent<SVGCircleElement>) => void;
};

export function ConnectorLayer({
  connectors,
  selectedConnectorId,
  reconnectDisabled,
  onSelectConnector,
  onReconnectEndpointStart,
  onReconnectEndpointMove,
  onReconnectEndpointEnd,
  onReconnectEndpointCancel,
}: ConnectorLayerProps) {
  if (!connectors.length) return null;

  const selectedConnector = connectors.find((connector) => connector.id === selectedConnectorId) ?? null;

  return (
    <>
      <svg className="pointer-events-none absolute left-0 top-0 z-0 overflow-visible" width={1} height={1} aria-hidden="true">
        <defs>
          <marker id={ARROW_MARKER_ID} viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#64748b" />
          </marker>
          <marker id={SELECTED_ARROW_MARKER_ID} viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="#2563eb" />
          </marker>
        </defs>
        {connectors.map((connector) => {
          const route = connector.route ?? routeConnector(connector.source, connector.target);
          const path = connectorPath(route);
          if (!path) return null;

          const selected = connector.id === selectedConnectorId;

          return (
            <g key={connector.id}>
              <path
                d={path}
                fill="none"
                stroke={selected ? '#bfdbfe' : '#e2e8f0'}
                strokeWidth={selected ? 8 : 6}
                strokeLinecap="round"
                strokeLinejoin="round"
              />
              <path
                d={path}
                fill="none"
                stroke="transparent"
                strokeWidth={16}
                strokeLinecap="round"
                strokeLinejoin="round"
                className="pointer-events-auto cursor-pointer"
                onPointerDown={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                }}
                onClick={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                  onSelectConnector(connector.id);
                }}
              />
              <path
                d={path}
                fill="none"
                stroke={selected ? '#2563eb' : '#64748b'}
                strokeWidth={selected ? 2.75 : 2}
                strokeLinecap="round"
                strokeLinejoin="round"
                markerEnd={`url(#${selected ? SELECTED_ARROW_MARKER_ID : ARROW_MARKER_ID})`}
              />
              {connector.label ? <ConnectorLabel connector={connector} route={route} selected={selected} /> : null}
            </g>
          );
        })}
      </svg>
      {selectedConnector ? (
        <svg className="pointer-events-none absolute left-0 top-0 z-30 overflow-visible" width={1} height={1} aria-hidden="true">
          <EndpointHandle
            connector={selectedConnector}
            endpoint="source"
            disabled={reconnectDisabled}
            onReconnectEndpointStart={onReconnectEndpointStart}
            onReconnectEndpointMove={onReconnectEndpointMove}
            onReconnectEndpointEnd={onReconnectEndpointEnd}
            onReconnectEndpointCancel={onReconnectEndpointCancel}
          />
          <EndpointHandle
            connector={selectedConnector}
            endpoint="target"
            disabled={reconnectDisabled}
            onReconnectEndpointStart={onReconnectEndpointStart}
            onReconnectEndpointMove={onReconnectEndpointMove}
            onReconnectEndpointEnd={onReconnectEndpointEnd}
            onReconnectEndpointCancel={onReconnectEndpointCancel}
          />
        </svg>
      ) : null}
    </>
  );
}

function ConnectorLabel({ connector, route, selected }: { connector: ConnectorModel; route: { x: number; y: number }[]; selected: boolean }) {
  const point = route[Math.floor(route.length / 2)];
  if (!point) return null;

  return (
    <text
      x={point.x}
      y={point.y - 8}
      className={clsx('text-[11px] font-semibold', selected ? 'fill-blue-700' : 'fill-slate-500')}
      textAnchor="middle"
      dominantBaseline="central"
      paintOrder="stroke"
      stroke="#f8fafc"
      strokeWidth={4}
      strokeLinejoin="round"
    >
      {connector.label}
    </text>
  );
}

function EndpointHandle({
  connector,
  endpoint,
  disabled,
  onReconnectEndpointStart,
  onReconnectEndpointMove,
  onReconnectEndpointEnd,
  onReconnectEndpointCancel,
}: {
  connector: ConnectorModel;
  endpoint: ConnectorEndpointRole;
  disabled: boolean;
  onReconnectEndpointStart: (connector: ConnectorModel, endpoint: ConnectorEndpointRole, event: PointerEvent<SVGCircleElement>) => void;
  onReconnectEndpointMove: (event: PointerEvent<SVGCircleElement>) => void;
  onReconnectEndpointEnd: (event: PointerEvent<SVGCircleElement>) => void;
  onReconnectEndpointCancel: (event: PointerEvent<SVGCircleElement>) => void;
}) {
  const model = endpoint === 'source' ? connector.source : connector.target;
  if (!model.point) return null;

  return (
    <g>
      <circle cx={model.point.x} cy={model.point.y} r={8} fill="#ffffff" stroke="#2563eb" strokeWidth={2.5} />
      <circle
        cx={model.point.x}
        cy={model.point.y}
        r={16}
        fill="transparent"
        className={clsx('pointer-events-auto', disabled ? 'cursor-not-allowed' : 'cursor-crosshair')}
        onPointerDown={(event) => {
          if (disabled) return;
          event.preventDefault();
          event.stopPropagation();
          event.currentTarget.setPointerCapture(event.pointerId);
          onReconnectEndpointStart(connector, endpoint, event);
        }}
        onPointerMove={(event) => {
          if (disabled) return;
          event.preventDefault();
          event.stopPropagation();
          onReconnectEndpointMove(event);
        }}
        onPointerUp={(event) => {
          if (disabled) return;
          event.preventDefault();
          event.stopPropagation();
          if (event.currentTarget.hasPointerCapture(event.pointerId)) {
            event.currentTarget.releasePointerCapture(event.pointerId);
          }
          onReconnectEndpointEnd(event);
        }}
        onPointerCancel={(event) => {
          if (disabled) return;
          event.preventDefault();
          event.stopPropagation();
          if (event.currentTarget.hasPointerCapture(event.pointerId)) {
            event.currentTarget.releasePointerCapture(event.pointerId);
          }
          onReconnectEndpointCancel(event);
        }}
      />
    </g>
  );
}
