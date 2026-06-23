---
name: consflow-materials-warehouse-domain
description: Protect Consflow material and warehouse workflows across materials, warehouses, stock receive/issue/transfer/return, stock movement ledgers, site material balances, material before/after snapshots, surplus handling, custody location, and stock traceability. Use when tasks mention kho, warehouse, material, vat tu, stock, ton kho, xuat kho, nhap kho, transfer, return, material balance, before after material, surplus, or custody.
---

# Consflow Materials Warehouse Domain

Use this skill for inventory state and material traceability.

## Required Reading

- `AGENTS.md`
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
- `references/domain-map.md`
- Pair with `consflow-commercial-procurement-domain` for procurement receive and `consflow-quality-acceptance-handover-domain` for incoming acceptance.

## Workflow

1. Identify the source document, material, project, warehouse/custody location, quantity, movement type, and actor.
2. Preserve stock movement traceability for receive, issue, transfer, return-to-warehouse, and return-to-subcontractor.
3. Keep site material balances separate from warehouse stock unless a movement explicitly links them.
4. For material before/after snapshots, preserve submit-time site balance usage and evidence links.
5. For surplus handling, preserve partial/handled balances, custody, condition, evidence, and movement linkage.
6. For code or UI changes, follow the repo review package and Mockup as Text gates before editing.

## Guardrails

- Do not create implicit stock movement from acceptance or snapshots unless the existing domain contract requires it.
- Do not allow negative or untraceable material balances.
- Do not rely on frontend-only filtering for active materials, warehouses, or stock records.
- Do not hide movement source and target document links.

## Output Expectations

- State movement type or material balance workflow affected.
- Name source document and target stock/site balance records.
- List commercial, construction, and quality downstream checks.
- Report focused validation and skipped browser checks.
