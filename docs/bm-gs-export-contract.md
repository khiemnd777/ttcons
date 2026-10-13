# Superseded BM-GS prototype implementation contract

Status: **SUPERSEDED / PRODUCT DESIGN REJECTED**, 2026-10-08. The user clarified that exports belong in each existing construction/supervision feature and inherit its report/filter/revision and linked photos. The template/source/photo chooser and export-time uploader below are the rejected prototype, not authorization for future work. Use [the retro and replacement review package](bm-gs-retro-replan.md) and `docs/current-states.md` for the active plan. Preserve this document only to trace the old implementation and its technical contracts; it is not product acceptance evidence.

## Ownership and public contracts

- Export API worker owns new `*_bm_gs_exports.go` query/service/handler/route files and route registration only. Renderer worker owns `bm_gs_workbook.go`, `bm_gs_images.go`, related render/image tests and backend go.mod/go.sum. Source worker owns `bm_gs_sources.go`, `bm_gs_source_repository.go`, `bm_gs_source_validation.go`, related tests, additive V152 migration and minimal canonical snapshot integrations. Frontend export worker owns the new shared export dialog/API contracts and Project Detail entrypoint. Frontend source worker owns the shared typed source-data editor and canonical source entrypoints. Root owns this contract, current-states and final mapping integration. Reviewer is read-only.
- All workers share the checkout. Do not revert other changes or commit/push. Notify root before touching a file owned by another worker.

### Export request

`POST /projects/:id/bm-gs/preview` and `POST /projects/:id/bm-gs/export` accept:

```json
{"template_code":"BM-GS-07","date_from":"2026-10-01","date_to":"2026-10-08","sources":[{"kind":"evidence","id":"UUID","revision_no":0}],"evidence_ids":["UUID"]}
```

Dates are optional when exact sources are selected. Bounds, exact source IDs, date order, source kind, template membership, project scope and actor permission are validated server-side. Do not derive an ID from a name. Export rechecks source readiness and authorization; preview is advisory.

`GET /projects/:id/bm-gs/options?template_code=&search=&cursor=&limit=&date_from=&date_to=` returns `{templates:[{code,name,purpose,recommended,reason}],sources:{items:[{kind,id,label,date,status,revision_no,work_item_id}],has_more,next_cursor},capabilities:{export,upload},upload_work_items:[{id,label}]}`. Default 25/max50; stable sort and source visibility before paging. Only allow-listed source kinds/templates. Source options can include immutable report revision references (`kind=work_report`, `revision_no>=1`) and editable daily sources (`kind=daily_report`, `revision_no=0`); historical report choices must not load live content/evidence instead. Recommendations use the active workflow Giai đoạn business code and visible source kinds; the supervisor retains template selection.

Preview returns `{template_code,source_count,image_count,issues:[{source_kind,source_id,field,label,message,required}],can_export,estimated_image_bytes,evidence_records:[{record,protected_file_path}]}`. Protected paths preserve the selected immutable report revision and do not require invented storage keys. Missing optional fields may stay blank. Required checks reflect the client field meaning; signatures remain an unsigned signing area. Never invent pass/fail, quantities, timestamps or progress. Preview and export both use the heavy-export concurrency guard.

Export returns the XLSX binary with filename `[project_code]_[template_code]_[date_or_range]_[content].xlsx`. Keep existing project/reference/register exports unchanged. Export permission starts with `work_report.export` plus exact project/source visibility; source permissions remain authoritative, including historical report evidence authorization. Do not repurpose dashboard permission.

### Source-owned enrichment

Canonical kinds/table owners: `survey` -> workflow_form_instances; `daily_report` -> operation_daily_reports; `acceptance` -> acceptance_records; `action` -> construction_action_items; `variation` -> contract_variations; `site_work` -> work_items; `evidence` -> evidence_records; `handover` -> handover_records.

Add non-null typed `bm_gs_json` (default `{}`), `bm_gs_version` and actor/time metadata on owning records via V152. No parallel report ledger and no arbitrary JSON writer. Source-specific typed schemas/limits/validation govern the JSON. IDs/units/source references are explicit. Changes are allowed only to actor-authorized editable current sources; reject historical/submitted/acknowledged/signed/closed sources according to each existing owner. Under a transaction, lock/recheck source, current Giai đoạn, schedule, actor and expected version. Linked report mutations follow form → daily report → Đầu việc lock ordering. Evidence enrichment preserves original file/checksum and accepted/historical metadata.

