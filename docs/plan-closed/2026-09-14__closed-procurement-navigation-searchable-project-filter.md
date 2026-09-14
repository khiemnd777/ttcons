# [Closed] Procurement navigation and searchable project filter

- Closed: 2026-09-14
- Status at closure: TECHNICALLY VERIFIED — NOT UX ACCEPTED
- Completed: added the `Mua sắm vật tư` global navigation item immediately after `Công trình`; replaced the global Procurement native project select with the shared searchable `AsyncProjectPicker`; added `GET /procurement-requests/projects?search&cursor&limit&selected_id` with existing project scope plus `procurement.view` enforcement; updated regression coverage and module maps.
- Verified: focused frontend tests, focused backend scope/permission/cursor test, and the frontend production build passed on the final working tree. No database migration or business-data mutation was required.
- Remaining: browser/UAT verification was not run because it requires explicit authorization.
