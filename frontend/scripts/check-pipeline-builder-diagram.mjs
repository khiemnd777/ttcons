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
  nodeInspector: 'src/modules/pipeline-builder/components/NodeInspectorPanel.tsx',
  edgeInspector: 'src/modules/pipeline-builder/components/EdgeInspectorPanel.tsx',
  shell: 'src/modules/pipeline-builder/components/PipelineBuilderShell.tsx',
  edgePersistence: 'src/modules/pipeline-builder/components/diagram/edgePersistence.ts',
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

check('Pipeline Builder exports the selected revision as PDF', () => {
  expectIncludes(source.page, 'downloadBlob', files.page);
  expectIncludes(source.page, '/pipeline-templates/${selectedTemplateId}/pdf', files.page);
  expectIncludes(source.page, 'pipelineTemplatePDFDownloadName', files.page);
  expectIncludes(source.page, 'exportPDFPending={exportPDF.isPending}', files.page);
  expectIncludes(source.shell, 'Xuất PDF', files.shell);
  expectIncludes(source.shell, 'Đang xuất...', files.shell);
  expectIncludes(source.shell, 'disabled={!canExportPDF || exportPDFPending}', files.shell);
});

check('Published templates expose a read-only node inspector', () => {
  expectIncludes(source.page, 'detailsNode ? (', files.page);
  expectIncludes(source.page, 'readOnly={isReadOnly}', files.page);
  expectIncludes(source.page, "mode: 'view'", files.page);
  expectIncludes(source.nodeInspector, 'readOnly: boolean', files.nodeInspector);
  expectIncludes(source.nodeInspector, 'Đã phát hành · Chỉ xem', files.nodeInspector);
  expectIncludes(source.nodeInspector, 'onOpenViewStep', files.nodeInspector);
  expectIncludes(source.nodeInspector, "stepDialogMode === 'view'", files.nodeInspector);
  expectIncludes(source.nodeInspector, "open={Boolean(stepDialogMode) && (!readOnly || stepDialogMode === 'view')}", files.nodeInspector);
  expectIncludes(source.nodeInspector, 'disabled={stepDialogReadOnly}', files.nodeInspector);
  expectIncludes(source.nodeInspector, 'Nội dung bên dưới chỉ để xem.', files.nodeInspector);
});

check('Published templates expose a read-only connector inspector', () => {
  expectIncludes(source.page, ') : selectedConnector ? (', files.page);
  expectNotIncludes(source.page, ') : !isReadOnly && selectedConnector ? (', files.page);
  expectIncludes(source.page, '<EdgeInspectorPanel', files.page);
  expectIncludes(source.page, 'readOnly={isReadOnly}', files.page);
  expectIncludes(source.page, 'if (!isReadOnly) submitEdgeSettings();', files.page);
  expectIncludes(source.edgeInspector, 'readOnly: boolean', files.edgeInspector);
  expectIncludes(source.edgeInspector, 'Đã phát hành · Chỉ xem', files.edgeInspector);
  expectIncludes(source.edgeInspector, '!readOnly ? (', files.edgeInspector);
  expectIncludes(source.edgeInspector, 'disabled={readOnly}', files.edgeInspector);
  expectIncludes(source.edgeInspector, 'disabled={readOnly || conditionsLoading || conditionLoadError}', files.edgeInspector);
});

check('Published templates keep presentation-only layout editing', () => {
  expectIncludes(source.page, "const diagramEditingMode: DiagramEditingMode = canEditStructure ? 'full' : canEditLayout ? 'layout' : 'view'", files.page);
  expectIncludes(source.page, '/pipeline-templates/${templateId}/layout', files.page);
  expectIncludes(source.page, 'buildUpdateEdgeLayoutPayload(input)', files.page);
  expectIncludes(source.shell, 'isPublished && canEditLayout', files.shell);
  expectIncludes(source.shell, 'đổi cạnh bám của connector trên chính node hiện tại', files.shell);
  expectIncludes(source.canvas, "editingMode === 'layout'", files.canvas);
  expectIncludes(source.canvas, "invalidReason: 'locked-node'", files.canvas);
  expectIncludes(source.canvas, "editingMode !== 'full'", files.canvas);

  const layoutPayloadStart = source.edgePersistence.indexOf('export function buildUpdateEdgeLayoutPayload');
  const connectorModelsStart = source.edgePersistence.indexOf('export function buildConnectorModels', layoutPayloadStart);
  if (layoutPayloadStart < 0 || connectorModelsStart < 0) {
    throw new Error(`${files.edgePersistence} must keep a dedicated layout payload builder`);
  }
  const layoutPayload = source.edgePersistence.slice(layoutPayloadStart, connectorModelsStart);
  expectIncludes(layoutPayload, 'source_side', files.edgePersistence);
  expectIncludes(layoutPayload, 'target_side', files.edgePersistence);
  expectNotIncludes(layoutPayload, 'from_node_id', files.edgePersistence);
  expectNotIncludes(layoutPayload, 'to_node_id', files.edgePersistence);

  const autoLayoutStart = source.page.indexOf('const autoLayout = useMutation');
  const selectTemplateStart = source.page.indexOf('const selectTemplate', autoLayoutStart);
  if (autoLayoutStart < 0 || selectTemplateStart < 0) {
    throw new Error(`${files.page} must keep the auto-layout mutation isolated`);
  }
  const autoLayoutMutation = source.page.slice(autoLayoutStart, selectTemplateStart);
  expectIncludes(autoLayoutMutation, '/pipeline-templates/${templateId}/layout', files.page);
  expectNotIncludes(autoLayoutMutation, 'order_index', files.page);
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
