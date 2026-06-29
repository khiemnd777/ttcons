# Consflow Design Guide

Consflow is a construction operations admin system. The UI must help users decide what to do next, not simply expose every available feature at the same visual weight.

## UX Hierarchy Rules

- Always start UI/UX analysis by identifying the `Mục đích chính` of the screen, dialog, panel, or component.
- The primary purpose must be stated before discussing layout, sections, components, fields, tabs, CTAs, or implementation details.
- Do not let available data, backend capabilities, form engines, workflow blockers, or feature inventory define the UI hierarchy before the primary purpose is clear.
- Every screen, modal, and major panel must have one clear primary task.
- Primary, secondary, and supporting content must be visually distinct.
- A region should have one primary call to action. Secondary actions must be quieter, grouped, or moved behind menus when they are not part of the current task.
- Do not place unrelated domain workflows in the same vertical flow. Split them into tabs, collapsible sections, a detail page, or a separate dialog.
- If a work item has a specific title such as `Upload ảnh hiện trạng`, the visible primary flow must match that job first. Related workflows such as checklist, material snapshot, acceptance, PDF, signature, evidence, and activity must not compete as equal sections.
- Status badges, metadata chips, cards, and metrics must support the user's next decision. Do not show all available labels just because data exists.
- Validation and gating must point directly to the fields or missing evidence that block submission.
- Dense admin UI is acceptable only when it remains scannable. Density must not remove hierarchy.

## Work Item Detail Rules

- A work item detail surface should lead with: current status, required next action, owner, due date, blocking validation, required evidence, and the active form/task.
- Put supporting history, activity, generated documents, signatures, and cross-domain records after the primary flow or behind tabs/collapsible sections.
- Avoid turning a modal into a full project workspace. If the content needs multiple domain workflows, use a full page or a tabbed detail layout instead of one long modal.
- Footer actions must reflect the current state. Disabled or unavailable actions should explain the blocker close to the blocking field, not only in a generic banner.

## Review Gate Expectations

- Before implementing UI changes, provide a Mockup as Text that shows hierarchy, primary action, secondary actions, empty/loading/error states, and responsive behavior when relevant.
- The review package and mockup must start with `Mục đích chính`, then derive the primary flow and CTA from that purpose.
- The mockup must make it obvious what the user should do first.
- If the mockup shows many same-weight cards, badges, buttons, or sections, revise it before implementation.
