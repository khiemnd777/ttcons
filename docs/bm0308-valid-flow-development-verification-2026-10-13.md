# BM03/BM08 valid dossier-flow Development UAT — 2026-10-13

## Decision and limits first

- **BM03 client-package full flow: BLOCKED / NOT UX ACCEPTED.** The exact submitted native client package exports correctly, but its own reconciliation endpoint returns `available=false`, `Biểu mẫu quy trình không phải checklist thầu phụ`. `service_reconciliations.go:279–282` accepts only `builtin_subcontractor_checklist`. This is a verified support gap, separate from XLSX mapping.
- **Official Rev10 full-flow parity: NOT_RUN.** The three isolated synthetic test templates use canonical CreateTemplate and therefore engine2/new families. Official Rev10 uses engine4/official family. Matching pins, node/step flags and actions does not reproduce the official-family automation gate. Custom WorkItem completion is not evidence that the official source gate passed. Read-only official WorkItem proof remains pending/source not ready. No official-family clone/publish or historical run was changed.
- **BM03 submitted exports, builtin reconciliation and BM08 physical dossier/export: TECHNICALLY VERIFIED in the bounded synthetic Development scope. User visual review PENDING.** This does not mean either complete client workflow or all eight templates are accepted.
- Accounting closure, final stage approval, ordinary-role/denied-permission cases, full official project sequence, physical inspection/camera, mobile, desktop Excel/print and Production were not tested. PDF-progress visual acceptance remains separately pending.
- The optional handover date was entered through automated date-input fill, but the created record has `handover_date=null`. The raw Create request body was not captured, so no request-body claim is made. Complete used the existing server-date default `2026-10-12`, while the local completion display is13-10-2026. The BM08 worksheet name faithfully uses the stored date. Date entry/timezone UX is **not accepted by this UAT**; no application date bug/root cause is claimed from this automation result. The immutable completed dossier was not rewritten.

## Authorization and source

User: `Duyệt nghiệm thu BM03 và BM08 theo luồng hồ sơ hợp lệ.` Approves bounded synthetic Development dossier actions, existing export buttons, unchanged private native Google imports and screenshots. No new field/schema, application code change, direct SQL fixture mutation, guard bypass, Production or Git delivery.

The original client workbook was read again, including TOC. BM03 covers internal quality and quantity with subcontractors; BM08 covers completion, as-built documents and project handover. Original core headings/columns/conclusion/signature labels remain in the exports. Additional existing source-information appendices are present below the core form; these were not added this turn. Blank defect/deadline/signature cells are retained when no such source fact exists.

Maps/owners loaded: central/backend/frontend/triage + Quality Acceptance/Handover and Workflow domain maps. First targeted searches: `SubmitWorkflowForm`, `isSubcontractorChecklistForm`, `final_acceptance_id`, `HandoverOptions`, `HandoverCheck`, `CompleteHandover`. Main owners: `service_reconciliations.go`, `repository_pipeline.go`, `service_acceptance_outgoing.go`, `repository_handover.go`, `WorkflowFormPanel`, `HandoverCreateDialog`, `HandoverWorkspace`, `HandoverPage`, native source exporters. Standard interactions used: existing WorkItem/WorkflowFormPanel chốt/signature/PDF/export and HandoverCreateDialog/SimpleForm assessment/source verification/completion. No new UI, module, photo chooser or fields.

### Tested revision and environment

- HEAD `75dc44707785d399ed2408959bf784928a544e96`, retained dirty worktree. All968 application hashes match the previous PDF-progress build. Source manifest copied and rechecked live; no application edit this turn.
- UI `http://127.0.0.1:5173`; API `http://127.0.0.1:8081/api`; readiness200/database ok. Existing Development runtime was reused; no rebuild/restart was needed.
- Backend image `sha256:a669db43e6ede2571e57c3b7d818a89c2167b556ce4b2c15f0306048add2ebe0`, started2026-10-12T10:37:18.922633346Z. Admin image `sha256:2fc711645f527a8fea04477bca8578d9887aa652af799e577d34ee78d9964ed3`, started10:37:19.005808596Z. PostgreSQL/Redis/cache healthy.
- DB `consflow`,157 migrations.26 baseline business/history tables: every pre-existing ID/row hash unchanged. Only three labelled test projects/templates, their members/runs/forms and the new test final/handover/documents were added. Existing evidence and WorkReport history unchanged.
- Root owns fixture/API/UI/Google/evidence. Existing GPT-6.1 Sol High reviewer independently checks source and final evidence; no implementation worker/code change. Evidence-only work requires no mapping behavior update.

