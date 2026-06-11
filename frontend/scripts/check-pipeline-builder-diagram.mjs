import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');

const files = {
  page: 'src/modules/pipeline-builder/PipelineBuilderPage.tsx',
  canvas: 'src/modules/pipeline-builder/components/diagram/DiagramCanvas.tsx',
  connectorLayer: 'src/modules/pipeline-builder/components/diagram/ConnectorLayer.tsx',
  adornerLayer: 'src/modules/pipeline-builder/components/diagram/AdornerLayer.tsx',
  dragPreviewLayer: 'src/modules/pipeline-builder/components/diagram/DragPreviewLayer.tsx',
  nodeLayer: 'src/modules/pipeline-builder/components/diagram/NodeLayer.tsx',
  nodeCard: 'src/modules/pipeline-builder/components/diagram/NodeCard.tsx',
  geometry: 'src/modules/pipeline-builder/components/diagram/geometry.ts',
  hitTesting: 'src/modules/pipeline-builder/components/diagram/hitTesting.ts',
  router: 'src/modules/pipeline-builder/components/diagram/router.ts',
  routingEngine: 'src/modules/pipeline-builder/components/diagram/routingEngine.ts',
  routingLayoutHook: 'src/modules/pipeline-builder/components/diagram/useConnectorRoutingLayout.ts',
  interactionReducer: 'src/modules/pipeline-builder/components/diagram/interactionReducer.ts',
};

const source = Object.fromEntries(Object.entries(files).map(([key, file]) => [key, read(file)]));
const failures = [];

check('PipelineBuilderPage composes DiagramCanvas', () => {
  expectIncludes(source.page, "import { DiagramCanvas }", files.page);
  expectIncludes(source.page, '<DiagramCanvas', files.page);
});

check('PipelineBuilderPage does not own diagram layers or geometry', () => {
  const forbiddenImports = [
    'components/diagram/AdornerLayer',
    'components/diagram/ConnectorLayer',
    'components/diagram/DragPreviewLayer',
    'components/diagram/NodeLayer',
    'components/diagram/NodeCard',
    'components/diagram/geometry',
    'components/diagram/hitTesting',
    'components/diagram/router',
    'components/diagram/routingEngine',
    'components/diagram/useConnectorRoutingLayout',
    'components/diagram/interactionReducer',
  ];

  forbiddenImports.forEach((pattern) => expectNotIncludes(source.page, pattern, files.page));
  ['<AdornerLayer', '<ConnectorLayer', '<DragPreviewLayer', '<NodeLayer', '<NodeCard'].forEach((pattern) =>
    expectNotIncludes(source.page, pattern, files.page),
  );
});

check('DiagramCanvas keeps explicit layer composition', () => {
  ['AdornerLayer', 'ConnectorLayer', 'DragPreviewLayer', 'NodeLayer'].forEach((name) => {
    expectIncludes(source.canvas, `import { ${name} }`, files.canvas);
    expectIncludes(source.canvas, `<${name}`, files.canvas);
  });
});

check('DiagramCanvas keeps editor command handling explicit', () => {
  expectIncludes(source.canvas, 'handleCanvasKeyDown', files.canvas);
  expectIncludes(source.canvas, "event.key === 'Escape'", files.canvas);
  expectIncludes(source.canvas, "event.key !== 'Delete' && event.key !== 'Backspace'", files.canvas);
  expectIncludes(source.canvas, 'onDeleteConnector(selectedConnectorId)', files.canvas);
  expectIncludes(source.canvas, 'connectorMutationPending', files.canvas);
});

check('ConnectorLayer delegates route layout to routing hook', () => {
  expectIncludes(source.connectorLayer, 'useConnectorRoutingLayout', files.connectorLayer);
  expectIncludes(source.connectorLayer, 'EndpointHandle', files.connectorLayer);
  expectNotIncludes(source.connectorLayer, 'routeConnectors', files.connectorLayer);
  expectNotIncludes(source.connectorLayer, 'routeConnector', files.connectorLayer);
});

check('Routing hook preserves cached base layout plus live incident overlay', () => {
  expectIncludes(source.routingLayoutHook, 'cacheRef', files.routingLayoutHook);
  expectIncludes(source.routingLayoutHook, 'connectorTouchesNode', files.routingLayoutHook);
  expectIncludes(source.routingLayoutHook, 'routeConnectors({ connectors: incidentConnectors', files.routingLayoutHook);
});

check('Router remains only SVG path serialization', () => {
  expectIncludes(source.router, 'export function connectorPath', files.router);
  expectNotIncludes(source.router, 'routeConnector', files.router);
  expectNotIncludes(source.router, 'PORT_EXIT_DISTANCE', files.router);
  expectNotIncludes(source.router, 'routeIntersectsRect', files.router);
});

check('Pure geometry helpers remain available', () => {
  [
    'export function nodeRectsFromNodes',
    'export function portPoint',
    'export function visibleConnectorEndpoint',
    'export function preferredPortPair',
    'export function nearestPort',
    'export function expandNodeRect',
    'export function routeIntersectsRect',
  ].forEach((pattern) => expectIncludes(source.geometry, pattern, files.geometry));
});

check('Pure hit-testing helpers remain available', () => {
  ['export function pointInRect', 'export function hitTestNode', 'export function nearestPortHit'].forEach((pattern) =>
    expectIncludes(source.hitTesting, pattern, files.hitTesting),
  );
});

check('Routing engine remains separated from rendering', () => {
  expectIncludes(source.routingEngine, 'export function routeConnectors', files.routingEngine);
  expectIncludes(source.routingEngine, 'export function routePreviewConnector', files.routingEngine);
  expectIncludes(source.routingEngine, 'function findOrthogonalPath', files.routingEngine);
  expectNotIncludes(source.routingEngine, 'from "react"', files.routingEngine);
  expectNotIncludes(source.routingEngine, "from 'react'", files.routingEngine);
});

check('Interaction reducer keeps explicit connector states', () => {
  [
    "'start-connector-drag'",
    "'cancel-connector-drag'",
    "'start-connector-reconnect'",
    "'cancel-connector-reconnect'",
    "'select-connector'",
    "'clear-selected-connector'",
  ].forEach((pattern) => expectIncludes(source.interactionReducer, pattern, files.interactionReducer));
});

if (failures.length) {
  console.error('Pipeline Builder diagram regression check failed:');
  failures.forEach((failure) => console.error(`- ${failure}`));
  process.exit(1);
}

console.log('Pipeline Builder diagram regression check passed.');

function check(name, fn) {
  try {
    fn();
  } catch (error) {
    failures.push(`${name}: ${error.message}`);
  }
}

function read(file) {
  return readFileSync(resolve(root, file), 'utf8');
}

function expectIncludes(content, pattern, file) {
  if (!content.includes(pattern)) {
    throw new Error(`${file} must include ${JSON.stringify(pattern)}`);
  }
}

function expectNotIncludes(content, pattern, file) {
  if (content.includes(pattern)) {
    throw new Error(`${file} must not include ${JSON.stringify(pattern)}`);
  }
}
