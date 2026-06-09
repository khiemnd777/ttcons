# Consflow Admin UI Expectations

Consflow is an operations admin system for construction workflows.

Prefer:

- Dense but readable layouts.
- Clear table, form, status, and action patterns.
- Quiet colors with status accents.
- Consistent route and navigation labels.
- Existing shared components such as `DataTable`, `PageHeader`, `StatusBadge`, `FormDrawer`, and `EmptyState`.

Avoid:

- Marketing-style hero sections.
- Decorative cards around entire page sections.
- Duplicated fetch or auth logic.
- UI text that explains obvious controls.
- Layouts where long labels overflow buttons, tabs, or table cells.

For workflow-heavy modules, expose:

- current status
- required next action
- actor or owner
- date/time context
- audit or history where available
