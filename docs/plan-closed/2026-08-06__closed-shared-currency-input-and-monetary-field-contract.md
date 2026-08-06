# [Closed] Shared Currency Input And Monetary Field Contract

Status: closed

Closed at: 2026-08-06

## Final status

- No mandatory implementation or non-browser verification remains.
- Browser/UAT was not requested in the final implementation turn, so the UI remains `NOT UX ACCEPTED` at closure.

## Done

- Loaded the mandatory central/frontend/backend/root-cause maps and the project-planning, commercial-procurement, accounting-finance, warehouse, construction, quality and workflow-orchestration domain maps.
- Confirmed the root cause: monetary controls used native `type="number"`; output-only `CurrencyText` existed, but no shared currency input or semantic currency field existed.
- Audited 47 direct React monetary fields and 13 legacy workflow monetary fields. `management_rate` remains an ordinary number because current source does not prove that it is a money amount.
- Produced the impact matrix, implementation review package and Mockup as Text; received explicit `OK triển khai` for the reviewed scope and the immutable-workflow compatibility strategy.
- Added shared `CurrencyInput` plus pure parse/format/caret helpers: vi-VN grouping, common pasted formats, canonical numeric values, optional negative values, configurable fraction digits and an explicit currency-code suffix; `SimpleForm` now owns semantic `currency` fields and still submits numeric API payloads.
- Replaced all 47 confirmed direct React monetary inputs across Projects, Warehouse, Procurement, Rework, Construction, BOQ, Supplier Quotations, Contracts, Project Finance, Accounting, Tax and Debt. Remaining native/generic number fields were re-audited as quantities, percentages, day counts, progress, ordering, quorum or editor metadata.
- Added exact compatibility mappings for eight top-level and five structured V41 legacy monetary fields. Rendering uses effective cloned currency definitions and does not update published workflow definitions; `management_rate` remains numeric and label inference is prohibited.
- Added workflow-editor/runtime/backend support for `field_type=currency`, strict currency configuration validation and additive V91 constraint migration. Applied V91 to local PostgreSQL and confirmed both schema history row `91|workflow currency field type` and the live constraint contain `currency`.
- Recorded the mandatory monetary-input rule in `AGENTS.md` and updated central, frontend, backend, root-cause plus affected business-domain maps.
- Confirmed from the supplied project-edit screenshot that VND rendered `2.850.000.000,00`; a read-only formatter reproduction returned the same value, while `Intl.NumberFormat` resolves VND to 0 and USD to 2 minor-unit digits.
- Confirmed local PostgreSQL had no persisted workflow fields using the new `currency` type and all current contract/supplier-quotation currencies were VND, so the decimal correction required no additional schema or data migration.
- Added currency-aware minor-unit resolution to shared `CurrencyInput`: VND always uses 0 fraction digits even when a caller supplies 2, while USD/JPY/KWD and other valid codes use ISO defaults unless a non-VND precision is explicit. VND payload/paste `.00` is removed from presentation without changing numeric API semantics.
- Preserved grouped-integer editing around the vi-VN `.` separator, including backspace from `1.000` to `100` and append from `1.000` to `10.000`; a comma followed by unsupported VND fraction digits is discarded rather than concatenated.
- Changed the exact V41 legacy compatibility registry to VND precision 0, made workflow runtime inherit currency-aware defaults, made new workflow VND drafts visibly fixed at 0, and made frontend/backend validation reject VND fractional configuration while retaining valid USD precision.
- Updated `AGENTS.md`, central/frontend/root-cause maps and the workflow-orchestration domain map with the durable VND integer-only and non-VND ISO-minor-unit contract.
- Technically verified the exact staged snapshot: focused currency/workflow tests passed, frontend 56/56 tests passed, TypeScript lint passed, production Vite build passed, bundle budget passed (121.4 KiB gzip entry; 107 JavaScript chunks under the 250 KiB ceiling), full backend `go test ./...` passed, all 91 migration checks passed and `git diff --cached --check` passed.
