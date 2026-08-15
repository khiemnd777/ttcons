# [Closed] Project Explorer Desktop Column Spacing

## Will be done

- Keep Project Explorer desktop `Tiến độ` and `Ngày tạo` in dedicated non-overlapping columns for every progress value from 0% through 100%.
- Reuse one desktop grid contract for the header and rows, allow the progress bar to shrink safely, and keep percentage/date text on one line without changing mobile cards.
- Add focused frontend regression coverage, update central/frontend/root-cause mapping, and run the source feedback loop plus non-browser frontend tests/build.

## Done

- Reproduced the supplied Edge symptom where `100%` touches `10/7/2026` and built the failing source signal `PROJECT_EXPLORER_PROGRESS_DATE_SPACING`.
- Proved the root cause is the 90px progress column containing a 64px minimum bar plus gap and percentage label; header/row grid mismatch and date formatting were ruled out.
- Produced the review package and Mockup as Text, then received explicit `OK triển khai` approval for the 112px progress column, 100px date column and shrink-safe progress presentation.
- Replaced the duplicated header/row grid declarations with one shared desktop contract, made the progress bar shrinkable, and kept percentage/date values non-wrapping with tabular numerals. Mobile cards, API behavior, sorting and pagination remain unchanged.
- Extended the responsive source-contract regression and updated the central, frontend and root-cause maps. No backend, database, migration or project-planning domain invariant changed.
- Technically verified the current working tree with the passing source feedback signal, focused responsive tests (3/3), complete frontend suite (105/105), TypeScript/Vite production build, bundle budget (111 chunks; 121.5 KiB gzip entry under 250 KiB) and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
