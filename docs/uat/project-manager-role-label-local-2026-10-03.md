# Local UAT — Quản lý dự án display name — 2026-10-03

Mục đích chính: verify that users can distinguish Project Manager (`pm`, Quản lý dự án) from Deputy Manager (`deputy_manager`, Phó Quản Lý), with existing role values and authority preserved.

## Result and limits

Scoped label and read-access scenarios passed as listed below. A pre-existing legacy startup CTA error was found outside the label-change scope; full client UX acceptance is not claimed. The former real-person assignment mismatch finding is withdrawn: the user clarified that names are examples and do not determine roles. This is agent-executed local browser UAT, not client acceptance or production proof. No new assignment, report receipt, report transition, startup save or workflow definition was submitted in the browser. The user subsequently requested closure and Git delivery of the label plan.

## Source and environment

- Uncommitted working tree on `main`, base HEAD `aef75f0`, current approved PM-label implementation. No application edit during UAT.
- Main local Docker stack: browser `http://localhost:5173`, API `http://localhost:8080/api`, existing PostgreSQL16 data. Backend image `sha256:2749ff2c9b0352940c8a1b19a5f270a664f79f0c63072805b486434c0e74a376`, built 2026-10-03 11:15:45 ICT. Filesystem check found no non-test application Go file modified after that build. Frontend source is bind-mounted into the Vite container.
- V149 already applied 11:15:53 ICT. Database checksum and local/image migration file SHA256 all equal `5c176972b77e8a0c183d72c0dec705420509d84d381726e7102217b6a14d6f0b`. Agent did not rebuild stack or apply migrations during this UAT.
- Browser: Codex in-app browser; current/default desktop plus explicit 1440×1000 desktop and 390×844 mobile viewport. Override reset afterward. UAT ran approximately 11:55–12:00 ICT.
- Mapping: Admin Governance / Project Planning / Workflow Orchestration. Owners: `roles.ts`, RolesPage, StaffPage, ProjectMembersPanel, ProjectStartupWizard, NodeInspectorPanel, `project_startup.go`. Targeted searches: display-name aliases, role choices, startup-readiness and actor scope; relevant maps had already been loaded and were checked against source.

## Observed scenarios

| Scenario | Result | Evidence |
| --- | --- | --- |
| Role catalog search and detail | PASS | Quản lý dự án, key `pm`, 79 assigned permissions; Phó Quản Lý, key `deputy_manager`, 8 permissions. Counts describe current local catalog only. |
| Staff rows and assignment dialog | PASS | Own PM fixture shows Quản lý dự án selected; Deputy choice remains distinct and unselected. Dialog closed without save. |
| PM header / MyWork / profile | PASS | All role summaries show Quản lý dự án; MyWork becomes online after loading. Free-text fixture position `pm` remains original data. |
| Legacy project member | PASS | SQL confirms fixture membership remains `["PM"]`; UI shows Quản lý dự án and Quản lý dự án chính, normalized filter option and checked edit choice, with no duplicate PM option. Edit cancelled. |
| Mobile member view | PASS, scoped | At 390×844, member card and Quản lý dự án chính label are readable, existing mobile cards used. Not a full mobile-device evaluation. |
| Published workflow responsibility | PASS | Rev9 construction planning checklist and read-only detail show Quản lý dự án for legacy PM responsibility; immutable descriptive text still contains PM. No definition save. |
| New startup fixture C | PASS, scoped | Staff-selection wizard opens with new role label. Saving selection, assigning manager and starting pipeline NOT_RUN. |
| Legacy startup fixture A | FAIL, pre-existing | Overview offers Hoàn tất thiết lập, but wizard receives HTTP400 twice; diagnosis below. |
| Deputy workspace and report detail | PASS | Deputy still has zero active memberships and sees Supervisor work and two UAT reports. Submission2 report shows separate existing Deputy and PM read receipts; report recipient remains own PM fixture. Review dialog opened and closed without submitting a new response. |
| Cleanup | PASS | Original hashes/statuses restored for two existing UAT accounts. Temporary local developer disabled and grants removed. Zero active sessions for all three identities; freshly issued PM token returns HTTP401 after revocation. |

Shared interactions observed: existing role/staff dialogs and checkboxes, project member dialog/table/mobile cards/badges, NodeInspector MultiSelect, existing ProjectStartupWizard and WorkReportReviewDialog. No alternative presentation was introduced for UAT.

NOT_RUN in browser: save/reload of member aliases, new workflow editing/publication, sample project generation, Excel import/export, complete startup, new report submit/request-changes/read-receipt/communication mutations. Prior isolated integration/build checks are archived in `docs/plan-closed/2026-10-03__closed-project-manager-role-label.md`; they do not replace these browser checks.

## Findings

**Correction after user clarification:** Cường and other personal names are illustrative. No name-to-role invariant exists; actual assigned roles determine authority. The earlier “Local identity mismatch — Bùi Văn Cường” finding is retracted and is neither a UAT failure nor a blocker. No account conversion is required.

**Pre-existing legacy project startup CTA error (outside this plan).** Reproduction: PM fixture opens UAT Công trình A overview → Hoàn tất thiết lập → error state → Thử lại → same error. GET `/api/projects/37afbfe8-7387-4c2e-b8b7-afe90b8d163a/startup-readiness` returns HTTP400 VALIDATION_ERROR: Công trình này không thuộc luồng điều phối khởi tạo mới. SQL confirms `created_by` NULL, `is_sample=false`; fixture C has a creator and opens the wizard successfully. Root cause: ProjectDetailPage offers startup when `!hasPipeline`, while `projectStartupReadiness` rejects projects without a creator or sample projects. Both predicates exist at HEAD before the display-label change. Proposed follow-up: review eligibility-aware CTA/legacy handling without fabricating/backfilling creator identity. Likely files: ProjectDetailPage, ProjectStartupWizard, startup readiness contract only if a new eligibility signal is needed. Validation should cover valid new projects, NULL-creator legacy projects, sample projects, runtime-present projects and denied actors. No fix implemented or implicitly approved by this UAT request.

## Evidence and cleanup

Screenshot directory (outside Git): `/Users/khiemnguyen/.codex/visualizations/2026/10/02/01a0fc1e-bd41-7350-97cc-6a24777bf4bc/consflow-pm-label-uat-1003/`.

- `role-pm.jpg`, `role-deputy.jpg`
- `pipeline-legacy-pm.jpg`, `members-mobile.jpg`
- `startup-fixture-a-error.jpg`
- `deputy-supervision-desktop.jpg`, `report-distinct-actors.jpg`

Existing own UAT fixture tag: `uat-dm-1002-6ff17d`. Temporary developer tag: `uat-pm-label-1003-admin`. Temporary credentials and backup hashes were removed after restoration; no password/token was written to repository or this report. Existing UAT business records were retained. Real users, persisted workflow descriptions and historical reports were not changed.

This turn changed verification documentation only. No additional mapping change is needed because feature ownership, behavior and contracts were not changed by UAT.
