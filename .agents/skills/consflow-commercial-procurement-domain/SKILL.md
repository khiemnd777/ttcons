---
name: consflow-commercial-procurement-domain
description: Protect Consflow commercial workflows across BOQ, estimates, unit price lifecycle, procurement requests and items, BOQ to procurement generation, purchase status transitions, subcontractor reconciliation, owner/client reconciliation, and quantity or price audit trails. Use when tasks mention BOQ, du toan, estimate, unit price, don gia, procurement, purchase request, mua hang, thau phu reconciliation, client reconciliation, quantity variance, or commercial approval.
---

# Consflow Commercial Procurement Domain

Use this skill for quantity, price, purchasing, and reconciliation flows before money or warehouse movement is finalized.

## Required Reading

- `AGENTS.md`
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
- `references/domain-map.md`
- Pair with `consflow-materials-warehouse-domain` for stock movement and `consflow-accounting-finance-domain` for payable/receivable effects.

## Workflow

1. Identify the commercial source: BOQ line, unit price event, procurement request/item, subcontractor reconciliation, or owner/client reconciliation.
2. Preserve BOQ approval and final unit-price locks before procurement generation.
3. Keep procurement status transitions explicit: submit, approve, order, receive.
4. Check whether receive actions create or rely on stock movements and route those changes through the materials/warehouse domain.
5. Preserve reconciliation variance, decision, actor, comment, and source document links.
6. For code or UI changes, follow the repo review package and Mockup as Text gates before editing.

## Guardrails

- Do not generate procurement from unapproved BOQ lines.
- Do not let final-lock procurement propagation rewrite historical price events.
- Do not hide quantity variance or reconciliation warnings.
- Do not update financial documents as a side effect unless accounting/finance rules are checked.

## Output Expectations

- State whether the change affects BOQ, unit price, procurement, subcontractor reconciliation, or owner/client reconciliation.
- Name source and target documents.
- List stock, quality, and finance downstream checks.
- Report focused validation and skipped browser checks.
