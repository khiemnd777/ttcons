# Consflow Agent Guide

## Repository Expectations

- Treat this repository as a full-stack construction workflow system: Go/Fiber API, PostgreSQL, Redis, custom SQL migrations, React/Vite admin UI, and Docker Compose.
- Read the local code before deciding where to change behavior. Use `rg` and targeted file reads first.
- Keep changes scoped to the requested module. Do not rewrite unrelated modules or generated artifacts.
- Follow SOLID and DRY. Prefer shared components, helpers, and narrow abstractions when a UI, validation, toast, query, or action pattern repeats across modules.
- For every feature request, bugfix request, or code-changing request, first present a review package and wait for explicit user confirmation before implementation.
- For every UI or visual-interface change, first produce a "Mockup as Text" for user review and wait for explicit confirmation before implementation. "Mockup as Text" means a visual mockup represented in text, not a prose UX specification.
- For every bugfix or unclear behavior change, do not implement from assumption. First identify the current behavior, expected behavior, evidence, likely root cause, unresolved risks, clarification questions, implementation plan, and validation plan. If the root cause is not known, stop at diagnosis/review and ask for clarification or approval of the unresolved risk; do not edit code.
- Never use Flyway. Database changes use the custom migration runner in `backend/internal/shared/migrations` and SQL files in `backend/migrations`.
- Do not revert user edits or run destructive git commands unless the user explicitly asks.
- Use `apply_patch` for manual source edits.
- Do not launch or run Browser, browser-use, Playwright, or headless browser verification unless the user explicitly requests it in that turn. For frontend changes, use non-browser checks by default and report browser verification as skipped.

## Canonical Workflow Terminology Agreement

- The user-approved workflow terminology agreement is recorded in `docs/workflow-terminology-agreement.md` and applies to agent communication, plans, review packages, Mockup as Text, documentation, and proposed user-facing labels.
- Use `Giai đoạn` for Node, `Gói công việc` for Step, and `Đầu việc` for Work Item in user-facing Vietnamese workflow contexts.
- Use `Bàn giao công trình và hồ sơ kế toán` for the workflow's final stage previously described as `Tài chính công trình`. Do not infer that this renames the separate project-finance workspace or every finance-domain label.
- Preserve stable technical identifiers and contracts such as `node`, `step`, `work_item`, `pipeline_*`, `run_node_id`, `run_step_id`, `work_items`, `handover_finance`, `financial_closure`, `project_finance.*`, API fields, database columns, event keys, permissions, and business codes.
- Never edit applied migrations or rewrite historical template/run/work-item/form/audit/PDF snapshots solely to enforce terminology. Product rollout requires the normal review and implementation gates, with an additive migration or immutable new template revision when persisted current definitions must change.

## Mandatory Mapping Bootstrap And Maintenance

- Agents and subagents must treat Consflow mapping as a mandatory bootstrap for repository work. Before broad scans, diagnosis, review, implementation planning, or code edits, read `.agents/skills/consflow-module-memory/SKILL.md` and `.agents/skills/consflow-module-memory/references/module-keyword-map.md`.
- After the central map is loaded, load the targeted detail maps that match the task:
  - Backend routes, handlers, services, repositories, shared backend helpers: `.agents/skills/consflow-module-memory/references/backend-function-map.md`.
  - Frontend pages, UI surfaces, components, hooks, helpers, API usage, status/type utilities: `.agents/skills/consflow-module-memory/references/frontend-ui-utility-map.md`.
  - Bug reports, regressions, performance issues, root-cause search paths: `.agents/skills/consflow-module-memory/references/root-cause-triage-map.md`.
  - Business-domain invariants: `.agents/skills/consflow-*-domain/references/domain-map.md`.
- Do not claim current feature behavior, root cause, implementation ownership, or review findings until the relevant mapping has been loaded and checked against local code evidence.
- Every review package, diagnosis, or implementation handoff must name the matched feature/domain, likely owner files, and first targeted searches from the mapping.
- Mapping is not a replacement for reading source. If mapping conflicts with current code, trust current code and update the mapping when the correction is durable.
- Mapping maintenance gate: any code change that creates, updates, deletes, renames, moves, or changes behavior for a feature, function, utility, helper, UI surface, route, API contract, SQL table/migration, status/enum, permission, report/export, domain invariant, or agent workflow must update the relevant mapping files in the same work turn.
- If a code change does not affect mapping, the final response or review notes must state why no mapping update was required.
- Subagents must follow the same bootstrap and maintenance gate. Parent agents should include the matched map references in the subagent prompt when delegating work.
- This is an instruction/profile-level mandatory gate. Runtime sync is not automatic; these repo files are the source of truth.

