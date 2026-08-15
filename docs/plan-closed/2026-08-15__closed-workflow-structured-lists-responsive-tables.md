# [Closed] Workflow Structured Lists As Responsive Tables

## Will be done

- Replace the repeated card editor used by every workflow-form `structured_list` field with one schema-driven desktop table and narrow-screen row-card presentation.
- Preserve each list's own `validation_json.item_fields`, typed inputs, checklist status rules, evidence references, payload shape, workflow gates and published-version immutability.
- Make `Thêm dòng` append one inline-editable row and focus its first available control; keep delete, required-cell highlighting, empty and read-only states accessible.
- Add focused frontend regression coverage, update central/frontend/workflow mapping and run the complete non-browser frontend tests/build.
- Query the local runtime for direct project/work-item routes covering every affected form so the user can perform visual acceptance manually.
- Make desktop horizontal overflow discoverable with a stable, styled scrollbar and an overflow-only drag hint.
- Move the editable-row Actions column from the far right to the first table column, keep its visual header blank and retain a screen-reader-only label.
- Require an explicit shared confirmation dialog before any desktop or mobile structured-list row deletion; cancellation must preserve the draft row.
- Constrain every desktop structured-list scroll viewport to the available workflow-form column width so a wide table overflows inside that viewport instead of expanding and being clipped by an ancestor.

## Done

- Audited the current source and local PostgreSQL runtime: 29 `structured_list` fields across 21 published workflow-form templates currently render through `WorkflowFormPanel.tsx`; 26 use the generic editor and three `checklist_items` fields use the checklist variant.
- Confirmed the shared behavior can stay schema-driven because each field supplies its own column key, label, input type, required flag and options through `validation_json.item_fields`; no backend, API, migration or stored answer change is required.
- Produced the review package and Mockup as Text, then received explicit `OK triển khai` approval for the all-structured-list scope and post-build manual verification links.
- Replaced both generic and checklist row-card editors with one schema-driven responsive table shell. Desktop keeps inline controls in dynamic columns with contained horizontal scrolling; narrow screens use labelled row cards; read-only/submitted forms render values without add/delete actions.
- Kept checklist statistics and done/not-applicable validation separate, retained evidence/select/currency/date/number/text/textarea cell renderers, added cell-level required highlighting, and made add-row focus the first available control.
- Added four focused structured-list contracts covering generic/checklist sharing, wide and narrow layouts, inline add/focus, required-cell feedback and read-only presentation; updated the central frontend and workflow-orchestration maps.
- Technically verified source revision `4b39c8d` plus the current working tree with 114/114 frontend tests, TypeScript/Vite production build, bundle budget (121.5 KiB gzip entry; 111 JavaScript chunks under 250 KiB) and `git diff --check`.
- Queried the running local PostgreSQL data and prepared direct routes for all 21 affected form screens covering all 29 structured-list fields; populated submitted examples are preferred for visual table review, with ready/draft examples reserved for add-row interaction.
- Browser/UAT was not run under the repository rule. The implementation is technically verified but remains `NOT UX ACCEPTED` until the user completes the supplied manual visual checks.
- The user visually confirmed that the wide checklist hid later columns without an obvious scrollbar, then approved the follow-up Mockup as Text: a visible overflow-only horizontal rail and the icon-only Actions column first, before `#`.
- Added measured desktop overflow using the table viewport's `scrollWidth`/`clientWidth`, a resize observer, a stable styled Firefox/WebKit scrollbar and a keyboard-focusable labelled scroll region; the drag hint renders only while real overflow exists.
- Moved the editable desktop Actions column, delete cell and assistive header before `#`; the header remains visually blank, read-only tables still omit actions, and mobile cards retain the delete action at the card header.
- Extended the focused contract to five checks covering the visible overflow rail/hint and exact action-first/header-hidden order; updated central/frontend/root-cause/workflow mapping for the durable behavior and triage path.
- Technically verified the final follow-up source with 115/115 frontend tests, TypeScript/Vite production build, bundle budget (121.5 KiB gzip entry; 111 JavaScript chunks under 250 KiB) and `git diff --check`.
- Browser/UAT was not run in this approved implementation turn, so the visible scrollbar and final column order remain `NOT UX ACCEPTED` until the user refreshes the supplied checklist route and confirms them visually.
- The user approved the follow-up delete-confirmation Mockup as Text: `Xóa dòng này?`, row identity context, safe cancel paths and a destructive `Xóa dòng` confirmation across every structured list on desktop and mobile.
- Added one shared pending-row confirmation dialog for desktop table and mobile card delete triggers. The dialog identifies the list, row and first meaningful value; `Hủy`, close, backdrop and Escape preserve the row, while `Xóa dòng` alone invokes the existing client-side draft removal.
- Extended the focused structured-list contract to six checks covering both delete entry points, the shared dialog, safe default focus, row context and explicit confirmation; updated central/frontend/root-cause/workflow mappings for the durable behavior and triage path.
- Technically verified the delete-confirmation follow-up against source revision `4b39c8d` plus the current working tree: 116/116 frontend tests passed, TypeScript/Vite production build passed, bundle budget passed (121.5 KiB gzip entry; 111 JavaScript chunks under 250 KiB), and `git diff --check` passed. Browser/UAT remains `NOT UX ACCEPTED` because no browser run was requested for this turn.
- The user's Edge screenshot reproduced the remaining wide-table defect: later checklist columns were clipped, no horizontal rail/hint appeared and the table could not scroll. Source diagnosis proved the direct workflow-field grid item and structured-list viewport lacked a shrinkable, width-constrained containment chain, so the viewport expanded to the table's min-content width before the process card clipped it.
- Produced the root-cause review package and corrected Mockup as Text; received explicit `OK triển khai` approval for the width-containment fix while preserving existing columns, mobile cards, delete confirmation, payload and workflow behavior.
- Added `min-w-0` to the direct workflow-field grid item and a `min-w-0 w-full max-w-full` containment chain to the structured-list shell and desktop scroll viewport. The table retains its schema-derived minimum width, so wide schemas now produce internal scroll overflow while compact schemas remain unchanged.
- Extended the focused structured-list contract with the complete width-containment chain. The two source assertions that reproduced the defect now pass, as does the focused wide-table regression test.
- Technically verified the scroll-containment fix against source revision `4b39c8d` plus the current working tree: 116/116 frontend tests passed, TypeScript/Vite production build passed, bundle budget passed (121.5 KiB gzip entry; 111 JavaScript chunks under 250 KiB), and `git diff --check` passed. Browser/UAT remains `NOT UX ACCEPTED`; the user must refresh the reproduced Edge route and confirm real pointer/trackpad/scrollbar interaction.