`GET /bm-gs/sources/:kind/:id` returns `{kind,id,project_id,label,version,data,capabilities:{edit},source_path}` plus source-specific authorized `crew_rows`, `criteria`, `partner_options`, `receive_source_options`, `risk_rows` and `handover_items/readiness`. `PUT` accepts `{expected_version,data}`; strict schema validation (128 KiB maximum), no source state transitions or stock/finance effects, audited update. `source_path` points to the exact canonical source. A 409 retains the frontend draft and requires an explicit reload; the server does not silently overwrite newer data.

Implemented typed schemas: survey `locations[]` with row_id/location/dimensions/mep/existing_condition/assessment/note and `risks[]` linked by exact source_id/source_row; daily `criteria[]` with 12 semantic codes/assessment/note/evidence_ids, `crews[]` with canonical row_id/representative/contact/partner ref, `deliveries[]` with row_id/exact receipt source/specification/origin/quantity/unit; acceptance `items[]` keyed to canonical criterion with standard/drawing revision/technical requirement/design quantity/actual quantity/unit/measurement source; action fault_code/location/severity/partner/representative/contact plus risk facts; variation risk_code/detected_at/cause/impact/risk_level/proposed_action/owner/due_at; site_work instruction_code/location/partner/representative/contact/issued_at/due_at/progress_percent with actor/time; evidence location/captured_at/photo_phase (`before|during|after`)/technical_content/handover_note; handover `items[]` with canonical row_id/requirement/defect/due_at and exact existing evidence/document references.

BM-GS-02 criterion codes are `progress`, `workforce`, `deliveries`, `quality`, `completed_quantity`, `variation`, `mep`, `safety`, `acceptance`, `risk`, `images`, `instructions`. STT 1–12 is presentation only. All 12 need a recorded assessment before export. Crew names/counts stay in the canonical report; enrichment joins by persistent row_id, never name or position. V152 adds row IDs only to current editable eligible reports, audits the backfill and leaves historical revisions unchanged. Receipt quantities/units come from exact receive movements, not material consumption.

Workflow-created daily reports exist after form submission. Their current additions remain editable under the same canonical amendment authority before formal report delivery or after `needs_changes`; a submitted/acknowledged formal report remains locked. Crew IDs are established before canonical Save/Amend serialization. V152 synchronizes exact matching current form crew facts and IDs without rewriting immutable report revisions or PDF artifacts. A workflow form's current editable snapshot is distinct from an immutable delivered WorkReport revision.

BM-GS-05 combines selected survey risks, construction actions and contract variations with exact source identity and causal deduplication. BM-GS-06 includes construction/site-instruction Đầu việc only. BM-GS-08 uses actual `handover_checklist_items`; canonical title, verification status, accepted evidence and readiness determine result. A pending checklist cannot become Đạt through enrichment. Notes do not become a handover conclusion. Required quality checks stay with the owning acceptance/handover workflow.

Read projections combine canonical facts with these strictly defined source additions. The enrichment must not override canonical approval/status/date/identity, fabricate relations, or infer received material from usage. Submitted work-report snapshots include daily data, complete selected evidence metadata/enrichment and project identity/address under source locks. Historical sheets use frozen project metadata even after a project rename. Existing snapshots without additions expose missing fields honestly and never fall back to live enrichment. Business dates use their source meaning and Vietnam date boundaries; upload/create times do not replace capture, detection or inspection dates.

### Pure renderer interface (root-owned Go types)

`generateBMGSWorkbook(data BMGSWorkbookData) ([]byte,error)` and `prepareBMGSImage(raw []byte, mimeType string) (BMGSImage,error)`.

`BMGSWorkbookData {TemplateCode,ProjectCode,ProjectName,Address,PreparedBy,PeriodLabel string; GeneratedAt time.Time; Sheets []BMGSSheet; Images []BMGSImage}`.

