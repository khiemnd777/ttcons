# [Closed] Làm rõ Phạm vi, bộ lọc nhanh và phê duyệt trên Bàn làm việc

Closed on: 2026-08-24

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED

## Will be done

- Keep `Phạm vi` as the assignment/visibility dimension for ordinary work only: `Tôi phụ trách`, `Giao trực tiếp`, `Phòng ban`, `Đang theo dõi`, and `Tất cả được phép xem`.
- Move the operational status controls into the ordinary list column as `Bộ lọc nhanh · <phạm vi>` and add an explicit source-backed `Tất cả` total anchor; scope changes select that all-work bucket visibly.
- Separate actor-decidable work-item approvals and pipeline-node approvals into one dedicated `Chờ tôi duyệt` mode/tab, without changing backend approval, RBAC, readiness, mutation or audit semantics.
- Preserve legacy `queue=pending_approval` and `bucket=waiting_decision` links by presenting the dedicated approval mode, while new navigation uses the canonical `mode=approval` presentation state.
- Add focused frontend regression coverage, update central/frontend/Workflow Orchestration maps and run the full frontend tests, production build, bundle budget and whitespace validation. Browser/UAT remains skipped unless separately authorized.

## Done

- User approved the diagnosis, proposed information architecture and Mockup as Text with `OK triển khai`.
- Loaded the mandatory central/frontend and Workflow Orchestration maps plus Frontend Workflow instructions; confirmed `frontend/src/modules/my-work/MyWorkPage.tsx` and `frontend/tests/myWorkListPresentation.test.ts` as the implementation/test owners.
- Confirmed the backend counts already match their contracts: ordinary summary values follow the selected queue, `waiting_decision` is actor-decidable independently, and the five former cards were overlapping quick filters rather than a complete partition. No backend/API/schema/migration/seed change is planned.
- Removed `Chờ tôi duyệt` from the ordinary assignment/visibility scope. The remaining label-only scope rail contains `Tôi phụ trách`, `Giao trực tiếp`, `Phòng ban`, `Đang theo dõi` and `Tất cả được phép xem`; selecting any scope visibly selects `Tất cả`.
- Moved the former top-level cards into the central list column as `Bộ lọc nhanh · <phạm vi>`, added the source-backed `Tất cả=summary.total` anchor and retained `Làm ngay`, `Đang làm`, `Bị chặn` and read-only `Sắp tới` as explicitly scoped operational subsets.
- Added the dedicated `Chờ tôi duyệt` mode with a lazy actor-decidable work-item list, the existing non-synthetic pipeline-node approval lane, a project filter, combined badge count and one truthful overall empty state. Node approvals are no longer added to ordinary work totals.
- Kept legacy `queue=pending_approval` and `bucket=waiting_decision` URLs presentation-compatible while canonical tab navigation uses `mode=approval`. Backend routes, response shapes, queue predicates, RBAC, readiness, approval mutation and audit contracts remain unchanged.
- Added focused source-contract regressions and updated the central/frontend/root-cause/Workflow Orchestration maps with the new hierarchy and compatibility contract.
- Current working tree at HEAD `c4206a3` plus the listed changes passed 262/262 frontend tests, TypeScript/Vite production build, the 250 KiB gzip/141-chunk bundle budget and `git diff --check`; the My Work chunk is 10.0 KiB gzip.
- Rebuilt and restarted the local Docker stack through the required `down -> build -> run` sequence. PostgreSQL and both Redis services are healthy, frontend `/my-work` returns HTTP 200 and backend `/api/health` returns `ok`.

## Skipped / remaining acceptance

- Browser/UAT was not authorized in this turn and remains `NOT UX ACCEPTED`.
- Production deployment was not requested and was not performed.
