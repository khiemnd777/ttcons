# [Closed] Project Active Work Timeline Filter

Status: closed on 2026-06-13. The project detail `Active Work` panel now filters by the selected `Workflow Timeline` node.

## Objective

Filter the project detail `Active Work` panel by the selected `Workflow Timeline` node instead of showing all open project work items.

## Completed Scope

- Added selected workflow node state to `ProjectDetailPage`.
- Defaulted the selection to the current runtime node.
- Made `Workflow Timeline` nodes clickable with selected styling.
- Filtered `Active Work` by selected `run_node_id` from the full `work_items` response.
- Updated the `Active Work` heading and empty state to reflect the selected workflow node.

## Validation

- `cd frontend && npm run build`: passed.
- `git diff --check`: passed.
- Browser verification skipped per repo instruction because it was not explicitly requested.

## Key Files

- `frontend/src/modules/projects/ProjectDetailPage.tsx`
- `docs/current-states.md`
