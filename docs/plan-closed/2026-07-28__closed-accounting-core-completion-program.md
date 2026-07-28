# [Closed] Accounting Core Completion Program

Status: closed with recorded acceptance limitation

Closed at: 2026-07-28

Acceptance limitation:

- The final `8.6/10` result is a task-based Browser heuristic on the accounting demo role and Golden Project. No task-based session with real accountants was run, so the result is not `user validated` and the closed plan must not be cited as real-user UX acceptance.

Done:

- Corrected status after the first authorized built-UI Browser UAT: UX was `7.2/10 heuristic`, VAT `5.0/10`, responsive `5.5/10`; this was not evidence of an 8.5 result.
- Final post-build Browser UAT passed the agreed heuristic gates: average `8.6/10`, VAT `8.6/10`, responsive `8.4/10`, with no blocking defect or Browser console warning.
- Rebuilt the full Docker stack after every score-limiting UI correction. The final runtime reported migrations `applied=0 skipped=82`, `/api/health` returned 200, and port 5173 served the corrected accounting implementation.
- Browser UAT exercised desktop navigation, project selection, budget actual/commitment/payment detail, payment reversal validation, balanced Journal/General Ledger detail, manual CTGS validation, monthly VAT readiness, purchase/sales ledgers, declaration summary, adjustment/history/action dialogs, and the 390x844 mobile journal/VAT/manual-CTGS flows without committing a destructive accounting action.
- Repaired defects found by Browser UAT: localized raw journal source `payment`; persisted manual CTGS source-document number through V82; aligned provisional VAT payable `129,045,455` across readiness and declaration summary; removed the input-only deductible column from the sales ledger; wrapped long CTGS numbers. Final mobile measurements were `scrollWidth=390` at `innerWidth=390` for both Journal and VAT.
- Final verification after those repairs passed: full backend `go test -p 1 ./... -count=1` on a fresh V82 PostgreSQL database, focused DB-backed CTGS/VAT regression tests, 23 frontend tests, TypeScript/Vite production build, 250 KiB bundle budget, Docker health, and zero Browser warnings/errors.
- Recorded the original main-database V74 blocker, then repaired the real upgrade path without changing applied V73: V74 releases only transient V73 finance constraints before dimensions; V77 normalizes legacy `paid|received` vouchers to `approved`, preserves `paid_at`, and clears unusable target kinds that have no target ID. The isolated V72-to-V81 regression covers `recorded`, `waiting_approval`, `paid`, `received` and partial targets.
- Main PostgreSQL reached V72-V81. Post-migration evidence: 13 vouchers were canonical `approved` with no missing `paid_at`, zero partial target pairs, zero unvalidated constraints in the accounting/VAT tables, zero unbalanced posted journals, and current-year 12 monthly plus 4 quarterly VAT periods.
- Implemented the VAT-only lifecycle in `accounting_tax.go`/V81: canonical readiness and TK 1331/3331 reconciliation, immutable prepare/file/lock events and return snapshots, append-only supplemental declarations, balanced adjustment/reversal, linked VAT payment Dr 3331/Cr cash with no budget movement, source-period mutation gate and purchase/sales/declaration CSV. TNDN/TNCN remain explicitly out of scope.
- Added `TaxWorkspace` with accounting-language issues-first hierarchy, declaration/purchase/sales/adjustment/history tabs and contextual actions; added Journal detail/reversal, responsive journal/manual-entry cards, stale validation reset, collapsed mobile project navigation, global Kế toán navigation, fiscal-year cashflow fallback and Vietnamese source/event labels.
- Added the approved posting policy in `docs/accounting/accounting-posting-policy.md` and migrations V73-V80 for books, chart of accounts, periods, immutable balanced journals, posting rules, append-only budget movements, VAT dimensions, payment reversals, aligned constraints, ledger-derived budget balances, idempotent historical backfill and controlled reconciliation resolution.
- Added project/account/tax/budget dimensions and DB defaults to legacy finance sources without changing AP/AR ownership. Verified input invoices, issued output invoices, approved allocated/generic vouchers and their reversals post exactly once in the same transaction as their source transition.
- Replaced static budget usage in accounting/dashboard/report reads with movement-derived Đã cam kết, Thực chi ghi nhận, Đã thanh toán and Ngân sách khả dụng; actual cost is recognized from verified input invoices and explicit generic expense vouchers rather than cash timing.
- Blocked direct payable `pay` and receivable `receive`; settlement must use allocated Thu/Chi vouchers. Posted invoice correction and approved payment correction require a reason, preserve the original record and create balanced reversal journals plus offsetting allocation/budget effects.
- Added permission-scoped Trial Balance, General Ledger, P&L and VAT ledger APIs, accounting period close/reopen, manual CTGS posting and a migration-reconciliation queue. An issue is resolved only by a same-project posted CTGS or accepted without posting with an explicit accounting basis.
- Reworked Accounting UI into Tổng quan, Việc cần xử lý, Ngân sách, Thu/Chi, Sổ Nhật ký chung, Sổ Cái and Thuế; added business-document/account selectors, live Nợ/Có validation, period controls, safe payment reversal and migration reconciliation without raw UUID entry.
- Added accounting contract/status types, ledger-specific permissions and seed parity. Main seed and Golden finance fixtures use the canonical `approved` voucher status; post-seed backfill creates journals and queues only genuinely ambiguous historical vouchers.
- Updated central/backend/frontend/accounting/reporting/admin/commercial maps for every changed route, table, invariant and UI owner.
