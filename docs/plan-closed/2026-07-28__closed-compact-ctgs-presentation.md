# [Closed] Compact CTGS Presentation

Status: closed

Closed at: 2026-07-28

Will be done:

- None. The user closed this plan after the approved implementation and technical verification.

Done:

- User approved points 1–4: keep normal sequential CTGS numbers in full, compact long converted values, apply the presentation consistently across the accounting books and tax ledgers, and make no API/database or posted-journal mutation.
- Added one shared CTGS presentation helper/component for Journal, General Ledger and VAT desktop/mobile surfaces. Sequential `CTGS-*` values remain complete; long `BF-*` and future long identifiers display a stable prefix/suffix abbreviation while retaining the immutable full value in the accessible label and raw row data.
- Focused presentation tests passed `4/4`; the full frontend suite passed `27/27`; TypeScript/Vite production build and the 250 KiB bundle budget passed on the working tree at implementation time.
- Browser UAT was not authorized in the implementation turn, so the change was technically verified but remained `NOT UX ACCEPTED` when the plan closed.