## BM03 evidence

Separate projects prevent a submitted builtin reconciliation from making the client project's aggregate source ready:

| Case | Source / native pin | Actual result |
|---|---|---|
| Builtin | project `dad2862d-def4-4833-a1f8-c8b7e329bd54`; form `122f0acb-6a1d-4337-a737-814764dbfb9a`; builtin v4 `61e9bf6c-33fb-4e62-ad98-5dc6d9cdba19` | Saved draft through existing API; chốt through actual UI. Own canonical reconciliation then submitted; WorkItem completed afterwards in the custom-template scope. The downloaded file was captured after form chốt and before reconciliation submission; it contains submitted form facts and does not attest the reconciliation event. |
| Client package | project `8ae7b7e1-2bcd-4633-a83a-b117eab65ead`; form `86535cf4-9104-4ede-acb1-48554178df1b`; exact Rev10 v3 pin `0845e4bc-a008-48bc-be5e-6d873dad8d43` | Chốt and UI download work. Own reconciliation unavailable. Custom WorkItem completed, but this is engine2 behavior, not official hard-gate acceptance. |

Three rows use explicit design25m² vs measured24.8/25/24.6m² and separately saved form claimed/confirmed quantities23/20/21m². Both exports use those saved form facts; the client case does not have a canonical reconciliation record. C9:C11 contains both standard and drawing/revision; D9:D11 holds technical requirements; G9:G11 holds explicit quality result/note; A13 holds the saved acceptance conclusion. `done` or a reconciliation decision does not replace quality facts. Claim quantity is optional in these native schemas; it is deliberately entered for this UAT, not newly required.

Read-only official WorkItem `2f10e39a-d2ee-412e-a343-36e3d0f8cee6` returns engine4, pending, `business_source.ready=false`, submitted_reconciliations0. This confirms current readiness only; the official project remains at its earlier stage. No force-advance or new source was added to it.

## BM08 evidence

Project `94f3512a-6625-40a8-a2ce-e3bf67619c66`, handover `f423fea4-5f37-4672-bc59-afcf6e1ed312`, code `UAT-BM08-VALID-20261013`.

1. Exact official owner-checklist v2 pin `0646a055-2c3d-4409-9e3b-26ff10ad4896` is active/published. Its omission from the broad catalog is not an invalid pin. Submitted actual UI source form `8cc65195-7bbb-4333-ab16-d8c5dd1c5730`.
2. Actual UI canvas test mark saved as **internal_ack**, actor Consflow Admin, synthetic/no legal effect. Signature `3d3b3d18-3b52-4c0a-abae-5618dbeddf72`. Actual UI PDF generation `0aa5d7a2-5eee-4dc1-a4e5-db79eade3671`; no `signed/waived` header spoof.
3. Canonical final acceptance `056e3830-c93a-4516-9d02-f7079495bef4` links that same form/PDF and derives `signature_status=signed`, `document_status=linked`; submitted then completed/pass. Owner reconciliation accepted that exact completed final acceptance.
4. Existing HandoverCreateDialog selected the actual eligible final acceptance and auto-suggested its representatives. Created the dossier through UI.
5. Four actual labelled PDF fixtures were rendered/inspected, uploaded through project document API and linked through existing UI verification: as-built, operation/manual, warranty, two-party test minutes. Required files were not waived; auto final/warehouse rows were not manually marked verified. Six physical assessment results were explicitly recorded in the synthetic scenario.
6. Actual UI submit → readiness check: six verified required rows, documents_ready/warehouse_closed true, zero issues/NCRs/reworks/construction actions/surplus blockers.
7. Actual UI Complete reran readiness. GET proves `status=handed_over` and nonempty completed_at. No HTTP200-only completion claim. Final node still retains separate accounting step and approval policy; no claim of finance or stage closure.

