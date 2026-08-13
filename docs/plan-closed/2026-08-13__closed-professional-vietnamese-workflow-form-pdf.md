# [Closed] Professional Vietnamese Workflow Form PDF

## Will be done

- Replace the workflow-form PDF's ASCII-only renderer with an embedded Noto Sans UTF-8 A4 document while preserving the existing immutable payload hash and protected Local/S3 storage flow.
- Add a workflow-form presentation layer for Vietnamese status, signature-purpose, option, boolean, date, currency, checklist and structured-list values without changing persisted answers.
- Reframe the document around business reading order: branded identity, record summary, form content, confirmation/signatures and compact verification metadata with repeated page furniture.
- Add focused generator/service regression coverage, render representative one-page/multi-page/signature PDFs for visual inspection, update the Consflow central/backend/workflow mappings and run current-source backend verification.

## Done

- Confirmed the previous generator stripped Vietnamese diacritics through `asciiFold`, used built-in Helvetica, printed raw technical values and gave UUID/checksum metadata higher visual priority than the business content.
- Confirmed the repository already embeds Noto Sans Regular/Bold and uses the UTF-8-capable `codeberg.org/go-pdf/fpdf` library for pipeline-template PDFs.
- Produced and received explicit approval for the A4 portrait navy/blue review package and Mockup as Text, including Vietnamese presentation mapping and unchanged storage/database/API/UI boundaries.
- Replaced the handwritten Helvetica stream with a branded A4 portrait `fpdf` renderer that embeds the existing Noto Sans Regular/Bold assets, declares `vi-VN`, adds document metadata and repeats a compact brand/header/footer with page numbers and abbreviated verification hash.
- Added a presentation model that respects field visibility and translates published option labels, booleans, dates, VND/non-VND currency, evidence references and structured lists without mutating answers. Empty values use `Chưa cung cấp`; confirmation boxes and signature strokes are vector-rendered.
- Added project, node, step, export actor and export time as PDF-only context from the already-authorized work item. The immutable payload hash still covers only the existing form snapshot and signatures, so presentation/export metadata cannot change content identity.
- Added focused regression coverage for Vietnamese business context, `minimal -> Tối giản`, localized date/VND/confirmation output, vector signature preservation, embedded-font PDF output and payload-hash stability across different export metadata.
- Rendered and visually inspected compact, multi-page structured-list and signed PDF fixtures with `pdftoppm`: Vietnamese diacritics are intact, A4 page geometry is correct, confirmation marks render without glyph fallback, section headings are not orphaned, structured items retain their headings across page breaks, signature strokes remain visible and no clipping was observed.
- Updated central/backend/workflow mappings for the UTF-8 workflow-form PDF contract. No frontend mapping update was required because the existing protected open/delete UI and API contract are unchanged.
- Technically verified the current backend source with focused PDF tests, full `make test`, `git diff --check` and agent-artifact validation. A PostgreSQL-backed create/store/open/delete integration test passed on isolated database `consflow_pdf_render_019ff90a`; that database was dropped afterward.
- Rebuilt and restarted the local Compose stack from the verified backend source. PostgreSQL/Redis are healthy, backend/admin are running, all 93 migrations were already applied, local storage preflight is ready, and `/api/health` plus `/api/ready` return HTTP 200. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
