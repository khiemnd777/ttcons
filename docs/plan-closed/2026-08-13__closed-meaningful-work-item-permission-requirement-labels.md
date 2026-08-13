# [Closed] Meaningful Work Item Permission Requirement Labels

## Will be done

- Keep the raw work-item `required_permission` key as the backend authorization source of truth while enriching work-item detail with the permission name and description stored in RBAC.
- Replace the user-facing `planning.edit` enum in `Điều kiện bắt buộc` with meaningful Vietnamese wording and a safe generic fallback that never exposes an unknown key.
- Standardize existing and seeded `planning.edit` permission metadata through a new custom migration without changing role grants or historical workflow snapshots.
- Add focused backend/frontend regression coverage, update Consflow central/detail/domain mappings, and run non-browser technical verification. Browser/UAT remains skipped unless explicitly requested.

## Done

- Confirmed from the supplied Edge screenshot and current source that `WorkItemDetailDialog` interpolates `evidence_requirement_json.required_permission` directly into both the requirement description and handling guidance.
- Confirmed `permissions` already owns the approved name `Cập nhật kế hoạch` and description `Cập nhật nội dung kế hoạch được phân công`, but V68 used `ON CONFLICT DO NOTHING` while seed data still says `Sửa planning`, so existing environments are not repaired consistently.
- Confirmed the mutation path must continue authorizing with the immutable raw permission key; only work-item detail presentation metadata and user-facing text are in scope.
- Produced the review package and Mockup as Text, then received explicit `OK triển khai` approval.
- Added V96 and destructive-seed parity for the canonical `planning.edit` name/description. The migration is idempotent and changes no `role_permissions`, users, project memberships or runtime workflow snapshots.
- Enriched `GetWorkItemDetail` from the work item's snapshotted raw permission key with nullable catalog `required_permission_name` / `required_permission_description`; backend mutation authorization still reads the raw nested key.
- Replaced direct enum interpolation with `Yêu cầu: Cập nhật kế hoạch.` and explanatory guidance. Missing/unknown catalog metadata now shows generic administrator guidance and never exposes the raw key.
- Added focused migration, repository-response, presentation, fallback and source-wiring regressions; updated central/frontend/backend plus Admin Governance and Workflow Orchestration mappings.
- Technically verified the current working tree with all 92 frontend tests, TypeScript/Vite production build, 121.5 KiB gzip entry/111-chunk bundle budget, full `go test ./...`, 96-migration manifest validation, focused V96 and work-item-detail PostgreSQL regressions in an isolated schema, and `git diff --check`. The isolated schema was removed. V96 was not applied to the normal localhost database and the runtime was not restarted. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
