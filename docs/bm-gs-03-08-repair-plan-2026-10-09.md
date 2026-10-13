# BM03/BM08 native-source repair — approved 2026-10-09

User approval: `Approve 1-3`; UI text-mockup approval: `Duyệt bố trí này`. Status: **IMPLEMENTED / TECHNICALLY VERIFIED / BOUNDED SYNTHETIC BROWSER + NATIVE GOOGLE ACCEPTANCE PASS** on the final frozen source. Scope is existing subcontractor workflow forms and handover dossiers, their native export buttons, and bounded synthetic browser/Google acceptance. Normal/production data, deployment and Git delivery are outside scope. Exact evidence, tested source, scenarios and limits: [verification report](bm-gs-03-08-verification-2026-10-09.md). The parent eight-template plan remains active.

## Verified problem and source

Original workbook: `/Users/khiemnguyen/Works/ttcons/documents/Bo_Bieu_Mau_Giam_Sat_Thu_Thiem.xlsx`; TOC identifies BM03 as internal subcontractor quality/quantity acceptance and BM08 as completion/document/physical handover checklist. Previous actual native downloads under `/private/tmp/consflow-native-browser-20261008/workbook-check/` were compared with the original. Read-only checks in the task-owned DB confirmed the missing source facts.

- BM03 B5 reads a nonexistent canonical `reference_no`; G9+ currently shows task status/reconciliation response rather than physical quality result. A13 retains only its prefix, losing both client choices. New source fields are absent from the published form schemas.
- BM08 F5 expects `handover_stage` absent from native capture. A13 loses both client choices and no explicit conclusion is captured. The native dossier seeds six document/system-readiness rows and has no user physical-item creation. Existing verification update refuses automatic rows, so physical observations require a separate mutation that cannot modify verification.
- Linked manual rows already have `Cập nhật liên kết` in the source dialog; the previous claim that they cannot be edited was incorrect. Existing source and evidence viewers remain the owners.
- BM03 blank scope/subcontractor and BM08 blank row observations in previous files match absent UAT inputs; these are not mapper data-loss defects. Missing facts must stay missing. BM08 deadline formatting also needs date normalization; populated-date rendering was not tested in the previous fixture.

## Matched mapping, ownership and impact

Bootstrap: module keyword, backend function, frontend utility, root-cause maps and Quality/Handover + Workflow domain maps. First searches: `nativeBMGSChecklistResult|reference_no|conclusion|handover_stage`, `UpdateHandoverChecklistItem|CreateHandoverPackage`, `ChecklistAction|HandoverChecklistSourceDialog|item_fields`.

| Area | Existing owner | Potential collision | Required check |
|---|---|---|---|
| BM03 fields | Workflow form definitions/versions, WorkflowFormPanel | Rewriting pins or historical answers; task status mistaken for quality | Add immutable revisions; retain existing required fields/default eligibility; exact saved-answer projection |
| BM08 capture | Handover service/repository/workspace | Physical assessment changes verification/readiness; cross-project edit; completion race | Separate observations; parent-row lock, project-scoped existing permission, terminal lock and audit |
| XLSX | Native projection and workbook renderer | Sample accepted decision copied, dynamic footer clipping, timestamp text | Explicit decision only; both option labels; expanded rows/signatures and date-style regression |
| UI | Dialog, SimpleForm, shared workflow table and native export button | New export chooser or duplicate upload/viewer | Reuse native entrypoints, existing source viewer and closeable toast; desktop/mobile save/download |

Implementation is safe within this split. Source edits never automatically set quality pass, eligibility, document verification or readiness. New observation items do not introduce new mandatory evidence/readiness gates.

## Approved native UI mockup

Mục đích chính: ghi nhận nghiệm thu và bàn giao trong hồ sơ đang làm, rồi xuất dữ liệu đã lưu.

```text
PHIẾU NGHIỆM THU THẦU PHỤ                 [Xuất báo cáo]
Số biên bản [........]   Ngày nghiệm thu [........]
Hạng mục [...]          Thầu phụ [...]
Nội dung | Tiêu chuẩn | Yêu cầu | KL thiết kế | KL thực tế
         | Kết quả [Chưa đánh giá / Đạt / Không đạt]
         | Nhận xét nghiệm thu [...]
Kết luận [Chưa kết luận / Đồng ý nghiệm thu /
          Cần sửa chữa tồn tại]
                         [Lưu nháp] [Gửi phiếu]

HỒ SƠ BÀN GIAO                           [Xuất báo cáo]
Giai đoạn bàn giao [...]             [Cập nhật đánh giá]
Checklist                         [Thêm hạng mục bàn giao]
Hạng mục | Nguồn xác minh | Đánh giá | Thao tác
...      | ...            | ...      | [Đánh giá] [Xem nguồn]
Dialog đánh giá:
Yêu cầu [...]   Kết quả [Chưa đánh giá / Đạt / Không đạt]
Tồn tại [...]   Hạn xử lý [...]                 [Lưu]
Kết luận [Chưa kết luận / Đủ điều kiện bàn giao /
          Chưa đủ điều kiện]
```

