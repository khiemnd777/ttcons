# Project Manager role presentation

Approved 2026-10-03 after client feedback: the canonical internal role `pm` displays as **Quản lý dự án**. The distinct `deputy_manager` role continues to display **Phó Quản Lý**.

Personal names mentioned during analysis are examples, not role rules. Authority comes from assigned roles and resource scope; no person is automatically a Project Manager or Deputy Manager because of their name. This display-name change requires no account conversion.

## Scope

- V149 updates only the current system PM catalog display name and adds the immutable SQL `consflow_role_label_key` compatibility function. Seed uses the same display name. Role IDs/keys, permissions, assignments and `pm_*` API fields are unchanged.
- Stored PM / Project Manager labels and current Quản lý dự án labels refer to the same PM in workflow/member responsibility checks. Unrelated SQL labels retain their original values/comparison behavior. Deputy Manager is never a PM alias.
- Existing pipeline selectors keep raw `PM` values with the current display label. Member selectors deduplicate display aliases and retain saved raw values. Shared account/role presentation plus current project/planning role summaries use `lib/utils/roles.ts`.
- Sample role/actor resolution supports old PM-labelled workflows after catalog rename, retaining ambiguous-role and external-role guards. Workflow Excel export resolves old/current labels back to canonical `pm`.
- Role mentions retain project/active-member checks and resolve old/new PM labels. Mention options display the current name with compatible IDs. Current startup, Operations, Contracts and design handoff guidance names Quản lý dự án.
- Existing shared tables, badges, dialogs, checkboxes and NodeInspector MultiSelect are reused. No layout redesign, workflow state transition or new permission is introduced.

## Historical data

V1–V148 migrations and persisted template/run/work-item/form/report/audit/PDF snapshots are untouched. Arbitrary user-authored text and names are not bulk rewritten. Current presentation can localize a stored role value without modifying it.

## Verification

Implementation checks and exact environment/results are archived in `docs/plan-closed/2026-10-03__closed-project-manager-role-label.md`. Scoped main-local browser UAT and the already-applied V149 were verified on 2026-10-03; scenarios and limits are recorded in `docs/uat/project-manager-role-label-local-2026-10-03.md`. A pre-existing legacy startup CTA error remains outside the label scope. Production rollout and full client UX acceptance are not established by these checks.
