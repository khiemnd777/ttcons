# Consflow Domain Model Notes

Core concepts:

- Project: the construction job context.
- Pipeline template: reusable workflow definition.
- Pipeline run: project-specific copy of a template.
- Pipeline node: major workflow phase.
- Pipeline step: actionable checklist or submission inside a node.
- Approval: explicit decision gate tied to a workflow, document, budget, procurement, debt, or payment action.
- BOQ: bill of quantities and cost basis.
- Procurement request: purchase workflow that can originate from BOQ.
- Warehouse stock movement: receive, issue, transfer, or return event.
- Acceptance: quality handoff check.
- Rework: corrective work created manually or from failed acceptance.
- Handover: final project closure workflow.
- Audit log: history used to understand who changed what and why.

Typical flow:

1. Create project.
2. Choose or clone a pipeline template.
3. Start a project pipeline run.
4. Complete node steps and request approvals.
5. Prepare BOQ and submit for approval.
6. Generate procurement from approved BOQ where needed.
7. Receive and issue warehouse stock.
8. Record operations, acceptance, rework, handover, accounting, and reports.

Risk areas:

- Template-vs-run mutation bugs.
- Status drift between backend and frontend.
- Approvals that update state without actor/comment context.
- Procurement generation that bypasses BOQ approval.
- Stock movement updates that do not preserve balance.
- Acceptance and rework loops that hide quality history.