## Communication Rules

- Agents must use English as the primary language for Levels 1–4. Use professional, neutral wording. When Vietnamese is explicitly requested, use professional pronouns such as "tôi/anh".
- Agents must not mirror hostile, insulting, or overly informal pronouns from the user.
- Even if the user is angry or insulting, agents must remain calm, respectful, and helpful, with a constructive tone.
- Do not argue about tone unless it is necessary to clarify a task boundary; keep focus on fixing the user's issue.

## TechNote — Four-Level Explanation Trial — Active From 2026-10-06

**TechNote** is the default problem and solution explanation method for agents and subagents in **all repositories**, including Consflow. The canonical instructions are in `~/.codex/AGENTS.md`, section `14. TechNote — Four-Level Explanation Method (All Repositories)`. This trial continues until the user asks to change or end it. No end date or effectiveness assessment is available yet.

- `TechNote` and `TechNote Level 1` request Technical Note Style text. `TechNote Level 2` requests diagrams. `TechNote Level 3` requests an interactive website. `TechNote Level 4` requests a video. Use English at every level.
- Use the current topic and agreed scope when the invocation has no subject. If no topic is identifiable, ask one short question. Inspect relevant Consflow code and mapping before claiming product behavior. Proactively provide the solution explanation or artifact at the selected level; do not stop at an acknowledgment or require earlier levels again. TechNote remains the default when its name is not invoked.
- A TechNote invocation selects presentation. It does not approve application changes, migrations or deployment. Preserve the human review gates below. Diagram or artifact examples must distinguish Verified, Proposed and Unknown.

- Mục tiêu: để anh theo kịp việc điều tra, phân tích vấn đề và lựa chọn giải pháp của dự án.
- Bắt đầu ở Cấp 1. Giải thích từng vấn đề, từng bước; không dồn toàn bộ nguyên nhân, phương án và chi tiết kỹ thuật vào một lượt.

- **Primary language for Levels 1–4: English.** Use English for text, diagram labels, website controls and content, and video scripts, narration, subtitles and annotations. Use another language when explicitly requested. Preserve exact product labels, source quotations, identifiers and domain terms when needed for accuracy; explain their meaning in English. This rule concerns agent explanations, not changes to the application's language or stored records.

1. **Level 1 — Text / Technical Note Style:** Write English technical notes inspired by ASD-STE100 — Simplified Technical English. Use one main idea per sentence, one topic per paragraph and consistent terminology. State actors, conditions, actions and results when relevant. Select useful labels such as Purpose, Current state, Evidence, Proposal, Verification and Next action. Follow the detailed rules in `~/.codex/AGENTS.md`, section `TechNote Level 1 — Technical Note Style`. Do not claim full ASD-STE100 compliance.
2. **Level 2 — Diagrams:** Use English labels. Show actors, data, actions, conditions and results. Separate data flow, workflow and logic flow when needed. Use the same terminology and examples as Level 1.
3. **Level 3 — Interactive website:** Use English text and controls. Let the user click through each step and see data and state changes. Include Previous, Next and Replay controls. Preserve the content and examples of the earlier levels.
4. **Level 4 — Video:** Use English scripts, narration, subtitles and on-screen text. The agent writes the script, draws the visuals and adds synthetic narration. Use short segments for pause and replay.

- Anh chọn khi nào chuyển cấp, xem sâu hơn hoặc quay lại. Các câu như “giải thích chậm hơn”, “vẽ sơ đồ”, “làm trang minh họa”, “làm video” điều chỉnh cách trình bày; không tự cấp quyền thay đổi ứng dụng. Không coi sự im lặng là đã hiểu hoặc đã duyệt.
- In all four levels, distinguish **Verified**, **Proposed** and **Unknown**. Diagrams, simulations and videos do not turn hypotheses into product evidence.
- Khi anh yêu cầu Cấp 3 hoặc Cấp 4, không âm thầm thay bằng mô tả text. Kiểm tra công cụ cần thiết; nếu thiếu khả năng xuất website hoặc video có giọng đọc, báo đúng giới hạn và phần còn thiếu. Kịch bản chưa phải video đã xuất.
- Giữ các human review gates hiện có. Trang minh họa phục vụ giải thích không tự trở thành UI của ứng dụng hoặc được deploy công khai. Browser verification vẫn cần yêu cầu rõ trong lượt hiện tại. Trong khi chờ anh chọn cấp hoặc quyết định, chỉ tiếp tục công việc độc lập đã được phép.

