# Current States

## [Active] Global Project Progress Board

### Will be done

- Optional browser/UAT walkthrough when the user explicitly requests browser verification.

### Done

- Review package and Mockup as Text approved by the user.
- Matched Project Planning, Workflow Orchestration, global navigation, project-list, runtime-node and activity-stream owners through the mandatory mapping bootstrap.
- Confirmed no database migration is required.
- Added the permission-gated global `/progress` route and `Tiến trình` AppShell item directly below `Dashboard`; `/projects` remains unchanged.
- Added the read-only responsive board with search/PM/status filters, stage columns, project attention badges, loading/error/empty states, manual refresh, explicit connection state and 60-second disconnected polling.
- Added scoped `GET /projects/progress-board` and invalidation-only `GET /projects/progress-board/stream` before `/projects/:id`, plus the production Nginx no-buffer SSE whitelist.
- Derived normalized presentation stages from each project's snapshotted current pipeline run/node without mutating templates, runs, work items or approvals.
- Added focused stage/stream tests and updated central, frontend, backend, Project Planning and Workflow Orchestration maps.
- Passed `go test ./...`, `npm run build`, `docker compose config`, a read-only PostgreSQL snapshot query against current local data, and `git diff --check`; browser verification was not run by policy.