BM08 maps actual checklist titles and requirements to B8:C13, saved physical results to D8:D13, absent defects/deadlines stay blank, saved conclusion to A15. Source verified statuses are in a separate existing appendix and do not replace physical assessment.

## Downloads and native Google proof

Three exact browser-button files were copied byte-for-byte; no workbook regeneration, cell/formula/style/row edit or post-import repair:

| Actual Downloads file | Evidence copy | Native Google workbook |
|---|---|---|
| `BM-GS-03 (10).xlsx` | `BM03-builtin-submitted.xlsx` | https://docs.google.com/spreadsheets/d/1J639lD-1oV-M4rrlzfTe4fiXO-oVR9D3DIOylf25vCs/edit |
| `BM-GS-03 (11).xlsx` | `BM03-client-submitted.xlsx` | https://docs.google.com/spreadsheets/d/1TseWwvZy1iFQLFHB6oWrecPr9aGN7LVW_sH9npzvelI/edit |
| `BM-GS-08 (7).xlsx` | `BM08-handed-over.xlsx` | https://docs.google.com/spreadsheets/d/10azuQoYtNei0hdAtud0GBiuEbSfrQ7CEDMJ5WnaxkPk/edit |

Native metadata read before bounded cell reads. Both BM03 `2026-10-13_01!A1:G30`; BM08 `2026-10-12_01!A1:F32`. Each has60 populated cells, all60 match the downloaded source values, and no `effectiveValue.errorValue`, including `#VALUE!`. Numeric measured/design cells retain numbers and unit formatting. All three sheets visually inspected at100%, including core table/conclusion, source appendix and signature placeholders. Google imports private; no sharing changes.

Root evidence verifier `verify_evidence.py`: **PASS253 assertions**. It checks actual download identity, retained old rows, source hashes, form/signature/PDF/final/project links, required document identity, readiness/completed status, separated BM03 quantity authority, native cell/error equality, and scope limitations. This is UAT evidence validation, not new application unit coverage. Application unit/build tests were not repeated because source/runtime unchanged.

Independent GPT-6.1 Sol High read-only final audit confirms the253 checks, source968 hashes,26-table retained rows, byte-equal downloads, all bounded native Google values including blank cells, actual BM08 form/revision/signature/PDF/final/document links and Create/Ready/Completed/top/footer screenshots. No additional blocker for the bounded exports/BM08 dossier. Client full flow, official-family parity, finance/closure and date-entry UX remain outside acceptance. Three wording corrections from that audit are incorporated above; see `independent-audit.md` in the evidence directory.

Evidence root: `docs/acceptance-evidence/bm0308-valid-flow-development-2026-10-13/`. Screenshots: `bm03-builtin-submitted-ui.jpg`, `bm03-client-submitted-ui.jpg`, `final-source-signed-pdf-ui.jpg`, `bm08-create-valid-source-ui.jpg`, `bm08-ready-ui.jpg`, `bm08-handed-over-ui.jpg`; each Google workbook has `*-google-top.jpg` and `*-google-footer.jpg`.

## Proposed next action — not implemented

BM03 export mapping is supported. The unresolved client-package blocker is **source reconciliation support**, not absent XLSX fields. Proposal for a separately approved fix:

1. Extend the existing reconciliation contract to the exact submitted client-package code, bind rows/revision/project to its existing source data, preserve the separation of measured vs claimed/confirmed quantities, permissions and immutable submitted history. Do not add fields or substitute another builtin form.
2. Verify the existing UI route/surface can expose and submit that source. The current reconciliation component suppresses non-submit `available=false` reasons; avoid treating an absent panel as success. Any material UI adaptation requires the usual review package and Mockup as Text before implementation.
3. Prove client-package own reconciliation → official readiness with meaningful regression and an appropriate official-family test context, then repeat actual UI export/Google/screenshot checks. Do not publish a reduced fixture into the shared official family or mutate retained official runs merely to obtain a green UAT.

User approval of UAT is not approval of this code fix. No application change was made.