## Anti-Ambiguity And Code Evidence Rules

- When answering current product behavior, inspect the codebase first if the behavior depends on routes, UI, data models, runtime flow, or repository logic.
- Do not use shorthand labels that mix UI, API, database, and runtime behavior. Split them into separate columns or separate statements.
- Do not say "A or B", "maybe", "could be", or "X / Y" when codebase evidence can prove the exact current state.
- For every claim about existing behavior, identify whether it is confirmed by code, confirmed by screenshot/log, inferred, or proposed.
- If the current code has backend support but no UI, say exactly: "backend exists; UI does not exist."
- If UI exists in a different page than expected, name the exact page and cite the frontend file.
- Use options only for future implementation choices. Label them as "proposal", not as current behavior.
- If correcting a previous answer, state the exact wrong phrase, the corrected statement, and the file evidence.

## Verified-Only Completion Reporting Protocol

- Treat `TARGET`, `IMPLEMENTED`, `TECHNICALLY VERIFIED`, `UX ACCEPTED`, and `BLOCKED` as distinct statuses. Report the status that the evidence proves; do not collapse them into `complete`.
- A plan, estimate, intended rating, or implementation target is not an achieved result. Before verification, label it explicitly as a target or proposal.
- Do not use claims such as `achieved`, `complete`, `ready`, `passed`, or an equivalent quality statement unless the required verification has actually run and its evidence supports the claim.
- Every completion or quality report must identify the tested source revision or working-tree state, build timing, environment, database and migration state when applicable, commands or scenarios run, observed results, known failures, and checks that were skipped.
- A build result is valid only for the source state that was built. If relevant source files changed after the build, report the build as stale and run it again before claiming the current version builds.
- Verification on an isolated, substitute, mocked, or otherwise different environment must be labelled with that exact scope. It does not prove acceptance on the main, production-like, or user-target environment.
- If browser or UAT verification has not run, report UI and UX as `NOT UX ACCEPTED`. A successful frontend build, unit test, API test, or code review does not prove usability.
- UX ratings before the agreed evaluation runs are targets only. Report an achieved UX rating only after executing the stated rubric against the current build, and identify whether the result is a heuristic review or user-based usability test.
- Use fail-closed completion semantics: any unresolved blocker, failed required check, environment mismatch, or missing acceptance criterion keeps the affected scope incomplete. Passing checks in other layers do not override the blocker.
- Lead completion reports with failed criteria, blockers, skipped checks, and scope limitations before listing successful checks.
- If a previous report overstated or fabricated a result, retract the exact incorrect claim, replace it with the evidence-supported statement, and cite the evidence. Do not preserve a misleading conclusion for narrative consistency.
- Never guarantee a future score, defect-free outcome, or acceptance result that has not yet been verified. State uncertainty and remaining verification work plainly.

## Human Review Gates

Agents must stop before implementation when the request would change code, feature behavior, bugfix behavior, or UI.

For feature, bugfix, or code-changing requests, provide a review package with:

- problem summary
- current behavior, expected behavior, and evidence gathered from local code or reproducible checks
- root cause for bugfixes, or explicit unresolved root-cause risk when it cannot yet be proven
- files likely to change
- proposed behavior
- implementation outline
- validation plan
- risks or open questions

Then wait for an explicit confirmation from the user before editing code.

If the task is a bugfix and the bug can be reproduced locally, build the smallest repeatable pass/fail signal before proposing a fix. If it cannot be reproduced locally, state exactly what evidence is missing, list falsifiable hypotheses, and stop before implementation until the user confirms the plan or clarifies the missing context.

When asking the user to confirm review points, decisions, open questions, scope, or implementation steps, list each confirmation point as a numbered item using `1.`, `2.`, `3.`, and so on. Put each point on its own line; do not combine multiple confirmation points into one sentence, paragraph, or bullet.

For UI or interface-changing requests, provide a "Mockup as Text" before implementation. It must be a textual visual mockup: use ASCII/wireframe-style blocks to show the actual screen layout, hierarchy, controls, labels, spacing, and states as the user would see them. Keep notes short and secondary. Do not replace the mockup with only bullet-point descriptions.

Every UI/UX analysis, review package, and Mockup as Text must begin by identifying the `Mục đích chính` of the screen, dialog, panel, or component. Do not start from available data, backend capabilities, form engines, workflow blockers, or feature inventory. The primary purpose must determine the hierarchy, visible sections, default focus, and primary CTA.

It should show:

