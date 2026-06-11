# Consflow Admin UI Expectations

Consflow is an operations admin system for construction workflows.

Prefer:

- Dense but readable layouts.
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
