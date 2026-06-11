# Current States

## Pipeline Builder Connector/Adorner Rebuild Plan (Closed)

Status: closed on 2026-06-11. All planned slices through Slice 10 have been implemented and validated. This section is retained as historical reference only and should not be treated as an active implementation plan or a general repository rule.

### Intent

The connector and adorner system in Pipeline Builder must be rebuilt cleanly, not patched on top of the previous anchor/connector attempts. `PipelineBuilderPage.tsx` should not become a large mixed file that owns layout, API calls, canvas geometry, connector routing, pointer state machines, and SVG rendering all at once.

### Shared Output Target

- `PipelineBuilderPage.tsx` is only an orchestrator for template loading, mutations, and page-level composition.
- Canvas has clear layers: grid/world transform, connector layer, node layer, adorner/interaction layer.
- Normal node cards do not show permanent anchors or connector buttons.
- Hover/select node shows a diagrams.net-like adorner: outline, subtle port handles around the boundary, and directional connector affordances outside the node.
- Anchor/port is a logical point on the node boundary, not a button inside the node.
- Connector is an independent object with source endpoint, target endpoint, route, arrowhead, and selected state.
- Dragging from an adorner creates a preview connector, snaps to a valid target port, and cancels cleanly on invalid drop.
- Selecting a connector shows endpoint handles; dragging an endpoint reconnects it to another node/port.
- Moving nodes reroutes connectors in realtime while keeping start/end points on the correct node boundary.
- Routing should be orthogonal/manhattan, deterministic, avoid node bounds in common cases, and separate parallel connectors.
- Verification must not use Browser, Browser headless, Playwright, or similar visual automation unless the user explicitly requests it. Use build/unit checks by default.

### Proposed Code Split

```txt
frontend/src/modules/pipeline-builder/
  PipelineBuilderPage.tsx
  components/
    PipelineBuilderShell.tsx
    TemplateSidebar.tsx
    NodeLibraryPanel.tsx
    NodeInspectorPanel.tsx
    diagram/
      DiagramCanvas.tsx
      NodeLayer.tsx
      NodeCard.tsx
      ConnectorLayer.tsx
      AdornerLayer.tsx
      DragPreviewLayer.tsx
      diagramTypes.ts
      diagramConstants.ts
      geometry.ts
      hitTesting.ts
      router.ts
      interactionReducer.ts
      edgePersistence.ts
```

### Slice 1: Shell And Panels

Goal: split page shell/sidebar/library/inspector from `PipelineBuilderPage.tsx` without changing canvas behavior.

Expected files:
- `PipelineBuilderPage.tsx`
- `components/PipelineBuilderShell.tsx`
- `components/TemplateSidebar.tsx`
- `components/NodeLibraryPanel.tsx`
- `components/NodeInspectorPanel.tsx`

Done when:
- Page remains functional.
- `PipelineBuilderPage.tsx` no longer contains sidebar and inspector markup.
- Build passes.

### Slice 2: DiagramCanvas And NodeLayer

Goal: move canvas pan, node drag, and node card rendering out of `PipelineBuilderPage.tsx`.

Expected files:
- `components/diagram/DiagramCanvas.tsx`
- `components/diagram/NodeLayer.tsx`
- `components/diagram/NodeCard.tsx`
- `components/diagram/diagramTypes.ts`
- `components/diagram/diagramConstants.ts`

Done when:
- Canvas remains fullscreen.
- Node drag remains unlimited in world coordinates.
- No connector or adorner behavior is introduced yet.
- Build passes.

### Slice 3: Geometry Foundation

Goal: define the production geometry model before rendering connector UI.

Core concepts:
- `DiagramNodeRect`
- `PortSide`
- `PortRef`
- `ConnectorEndpoint`
- `ConnectorModel`
- screen/world coordinate conversion
- node rect and port-point calculation
- nearest-port and hit-testing helpers

Expected files:
- `components/diagram/geometry.ts`
- `components/diagram/hitTesting.ts`
- `components/diagram/diagramTypes.ts`

Done when:
- Geometry helpers are pure functions.
- Existing node drag still works.
- Helpers are testable without React.

### Slice 4: AdornerLayer

Goal: add a proper interaction overlay. The adorner is not part of the node card.