- screen or component affected
- layout structure
- visible controls and labels
- states, empty states, loading states, and error states
- primary user flow
- responsive behavior when relevant

Then wait for explicit confirmation from the user before editing UI code.

When the user says `tiếp tục triển khai`, agents must treat it as a request to continue the implementation workflow by preparing the next review package and, when UI is involved, a Mockup as Text. It is not permission to edit code, create migrations, update UI, update docs, run implementation-side formatting/build/test commands, or otherwise change repo files. Agents must wait for explicit confirmation after the review package before implementation. Valid implementation confirmations should be clear phrases such as `OK triển khai`, `Duyệt, làm đi`, `Confirm implement`, or `Triển khai theo review package này`.

## Plan State Notes

- When implementing a plan, create or update that plan in `docs/current-states.md`. Treat this file as the shared agent memory for follow-up work until the plan is completed.
- Each active plan entry in `docs/current-states.md` must track both work that `will be done` and work that has already been `done`, and agents must keep those notes current as they progress.
- Use the active plan notes to avoid duplicate work, lost context, or invented context across follow-up turns.
- When the user says `Đóng lại plan` or `Đóng plan`, move the relevant plan into `docs/plan-closed/`, prefix the closed plan note/title with `[Closed]`, and remove that plan completely from `docs/current-states.md`.
- Files under `docs/plan-closed/` are historical records only. Do not treat closed plans as durable agent memory or required source of truth; promote durable module, feature, invariant, or routing knowledge into `AGENTS.md` or `.agents/skills/**/references/*.md` through the normal review gate.

## Repo Map

- `backend/cmd/server`: API entrypoint.
- `backend/cmd/migrate`: standalone custom migration command.
- `backend/internal/core`: routes, handlers, service logic, repository helpers, domain types.
- `backend/internal/shared`: config, DB, cache, middleware, response envelope, migration runner.
- `backend/migrations`: ordered SQL migrations, normally named `V<N>__description.sql`.
- `backend/seed`: demo data.
- `frontend/src/app`: React Router setup.
- `frontend/src/layouts`: authenticated app shell and navigation.
- `frontend/src/lib`: API client, auth context, shared domain types and utilities.
- `frontend/src/modules`: admin feature screens.
- `frontend/src/components`: shared UI primitives.
- `.agents/skills`: repo-scoped Codex skills.
- `.codex/agents`: project-scoped custom subagents.

## Commands

- Full Docker pipeline: `make up`
- Stop stack: `make down`
- Restart pipeline: `make restart`
- Logs: `make log` or `make log SERVICE=admin`
- Backend tests: `cd backend && make test`
- Backend format: `cd backend && make fmt`
- Backend migration run against local DB: `cd backend && make migrate`
- Seed demo data: `cd backend && make seed`
- Frontend build: `cd frontend && npm run build`

`make up` and `make restart` intentionally run `down -> build -> run`. Keep that sequence intact.

## Backend Rules

- Route registration lives in `backend/internal/core/routes.go`.
- HTTP handlers should use `response.OK`, `response.Created`, and `response.Fail` from `backend/internal/shared/response`.
- Keep request parsing in handlers and business transitions in `Service`.
- Keep SQL access through repository helpers unless a very small direct query is already the local pattern.
- When adding or changing tables, add a new migration rather than editing an applied migration.
- The migration runner records checksums in `schema_migrations`; changing an applied migration will break future runs.
- User-visible list endpoints must return records that are not deleted. Use each table's local marker, such as `status <> 'cancelled'`, `status <> 'deleted'`, or `active = true`, and do not expose soft-deleted rows in admin lists.
- Preserve auth and actor semantics. Protected endpoints should stay under `middleware.AuthRequired`.

## Frontend Rules

