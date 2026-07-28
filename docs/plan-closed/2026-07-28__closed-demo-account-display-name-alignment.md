# [Closed] Demo Account Display Name Alignment

Status: closed

Closed at: 2026-07-28

Done:

- User approved the reviewed scope: `accounting@consflow.local` becomes `Ngân Kế Toán`, `director@consflow.local` becomes `Hoàng Long`, and historical actor-name snapshots remain unchanged.
- Updated both canonical `users.full_name` and linked `staff.full_name` values in `backend/seed/seed.sql`, and recorded the display-identity invariant in the Admin Governance domain map.
- Updated the current local PostgreSQL `consflow` database in one checked transaction: exactly two `users` rows and two linked `staff` rows were updated; both accounts remain active with their original `accounting` and `director` roles.
- Started the current backend temporarily on port 18080 with `AUTO_MIGRATE=false`; health returned 200 and authenticated login responses returned `Ngân Kế Toán` with role `Kế toán` and `Hoàng Long` with role `Giám đốc`. The temporary backend was stopped and both validation sessions were revoked.
- Full seed was deliberately not run because it truncates the local dataset. No frontend/backend executable source changed, so build and browser UAT were not applicable to this data-only rename.
