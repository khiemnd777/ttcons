# Consflow Domain Model Notes

This is the legacy cross-domain overview. Prefer the narrower production-grade domain skills for implementation details.

## Core Concepts

- Project: construction job context with members, activity, documents, and scoped permissions.
- Pipeline template: reusable workflow definition.
- Pipeline run: project-specific runtime copy of a template.
- Work item: executable runtime task, often backed by workflow form, evidence, approval, signature, or PDF gates.
- Workflow form template/version/instance: business form governance model. Published versions are immutable; instances preserve stable version references.
- Approval: explicit decision gate tied to workflow, work item, document, budget, procurement, debt, or payment action.
- BOQ and unit price: quantity and commercial basis. Expected, norm-locked, and final-locked prices are auditable.
- Procurement request: purchase workflow that can originate from approved/final-locked BOQ lines.
- Warehouse stock movement: receive, issue, transfer, or return event with material balance traceability.
- Site material balance: project-level material usage/surplus state, distinct from warehouse stock.
- Incoming acceptance: delivery/material quality check, not an implicit stock movement.
- Outgoing acceptance: client/output quality gate with checklist, evidence, signature/document readiness, and rework linkage.
- Rework/replanning: corrective workflow that preserves original acceptance and schedule history.
- Handover: final project closure workflow with readiness blockers.
- Accounting documents: budgets, payables, receivables, invoices, payment schedules, advances, settlements, and payments.
- Reports: dashboard, operational reports, cashflow, tax/VAT, profit/P&L, and CSV exports.
- Admin governance: auth sessions, staff/users, roles, permissions, project-scoped RBAC, partners, audit, and settings.

## Typical Flow

1. Create project and assign members/permissions.
2. Choose, clone, or publish a pipeline template.
3. Map template steps to workflow form versions and required role arrays.
4. Start a project pipeline run with runtime work items.
5. Complete work item forms, evidence, approvals, signatures, and PDFs.
6. Prepare BOQ, lock unit prices, approve, and generate procurement where needed.
7. Receive, issue, transfer, or return stock with traceable movements.
8. Record construction reports, checklist evidence, material before/after snapshots, and surplus handling.
9. Run incoming/outgoing acceptance, fail/rework/re-acceptance, and handover readiness.
10. Record invoices, schedules, advances, payments/receipts, settlements, and reconciliation.
11. Review dashboard, operational reports, cashflow, tax/VAT, profit/P&L, audit, and settings.

## Production-Grade Domain Skills

- `consflow-workflow-orchestration-domain`: pipeline, runtime work items, approvals, workflow forms, signatures, PDFs, gates.
- `consflow-project-planning-domain`: projects, planning workspace, daily reports, meeting minutes, planning checklists, activity.
- `consflow-construction-operations-domain`: field reports, site issues, evidence, construction forms, work item execution.
- `consflow-commercial-procurement-domain`: BOQ, unit price, procurement, quantity/price reconciliation.
- `consflow-materials-warehouse-domain`: materials, stock movements, site balances, snapshots, surplus.
- `consflow-quality-acceptance-handover-domain`: incoming/outgoing acceptance, rework, replanning, handover.
- `consflow-accounting-finance-domain`: AP/AR, invoices, payments, advances, reconciliation, cashflow, tax, profit.
- `consflow-admin-governance-domain`: auth sessions, staff/RBAC, permissions, partners, audit, settings.
- `consflow-reporting-analytics-domain`: dashboard, reports, warnings, filters, export parity.

## Cross-Domain Risk Areas

- Template-vs-runtime mutation bugs.
- `required_role_ids` collapsing from multi-role array to single role.
- Published workflow form version edits that break runtime history.
- Status drift between backend and frontend.
- Approvals or signatures that update state without actor/comment/hash context.
- Procurement generation that bypasses BOQ approval or final price locks.
- Stock movement updates that do not preserve balance and traceability.
- Material snapshot or acceptance flows that create unintended stock movement.
- Acceptance/rework loops that erase quality history.
- Handover completion that ignores readiness blockers.
- AP/AR/payment/invoice transitions that fail reconciliation.
- Report/export queries that ignore permission scope or source metric basis.
- Partner catalog updates that rewrite historical snapshots.
- Auth/session changes that weaken refresh/revoke lifecycle.