`BMGSSheet {Name,DateLabel,Title string; Metadata Record; Rows []Record; CrewRows []Record; MaterialRows []Record; CriteriaRows []Record}`. Row keys follow the client columns and are documented by renderer worker. Date/quantity values remain typed; identifiers remain text. Daily reports are separate sheets in one selected-template workbook; duplicates on the same day receive deterministic suffixes.

`BMGSImage {EvidenceID,Filename,Caption,Location,CapturedAt,Phase,SourceLabel,MIMEType string; Bytes []byte; Width,Height int}`. API worker loads exact authorized source bytes and calls preprocessing, bounded input bytes/pixels/image count/output budget; no remote URL fetch. Renderer never accesses storage/DB. Normalize EXIF orientation, max1280 long edge, JPEG80 photos, no upscaling/cropping; preserve appropriate PNG detail/transparency. Same-source images deduplicate. Appendix anchors and dimensions prevent overlap and repeat identifying captions. Missing/invalid images cause explicit issues/errors, never silent omission.

Main sheets preserve the client's column widths, print orientation, paper size and scale. Dynamic content updates fit-to-width and print area; text/caption height grows conservatively. Text beyond Excel's row-height ceiling fails explicitly instead of silently clipping. Native Excel pagination/readability still needs acceptance on the client's environment.

Public export limits: at most 50 selected sources and 20 selected photos; 12 MiB raw per photo, 40 MiB total raw bytes, 30 MiB processed image bytes and 50 MiB final workbook. JPEG/PNG/WebP actual byte signatures are checked; decode dimensions are bounded before allocation. Preprocessing has a defensive 40-million-pixel ceiling. These limits bound a synchronous export; a larger job is outside this approved slice.

### UI

Approved purpose: supervisor chooses the appropriate client template, source/date/revision and images, sees missing information, follows exact canonical source to add data, then generates Excel. Shared Dialog with responsive layout; existing EvidenceCapture/EvidenceUploader for uploads and EvidenceRecordViewerDialog/EvidenceDraftViewerDialog for previews. New uploads require explicit same-project Work Item and canonical upload permission/state; upload receipts prevent duplicate uploads on retry. Uploaded images are persisted originals before export. Each source enrichment editor is attached to the canonical source, not a parallel report content editor.

BM-GS-07 new uploads become explicitly selected canonical evidence sources. For one editable BM-GS-02 daily report, uploads also append through the existing daily-report evidence endpoint; uploading to an Đầu việc alone does not attach a photo to a report. Immutable report revisions cannot accept new uploads. Other templates select photographs already associated with their canonical source. Changing template, source, revision or images invalidates preflight.

BM-GS-07 source/photo selection uses capture-day filters through its export options. It does not reuse generic Evidence upload-date filters. Persisted new photos obtain fresh exact kind/id/project-validated source capabilities; the source editor is available before preflight. If metadata loading fails, the uploaded original and selected ID remain and the retry reads only metadata. Missing business dates under an active filter return required preflight issues; an actual recorded date outside the range is rejected.

Known assessment/status enums render in Vietnamese with source-specific meaning. `pending` stays Chưa đánh giá; a submitted survey form or contract approval state does not imply that its risks have been resolved. Free text and unknown historical values remain as recorded.

Exact source navigation reuses shared record links and canonical dialogs. Daily/action adapters read their existing project-scoped collections; this change does not add pagination to those legacy APIs. Contract links validate the exact variation in its current parent. WorkReport links select the requested immutable revision and reject missing revisions without fallback. Legacy standalone surveys have backend read support but no runtime source editor UI; their empty source path is explicit and cannot grant edit authority.

V152 requires the ordinary custom migration runner. It adds eight table columns/constraints and updates only eligible current crew metadata; table ALTER and current-row backfill locks must be assessed against deployment data volume. It has only been run on disposable synthetic test databases in this task. No production migration or rollback is claimed.

## Gates

- No browser/Playwright/headless verification without explicit request in the current turn.
- No production migrations, deployment, seed resets, Git commits/push or global installation.
- Add meaningful permission/snapshot/validation/concurrency/render/image/contract regressions and update mappings before handoff.
- User language: Vietnamese. TechNote statuses Verified/Proposed/Unknown remain explicit. Native Excel readability/print and browser UAT remain unverified unless actually performed.
