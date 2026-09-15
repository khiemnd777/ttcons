# [Closed] Chuẩn hóa tab workspace theo Thành viên

Status: closed on 2026-09-15 by user request.

## Objective

Dùng cùng một kiểu tab phẳng, icon 16 px, chữ xanh và gạch chân xanh khi active như màn Thành viên cho các workspace và bộ lọc nghiệp vụ đã được duyệt.

## Completed Scope

- Tạo owner trình bày dùng chung `frontend/src/components/WorkspaceTabs.tsx` với tab phẳng, icon Lucide, count chip, active underline và cuộn ngang trên màn hình hẹp.
- Áp dụng cho Project Detail/Thành viên, BOQ, Mua sắm, Kho vật tư, Thi công và Bàn giao kế toán.
- Áp dụng cho bộ lọc `Hồ sơ trong kế hoạch`, `Hàng đợi công việc` và bộ lọc Đầu việc theo trạng thái trong `Quy trình`.
- Bộ lọc Quy trình dùng chung cho route Pipeline và tab Quy trình trong Project Detail.
- Giữ nguyên state, count, quyền, URL, query, tìm kiếm, filter predicates và cơ chế reset theo trạng thái Giai đoạn.
- Cập nhật module map, frontend map, Project Planning map, Workflow Orchestration map và focused regression coverage.
- Không thay đổi API, database, migration hoặc workflow transition.

## Validation

- Tested source: `HEAD 86bed4f` plus the tab-standardization working-tree changes.
- `cd frontend && node --experimental-strip-types --test tests/workspaceTabs.test.ts tests/projectDetailTabs.test.ts tests/pipelineStageNavigation.test.ts tests/preparationWorkspaceResponsive.test.ts tests/planningTaskContextNavigation.test.ts`: passed 21/21.
- `cd frontend && npm run build`: passed; Vite transformed 1,950 modules.
- `git diff --check`: passed.
- Full frontend suite: 478/479 passed. The sole failure is the pre-existing `projectEvidenceColumns.test.ts` expectation for a separate `Giai đoạn` column while `HEAD` already renders `Nguồn / Giai đoạn`; neither baseline file belongs to this plan.
- Browser/UAT was not run because it was not explicitly requested. Final UX status remains `NOT UX ACCEPTED`; implementation status is `TECHNICALLY VERIFIED`.

## Key Files

- `frontend/src/components/WorkspaceTabs.tsx`
- `frontend/src/modules/projects/ProjectDetailPage.tsx`
- `frontend/src/modules/planning/PreparationPage.tsx`
- `frontend/src/modules/planning/PlanningPage.tsx`
- `frontend/src/modules/boq/BOQPage.tsx`
- `frontend/src/modules/procurement/ProcurementPage.tsx`
- `frontend/src/modules/warehouse/WarehousePage.tsx`
- `frontend/src/modules/operations/OperationsPage.tsx`
- `frontend/src/modules/project-finance/AccountingHandoffPage.tsx`
- `frontend/tests/workspaceTabs.test.ts`
- `frontend/tests/projectDetailTabs.test.ts`
