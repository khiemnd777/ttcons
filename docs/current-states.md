# Current States

## Production release and data transition — active 2026-10-20

**Target:** Prepare the demo-only production database for guarded migrations, release the approved Consflow source, then complete readiness checks for staff import, Workflow Pipeline v10 import, and sample data. Release tags follow `make release` automatic allocation. Cleanup requires an exact deletion manifest before execution.

**Will be done:**

1. Inventory production business records, notifications, My Work sources, discussion/activity, audit history, stored objects, and project FK delete actions. Preserve active Admin and separate canonical Developer access; prepare an exact pre-migration deletion and retention manifest before any destructive write.
2. Clear only the approved demo rows that block V150/V151, then confirm the migration guards are satisfied. Confirm source parity, run the tag-triggered validation/deploy workflow, and record the release tag, commit, migration outcome, and public health checks.
3. Run the remaining production readiness diffcheck with PASS/BLOCKED/UNKNOWN outcomes for migration integrity, protected accounts, reference roles/permissions/Forms, remaining cleanup, import previews, and sample-data prerequisites.
4. After the approved cleanup, verify Admin and Developer access. The human operator imports the staff list and Workflow Pipeline v10; verify imported identities, published Form/Pipeline revisions, and role coverage.
5. Agree how sample data will use the imported staff list. The existing sample generator creates separate sample accounts, so its current behavior does not meet that requirement by itself. Create and verify samples only after the actor model is approved.

**Done:**

- Read-only public checks on 2026-10-20 returned HTTP 204 for `/gateway-health` and HTTP 200 for `/api/ready`; those checks do not identify the deployed revision.
- Local `main` and `origin/main` both resolve to `eff150e3b7b4d6ba5c364e3ebe66859722687ee0` before release. The unrelated untracked `output/pdf/demo-guide-2026-10-16/` is preserved.
- Source inspection confirmed that bulk staff import rejects the `admin` and `developer` role codes; both accounts must be established and verified separately before cleanup.
- Plan note committed as `eab0643` and pushed to `main`. Policy-allocated tag `v2026-10-20.1` points to that commit. GitHub Actions run `39361206478` was cancelled during backend validation, before image build or VPS deploy, because the deploy script has no pre-migration backup/restore gate. No migration or new release was applied by that run. Public gateway/API readiness remained HTTP 204/200 after cancellation.
- One-time read-only inventory workflow ran successfully from `v2026-10-20.2` (run `39362430226`); its paired release run was cancelled before build/deploy. Production remains on `v2026-09-25.1` at migration 145. The inventory found 3 projects, 2 active Admin accounts, 1 active Developer account, 24 users, 23 Staff, 3 Pipeline runs, 63 Work Items and nonzero activity/notification records. Guarded retired accounting business tables were empty. Account login capability and record ownership remain unverified.

**Current gate:** No new release has deployed. The operator confirmed that current production records are demo-only and may be deleted. V150 refuses to migrate while `projects` or retired accounting business tables contain rows; V151 has a second empty-accounting-data guard. The read-only inventory proved 3 project rows, so an exact pre-migration cleanup is required before deploy. A second read-only inventory will report all nonempty tables and project FK actions for that cleanup design. Production account login, backup state, v10 package, and authenticated previews remain UNKNOWN. Direct read-only SSH from this workspace was denied; public health endpoints cannot establish these facts.
