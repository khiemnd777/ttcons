---
name: consflow-accounting-finance-domain
description: Protect Consflow accounting and finance workflows across budgets, payables, receivables, payments, payment schedules, advances, settlements, input invoice PDFs, output invoice PDFs, AP/AR aging, invoice/payment reconciliation, cashflow, tax/VAT, profit and loss, and financial report exports. Use when tasks mention accounting, ke toan, budget, payable, receivable, cong no, payment, receipt, advance, settlement, invoice PDF, hoa don, cashflow, tax, VAT, loi nhuan, profit, or P&L.
---

# Consflow Accounting Finance Domain

Use this skill for financial state, documents, and reports.

## Required Reading

- `AGENTS.md`
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
- `references/domain-map.md`
- Pair with `consflow-reporting-analytics-domain` for report UX/export changes and with source-domain skills for operational source documents.

## Workflow

1. Identify source document, project, counterparty, invoice, payable/receivable, schedule, advance, payment, allocation, and actor.
2. Preserve payable and receivable reconciliation across invoices, payments/receipts, schedules, advances, settlements, and remaining balances.
3. Keep input and output invoice PDF storage protected and linked to the correct AP/AR record and project.
4. Treat cashflow as cash-basis from payment ledger, tax as invoice/VAT ledger, and profit as invoice-based revenue/cost unless local code says otherwise.
5. Preserve approval, issue/void, verify/reject, pay/receive, and settle state transitions with actor/comment/timestamp.
6. For feature, bugfix, code-changing, or UI-changing requests, stop before editing and produce the repo review package. Include current contract/invariant, evidence, root cause or unresolved risk, clarification questions, implementation plan, validation plan, and any required Mockup as Text. Wait for explicit user confirmation.

## Guardrails

- Do not record payments or receipts without updating allocation/reconciliation semantics.
- Do not mix input invoices with receivables or output invoices with payables.
- Do not bypass invoice status gates for tax, profit, or aging reports.
- Do not expose financial data outside project/global permission scopes.

## Output Expectations

- State affected finance flow: budget, AP, AR, payment, advance, invoice, reconciliation, cashflow, tax, or profit.
- Name financial source documents and status transitions.
- List operational source domains checked.
- Report focused validation and skipped browser checks.