Behavior:
- Hover/select node shows outline and subtle boundary handles.
- Directional connector affordances appear outside the node boundary.
- Visual handles can be small, but hit zones must be comfortable.
- Adorner emits interaction intents only; it does not persist connector data.

Expected files:
- `components/diagram/AdornerLayer.tsx`
- `components/diagram/interactionReducer.ts`
- `components/diagram/hitTesting.ts`

Done when:
- Adorner appears/disappears correctly.
- No connector is persisted yet.
- Node drag and canvas pan do not conflict with adorner hit zones.

### Slice 5: Connector Read-Only Rendering

Goal: render saved template edges using the new connector model.

Behavior:
- Existing `template.edges` render as connectors.
- Connector start/end points land on node boundaries.
- Nodes render above connectors.
- Old edges without metadata use a deterministic fallback, usually right-to-left.

Persistence approach:
- Prefer versioned metadata under `condition_config.diagram`.
- Do not add a backend migration unless later requirements force one.

Expected files:
- `components/diagram/ConnectorLayer.tsx`
- `components/diagram/router.ts`
- `components/diagram/edgePersistence.ts`

Done when:
- Read-only connectors render from existing data.
- No ad hoc SVG path logic lives in `PipelineBuilderPage.tsx`.
- Build passes.

### Slice 6: Connector Creation

Goal: drag from an adorner affordance to create a connector.

Behavior:
- Drag starts from a logical source port.
- Preview follows the pointer in world coordinates.
- Target node/port highlights when snap is valid.
- Invalid drop cancels without side effects.
- Duplicate edge is blocked.
- Successful drop creates an edge and stores diagram metadata.

Expected files:
- `components/diagram/DragPreviewLayer.tsx`
- `components/diagram/interactionReducer.ts`
- `components/diagram/edgePersistence.ts`

Done when:
- Creation flow feels deterministic.
- API mutation stays page-level or in a narrow persistence helper, not inside drawing geometry.
- Success/error toast behavior is preserved.

### Slice 7: Connector Selection And Reconnect

Goal: allow production-grade connector editing.

Behavior:
- Click connector to select it.
- Selected connector shows endpoint handles.
- Dragging an endpoint reconnects it to another valid node/port.
- Invalid drop rolls back visually and does not mutate data.
- Reconnect updates the existing edge; it should not delete and recreate the edge.

Expected files:
- `components/diagram/ConnectorLayer.tsx`
- `components/diagram/interactionReducer.ts`
- `components/diagram/edgePersistence.ts`

Done when:
- Endpoint drag does not conflict with canvas pan.
- Existing edge identity is preserved.
- Build passes.

### Slice 8: Router Hardening

Goal: make routing production-worthy for common builder layouts.

Router requirements:
- Orthogonal/manhattan segments.
- Normal exit segment from the source port.
- Normal entry segment into the target port.
- Avoid node rectangles in common cases.
- Separate parallel connectors on the same side.
- Deterministic route so connectors do not jump unnecessarily during small node moves.

Expected files:
- `components/diagram/router.ts`
- `components/diagram/geometry.ts`

Done when:
- Node movement reroutes connectors realtime.
- Connector lines generally do not run through node cards.
- Parallel connectors are visually separable.

### Slice 9: Commands And Polish

Goal: add expected editor commands after the interaction model is stable.

Behavior:
- Delete selected connector.
- Escape cancels active connector drag/reconnect.
- Click empty canvas clears selection.
- Pending mutation states prevent conflicting edits.
- Toasts remain dismissible and useful.

Done when:
- Keyboard/pointer states are explicit.
- No hidden connector state is left stale after template switch, node delete, or mutation failure.

### Slice 10: Validation And Regression Checks

Goal: protect the implementation from another connector/adorner regression.

Checks:
- `cd frontend && npm run build`
- Add focused tests for pure helpers if frontend test infrastructure is available or easy to introduce locally:
  - `geometry.ts`
  - `hitTesting.ts`
  - `router.ts`
- Do not run Browser/headless verification unless explicitly requested by the user.

Done when:
- Build passes.
- `PipelineBuilderPage.tsx` remains thin.
- Connector, adorner, routing, and hit-testing logic live in dedicated files.