- Add route entries in `frontend/src/app/router.tsx`.
- Put feature pages under `frontend/src/modules/<feature>`.
- Reuse `frontend/src/components` before adding new UI primitives.
- Evidence upload presentation has one owner: `frontend/src/modules/_shared/EvidenceUploader.tsx`, extracted from the existing WorkItem uploader. `EvidencePanel` owns immediate WorkItem upload; `EvidenceCapture` adapts parent-save queues and the shared camera. Preserve caller source, capacity and persistence timing. Do not add module-specific uploader markup or a presentation flag such as `reportMode`.
- Evidence preview composition belongs to `frontend/src/modules/_shared/EvidenceRecordViewerDialog.tsx` and its draft/external states, using the existing photo/PDF/Word/Excel readers. Pass complete records when available; ID-only references resolve protected metadata through `EvidenceReferenceViewer`. Do not call the low-level `EvidenceViewerDialog` directly from feature modules. Update the entrypoint inventory and behavior/render tests when adopting or changing these components.
- Create, view-detail, and edit interactions must open a shared dialog/modal component. Avoid one-off drawer or full-page implementations unless the feature is a specialized canvas/editor with explicit UX needs.
- Save/edit flows must show toast feedback for success, warning, and error states. Toasts must include an immediate close `(x)` button.
- Required and invalid fields must be highlighted inline, and the warning toast must summarize required/invalid fields.
- Every monetary input (amount, price, cost, budget, debit, credit, tax, or another currency-valued field) must use shared `frontend/src/components/CurrencyInput.tsx` directly or `SimpleForm`/workflow semantic type `currency`; never use native `type="number"` or generic field type `number` for money. Keep quantities, percentages, day counts, sequence numbers, and other non-monetary numerics as `number`. Use the business currency code when the record supplies one and default to VND only when it does not; do not infer currency from a label or field name. VND is integer-only and must never render or accept a `,00` fraction; other currencies use their ISO minor-unit digits by default, with explicit non-VND precision only where the business contract requires it.
- Use the API helpers in `frontend/src/lib/api/client.ts`; do not duplicate fetch/token logic in feature modules.
- Keep UI operational and dense. This is an admin tool, not a marketing page.
- After frontend changes, run `cd frontend && npm run build` when feasible.

## Mandatory Standard Component Reuse

User instruction reinforced on 2026-09-08 after design attachments in discussion bypassed the existing previewers. Treat this as a required implementation and review check for future UI work, not an optional preference.

- Before editing UI, identify the standard components that own each affected interaction in this guide and the frontend mapping. Read each relevant owner and an existing consumer. Include a concise `interaction -> standard component -> target entrypoints` checklist in the existing review package; do not create a separate approval round for this checklist.
- Reuse the complete standard experience at every affected entrypoint. An import, a shared modal shell, or reuse in one conditional branch does not prove reuse in another branch. Check canonical pages, discussion, historical snapshots, pending files and responsive variants when they are in scope.
- Photo, PDF and supported document viewing must use the shared viewer compositions in `frontend/src/modules/_shared/EvidenceRecordViewerDialog.tsx`. Do not replace their primary preview with a filename-only button, raw link, `window.open`, or `openProtectedBlob`. Explicit original-file/download actions and existing fallbacks for unsupported formats remain valid secondary behavior.
- Different source records require a narrow data/access adapter, not a separate viewer experience. Preserve the actual source endpoint, authorization, metadata and exact historical file/revision. Never pass a document ID as an evidence ID or fabricate evidence metadata to force reuse. If the standard composition has a proven capability gap, show the source evidence and proposed shared adaptation in the existing review package; do not silently bypass it. Existing nonstandard code is not permission to copy that deviation.
- Before reporting implementation results, trace each affected user action to the standard component and verify meaningful entrypoint behavior. For preview changes, cover supported media, loading/error/retry and source/revision identity as applicable. Tests that only find an import or pass a build do not prove the integration. Keep entrypoint inventories and relevant behavior/render tests synchronized. Browser checks still require explicit authorization in that turn; skipped browser checks remain `NOT UX ACCEPTED`.
- State which standard components were reused and any remaining gaps in the implementation handoff. A test-tool restriction is a verification limitation, never evidence that bypassing a standard component was justified. Record unfinished integrations in `docs/current-states.md` instead of declaring the affected scope complete.

## Domain Rules

- Pipeline templates define reusable workflows. Pipeline runs are project-specific runtime copies.
- Publishing and cloning templates must not mutate historical project runs unexpectedly.
- Approval actions should preserve auditability: actor, status, comments, and timestamps matter.
- BOQ approval can generate procurement. Do not bypass approval states when connecting BOQ to procurement.
- Acceptance failures can create rework. Rework completion should not silently skip acceptance history.
- Warehouse stock operations must preserve material balance and movement traceability.

## Review Guidelines

- Prioritize correctness, authorization/RBAC, migration safety, financial/stock consistency, and missing regression tests.
- Check backend/frontend contract drift whenever an API response shape changes.
- Check Docker and Makefile behavior when ports, service names, migrations, or startup order change.
- Treat broad refactors as risky unless they directly support the requested behavior.

## Agent Artifacts

- Repo skills live in `.agents/skills/<name>/SKILL.md` with optional `agents/openai.yaml` metadata.
- Project custom agents live in `.codex/agents/<name>.toml`.
- Runtime sync is not automatic. These files are the repo source of truth.