Standard reuse: workflow schema fields → existing WorkflowFormPanel structured table; native record/row edits → Dialog + SimpleForm; export → NativeExcelExportButton; original evidence/source → existing HandoverChecklistSourceDialog/EvidenceReferenceViewer. Mobile fields stack; terminal dossiers are read-only; inline validation and dismissible toast handle errors; existing loading/retry remain.

## Capture and output contracts

- BM03 new immutable versions of `builtin_subcontractor_checklist` and `client_subcontractor_acceptance_package`: optional `reference_no` (160 chars), `acceptance_conclusion` (`accepted`, `rework_required`), row `quality_result` (`pass`, `fail`, `not_assessed`) and `quality_note` (1000 chars). Published old definitions, template/run/form pins, snapshots and signatures are not repointed. Future step configuration selects the new version explicitly through the existing catalog.
- BM08 nullable `handover_stage` (160 chars), explicit `handover_conclusion` (`eligible`, `not_eligible`) and decision actor/time. Header assessment, item assessment and physical-item creation are protected native dossier APIs. Omission preserves; explicit blank clears. Actor/project permission and terminal state are rechecked under the parent lock; mutations and before/after audit share a transaction.
- Item observations update only requirement/result/defect/deadline, including automatic/linked rows. They cannot set status, verification actor/time, sources, readiness or complete handover. User physical rows have generated stable codes, category `other`, `required=false`; no new delete operation or readiness gate is introduced.
- BM03 B5 comes from saved answers; G displays physical result and quality note. Original status/response/comment survive as supplemental source context. Both BM03 conclusion options remain visible; explicit accepted/rework decision marks only its option. BM08 uses the same explicit-only rule for eligible/not eligible. Missing/invalid legacy decisions do not mark an option.
- Deadlines use the stored business date without timezone shifts. Dynamic rows shift footer/signatures; both option labels must remain readable in Excel and native Google conversion. BM04 counts and BM07 image writer are unchanged.

## Validation executed

1. Fail-first reopened-XLSX regression for source mapping, result semantics and both conclusion choices; include missing/invalid decisions, dynamic row expansion, units and dates.
2. Disposable DB integration proves migration immutability, persisted capture, omission/clear, permission denial, terminal locks, unchanged automatic verification/readiness and audit. Verify test schemas are removed.
3. Frontend checks/build plus independent GPT-6.1 Sol High source/contract/domain review. Freeze tested source hashes and build fresh API/UI against only task-owned synthetic UAT.
4. Browser save/reload scenarios for both BM03 source types and BM08 header/manual/linked/automatic observations, positive/negative/unassessed states, validation and desktop/mobile. Capture actual download events from `Xuất báo cáo`.
5. Import those unchanged downloads privately as native Google Sheets. Read back expected cells and inspect rendered conclusions/results/dates, expanded rows and signature labels. Record exact file hashes, source revision/state, database/migration state, scenarios/results and skips.

No Microsoft Excel, physical print, normal database rollout or real-client acceptance is claimed. Update domain/feature/detail/triage maps and current-state notes in this work turn. Finish only when required checks have evidence; report any remaining blocker with its concrete next action.

### Done

- Capture/renderer/native UI implemented by GPT-6.1 Sol Medium; final scoped source audit by Sol High found no remaining actionable P0–P2. Browser acceptance exposed a handler allowlist HTTP500 missed by service-only coverage; safeTable and real-handler permission/save regressions were repaired. Earlier clean review was retracted. Pinned human labels and integer unit rendering were corrected after actual Google inspection.
- Final frontend focused checks 21/21 and tsc/Vite build passed; backend focused disposable-DB capture/HTTP/pinned-field tests, pure BMGS regressions, vet and fresh normal/guarded builds passed. Final 114-file manifest remains unchanged after builds.
- Both BM03 source types were saved/reloaded through the native browser; BM08 header and manual/linked/automatic/new physical items were saved/reloaded, with explicit clears, validation, failed-save retry and mobile/desktop checks. 27 old form pins/answer hashes and linked source verification stayed unchanged. Seven canonical/new physical rows remain pending and readiness remains unchecked.
- Four unchanged actual `Xuất báo cáo` downloads were privately imported to native Google Sheets and inspected at 100%. Both conclusion choices, explicit results/unassessed rows, numeric quantities, expanded signatures and numeric business date2026-10-12 passed bounded readback/render checks. The final dated negative BM08 supersedes the first undated Google artifact; no manual Google corrections or post-download transformations.
- Required central/backend/frontend/triage and quality/workflow/reporting maps updated. Standard Dialog/SimpleForm, schema renderer, native export button, API client/toast and existing source/evidence viewer are retained.

### Remaining outside the completed repair scope

Normal/production V157 rollout and explicit future-workflow form-version configuration require a separately authorized rollout. Existing pinned forms do not acquire new fields automatically. Full eight-template/client acceptance, Microsoft Excel, printing and complete submit/signature/real handover workflows were not certified by these draft-source UAT scenarios. No Git commit/push or plan closure.
