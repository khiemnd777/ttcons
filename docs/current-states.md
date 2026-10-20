# Current States

## Production release and data transition — active 2026-10-20

**Target:** Release the approved Consflow source to production, then establish readiness for cleanup, staff import, Workflow Pipeline v10 import, and sample data. The release tag follows `make release` automatic allocation. Production cleanup requires a separate exact deletion manifest and approval before execution.

**Will be done:**

1. Confirm source parity, run the tag-triggered validation/deploy workflow, and record the release tag, commit, migration outcome, and public health checks.
2. Run a production readiness diffcheck with PASS/BLOCKED/UNKNOWN outcomes for migration integrity, backup/restore, protected Admin and separate canonical Developer accounts, reference roles/permissions/Forms, cleanup dependencies, import previews, and sample-data prerequisites.
3. Inventory production business records, notifications, My Work sources, discussion/activity, audit history, and stored objects; prepare an exact deletion and retention manifest for review before any destructive write.
4. After the approved cleanup, verify Admin and Developer access. The human operator imports the staff list and Workflow Pipeline v10; verify imported identities, published Form/Pipeline revisions, and role coverage.
5. Agree how sample data will use the imported staff list. The existing sample generator creates separate sample accounts, so its current behavior does not meet that requirement by itself. Create and verify samples only after the actor model is approved.

**Done:**

- Read-only public checks on 2026-10-20 returned HTTP 204 for `/gateway-health` and HTTP 200 for `/api/ready`; those checks do not identify the deployed revision.
- Local `main` and `origin/main` both resolve to `eff150e3b7b4d6ba5c364e3ebe66859722687ee0` before release. The unrelated untracked `output/pdf/demo-guide-2026-10-16/` is preserved.
- Source inspection confirmed that bulk staff import rejects the `admin` and `developer` role codes; both accounts must be established and verified separately before cleanup.
- Plan note committed as `eab0643` and pushed to `main`. Policy-allocated tag `v2026-10-20.1` points to that commit. GitHub Actions run `39361206478` was cancelled during backend validation, before image build or VPS deploy, because the deploy script has no pre-migration backup/restore gate. No migration or new release was applied by that run. Public gateway/API readiness remained HTTP 204/200 after cancellation.

**Current gate:** The tag exists but its workflow was cancelled before deployment. Obtain and verify a production backup and restore path before resuming with a new policy-allocated tag or another reviewed release procedure. Production account state, data inventory, backup state, v10 package, and authenticated previews remain UNKNOWN. Direct read-only SSH from this workspace was denied; the public health endpoints cannot establish these facts.
