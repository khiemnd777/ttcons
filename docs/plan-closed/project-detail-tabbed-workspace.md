# [Closed] Project Detail Tabbed Workspace

Status: closed on 2026-06-13. The project detail workspace is now split into operational tabs, with the overview focused on summary, shortcuts, health, next work, and command signals.

## Objective

Refactor the project detail screen so overview, workflow, work items, communication, files, audit, and members are separated into tabs instead of one long mixed page. Keep Command Center focused only on immediate coordination signals.

## Completed Scope

- Refactored `ProjectDetailPage` into tabbed panels: `Tổng quan`, `Workflow`, `Công việc`, `Trao đổi`, `Hồ sơ`, `Audit`, and `Thành viên`.
- Removed `ActivityThread` and audit content from `Command Center`.
- Added dedicated `Trao đổi`, `Hồ sơ`, and `Audit` tab panels using existing API/data contracts.
- Moved project overview module shortcuts above the health cards and renamed the heading to `Module Shortcuts`.
- Updated `Next Work` cards to display titles as `Workflow / Work item title`, such as `Khảo sát / Upload ảnh hiện trạng`.
- Updated `Next Work` count to `6 / 30 việc phải làm`.
- Added `Xem tất cả` to open the `Công việc` tab.
- Added `Tất cả` work scope and paginated work items 10 per page.
- Kept backend/API unchanged.

## Validation

- `cd frontend && npm run build`: passed.
- Browser verification skipped per repo instruction because it was not explicitly requested.
- Vite still reports the existing chunk size warning over 500 kB.

## Key Files

- `frontend/src/modules/projects/ProjectDetailPage.tsx`
- `frontend/src/modules/work-items/WorkItemCard.tsx`
- `docs/current-states.md`
