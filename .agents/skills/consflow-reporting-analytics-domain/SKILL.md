---
name: consflow-reporting-analytics-domain
description: Protect Consflow reporting and analytics workflows across dashboard, project reports, warehouse reports, procurement reports, accounting reports, staff reports, cashflow reports, tax/VAT reports, profit reports, KPI summaries, warnings, filters, scoped visibility, and CSV export parity. Use when tasks mention reports, bao cao, dashboard, KPI, export CSV, cashflow report, tax report, profit report, warehouse report, procurement report, accounting report, staff report, or analytics.
---

# Consflow Reporting Analytics Domain

Use this skill for source-backed reporting, dashboard, and export work.

## Required Reading

- `AGENTS.md`
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
- `references/domain-map.md`
- Pair with the domain that owns the source metric before changing report semantics.

## Workflow

1. Identify report audience, source tables, filters, grouping, visibility scope, warnings, and export path.
2. Confirm the source domain owns the metric definition: accounting, materials, commercial, quality, planning, or admin.
3. Keep backend report queries as the source of truth. Frontend tables may filter/sort for display but must not invent scoped data.
4. Keep CSV exports aligned with the same filters and source logic as the UI endpoint.
5. Preserve warning checks for missing data, reconciliation gaps, tax code gaps, negative profit, stale status, or other report-specific caveats.
6. For feature, bugfix, code-changing, or UI-changing requests, stop before editing and produce the repo review package. Include current contract/invariant, evidence, root cause or unresolved risk, clarification questions, implementation plan, validation plan, and any required Mockup as Text. Wait for explicit user confirmation.

## Guardrails

- Do not mix cash-basis and invoice-basis metrics without naming the basis.
- Do not expose report rows across project permission boundaries.
- Do not make dashboard totals diverge from underlying report endpoints without documenting the reason.
- Do not add decorative report UI at the cost of dense operational scanability.

## Output Expectations

- State report type, grain, filters, and source tables.
- Name the owning source domain and metric basis.
- Call out export parity and permission-scope checks.
- Report focused validation and skipped browser checks.
