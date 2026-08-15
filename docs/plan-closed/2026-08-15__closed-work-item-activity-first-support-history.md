# [Closed] Work Item Activity-First Support History

## Will be done

- Put `Trao đổi & lịch sử` first and select it by default in work-item supporting tabs on desktop and mobile.
- Move workflow-form event history out of the primary two-step processing surface and expose it as the `Hồ sơ` filter inside `Trao đổi & lịch sử`, with `Tất cả` remaining the inner default.
- Preserve evidence, workflow-form, activity, permissions and runtime behavior; add focused frontend regression coverage and update the central/frontend/workflow mappings.
- Run non-browser frontend tests/build and report browser/UAT as skipped.

## Done

- Confirmed from the supplied Edge screenshot and current source that `Lịch sử hồ sơ` is rendered directly after the two-step workflow-form process while `Trao đổi & lịch sử` is currently second and `Minh chứng` is the selected default.
- Produced the revised Mockup as Text and received explicit `OK triển khai` approval for `Trao đổi & lịch sử` as the first/default support tab and `Hồ sơ` as its history filter.
- Made `Trao đổi & lịch sử` the first/default support tab and reset that default when the selected work item changes; `Minh chứng` follows without changing its evidence behavior.
- Added inner `Tất cả`, `Trao đổi`, and form-backed `Hồ sơ` filters. `Tất cả` remains the inner default, composing a new activity switches to `Trao đổi`, and `Hồ sơ` reuses the canonical `['work-item-form', workItemId]` query/cache with explicit loading, retryable error and empty states.
- Removed the standalone form-history disclosure from the two-step primary process and reused its exact workflow-form event labels/timestamps in the support history surface, preserving the prior five-most-recent display limit with explicit truncation copy.
- Added focused source-contract regression coverage and updated the central/frontend/workflow-orchestration maps. Backend/API/database, permissions and runtime workflow behavior remain unchanged.
- Technically verified the current frontend source with the focused work-item layout test (6/6), the complete frontend suite (110/110), the TypeScript/Vite production build, bundle budget (121.5 KiB gzip entry; 111 JavaScript chunks) and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.

## Closure

- Closed on 2026-08-15 at the user's request.
- This file is a historical record only. Durable ownership and behavior remain in the Consflow mapping files.
