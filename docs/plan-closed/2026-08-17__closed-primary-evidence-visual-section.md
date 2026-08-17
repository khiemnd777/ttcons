# [Closed] Primary Evidence Visual Section

## Will be done

- Give the work-item primary `EvidencePanel` one clear visual section without nesting another card when the same panel renders inside the supporting `Minh chứng` tab.
- Add an explicit primary/embedded presentation variant and use the primary variant only from `WorkItemPrimarySurface` for evidence-first work items.
- Add focused regression coverage for primary containment and embedded-tab behavior.
- Update the central, frontend, root-cause, and workflow-orchestration maps for the durable presentation contract.
- Run focused/full frontend tests and the current-source frontend build; skip browser/UAT unless explicitly requested.

## Done

- Confirmed from the supplied Edge screenshot and current source that the primary evidence ledger was semantically a `<section>` but lacked a visual card container, while the supporting tab already supplied its own outer card.
- Produced the review package, root-cause evidence, validation plan, and Mockup as Text, then received explicit `OK triển khai` approval.
- Added explicit `primary` and `embedded` presentation variants to `EvidencePanel`.
- Applied `primary` only to evidence-first work items and `embedded` to the supporting `Minh chứng` tab, preserving upload/review behavior and preventing double containment.
- Added focused source-contract regression coverage and updated the central, frontend, root-cause, and workflow-orchestration mapping files.
- Technically verified the final frontend source with the focused work-item layout/navigation suite (10/10), the complete frontend suite (124/124), the TypeScript/Vite production build, and `git diff --check`.
- No backend, database, migration, API, permission, or workflow-state change was included. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.

## Closure

- Closed on 2026-08-17 at the user's request.
- This file is a historical record only. Durable ownership and presentation behavior remain in the Consflow mapping files.
