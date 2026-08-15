# [Closed] Audited Out-of-Sequence Work-Item Start

## Scope closed

- Separate start readiness from completion requirements.
- Allow an authorized actor to start deterministic future-node work with explicit confirmation and a mandatory reason.
- Preserve the runtime current-node pointer and all normal permission, evidence, form, signature, approval, automation, blocker and completion gates.
- Persist the early-start decision and prerequisite snapshot in work-item activity and system audit history.
- Present the approved `Điều kiện để bắt đầu` panel, confirmation dialog and early-execution banner.

## Delivered

- Added the authoritative `start_readiness` detail contract with deterministic-route evaluation, predecessor work-item, transition, planned-time, approval and automation rows, permission-aware links, target blocker enforcement and a stale-safe snapshot token.
- Added the audited `POST /work-items/:id/start` override. Reason and matching snapshot token are mandatory; unresolved conditional routes, invalid target states, target blockers and stale snapshots fail closed.
- Kept `completion_requirements` unchanged as the exclusive completion-gate contract. Early start updates only the target work item/run step and never advances `pipeline_runs.current_node_id` or mutates the template/runtime definition.
- Stored actor, time, reason, current/target node and unmet-requirement snapshot in activity and audit metadata. The activity timeline displays the reason under `Trao đổi & lịch sử`; the audit description and detail retain the same reason and snapshot.
- Added the default-open `Điều kiện để bắt đầu` panel above `Điều kiện bắt buộc`, prerequisite links, retained future warning, server-gated `Bắt đầu`, shared mandatory-reason confirmation and the early-execution banner.
- Added focused route/action tests, a PostgreSQL-backed end-to-end regression and frontend contract coverage.
- Updated central, backend, frontend, root-cause and workflow-orchestration mappings. No migration was required because existing runtime, work-item, activity and audit columns persist the state and trace.

## Verification at closure

- Full backend `go test ./...` passed on revision `a270f26` plus the feature working tree.
- Focused PostgreSQL early-start integration coverage passed.
- All 108 frontend tests and the TypeScript/Vite production build passed.
- Bundle budget passed: 121.5 KiB gzip entry; largest feature chunk 44.8 KiB gzip.
- `git diff --check` passed.
- The user reported that the early-start confirmation and action worked on localhost in Microsoft Edge. Agent-run browser/UAT was not requested, so the broader UI remains `NOT UX ACCEPTED`.

## Durable references

- Feature routing and ownership are maintained in `.agents/skills/consflow-module-memory/references/`.
- Workflow invariants are maintained in `.agents/skills/consflow-workflow-orchestration-domain/references/domain-map.md`.
