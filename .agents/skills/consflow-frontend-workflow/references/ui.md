# Consflow Admin UI Expectations

Consflow is an operations admin system for construction workflows.

Prefer:

- Dense but readable layouts.
- UI/UX analysis that starts by stating the `Mục đích chính` of the screen, dialog, panel, or component.
- Clear primary task hierarchy for every screen, modal, and major panel.
- Clear table, form, status, and action patterns.
- Quiet colors with status accents.
- Consistent route and navigation labels.
- Existing shared components such as `DataTable`, `PageHeader`, `StatusBadge`, `FormDrawer`, dialog/detail components, form components, toast provider, and `EmptyState`.
- Shared dialogs for create, view-detail, and edit actions.
- Save/edit feedback through dismissible success, warning, and error toasts.
- Inline required/error field highlighting with a toast summary of invalid fields.
- API-backed lists that exclude deleted/cancelled/inactive records before data reaches the table.

Avoid:

- Marketing-style hero sections.
- Decorative cards around entire page sections.
- Starting UI design from available data, backend capabilities, form engines, workflow blockers, or feature inventory before identifying the user's primary purpose.
- Same-weight sections, cards, badges, buttons, and labels that make primary and secondary work indistinguishable.
- Long modals that combine unrelated domain workflows such as checklist, evidence, material snapshot, warehouse return, acceptance, PDF, signature, and activity in one vertical flow.
- Footer action bars with multiple competing primary actions.
- Duplicated fetch or auth logic.
- Duplicated form validation, toast handling, dialog scaffolding, or row action controls.
- UI text that explains obvious controls.
- Layouts where long labels overflow buttons, tabs, or table cells.
- Showing deleted, cancelled, inactive, or soft-deleted records in user-visible admin lists unless the screen explicitly audits deleted data.

For workflow-heavy modules, expose:

- current status
- required next action
- actor or owner
- date/time context
- audit or history where available

For work item detail surfaces:

- First state the work item's `Mục đích chính`, then choose the default panel, visible hierarchy, and CTA from that purpose.
- Lead with the task the title promises. For example, `Upload ảnh hiện trạng` should make uploading or satisfying required evidence the primary flow.
- Show one primary CTA for the current state. Group or demote secondary actions such as review, approval, blocker reporting, navigation, PDF, and signatures unless they are the current required next step.
- Keep blocking validation close to the blocked field or evidence requirement, not only in a generic banner.
- Move supporting workflows into tabs, collapsible sections, or a full detail page when they compete with the primary task.
- If a modal needs multiple domain workflows to be visible at once, it is probably no longer a modal; redesign it as a page or tabbed workspace.
