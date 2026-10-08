# Consflow Seed And Demo Data Policy

`backend/seed/seed.sql` is a deliberate local/dev bootstrap for demo master records. It is additive and idempotent: it adds users/staff, canonical role assignments, partners, materials and warehouses without truncating business data or overwriting existing records. It does not create projects, workflow runs, completed dossiers or financial transactions.

## Current migrations first

Workflow definitions, forms and permissions come from the custom migration runner. Apply the current chain before seeding. V150 retires internal accounting only when its no-business-data preconditions pass; V151 replaces payment proposals with direct source/MISA reconciliation. Do not bypass these guards or replay applied SQL files manually.

For the configured local environment:

```bash
cd backend
make migrate
make seed
make seed-validate
make seed-fixtures-check
```

`make seed` runs only `seed.sql` and `validate_current_seed.sql`; it does not run migrations itself. The validator checks the current published Rev9, withdrawn obsolete templates, removed ledger tables and canonical demo masters. It does not certify a completed project or MISA reconciliation.

`backend/Makefile` chooses the Compose PostgreSQL service when available, using configured `POSTGRES_USER` and `POSTGRES_DB`; otherwise it uses `DATABASE_URL`. For an isolated target, use an explicit approved database URL:

```bash
cd backend
DATABASE_URL="$DEMO_DATABASE_URL" MIGRATIONS_DIR=migrations go run ./cmd/migrate
psql "$DEMO_DATABASE_URL" -v ON_ERROR_STOP=1 \
  -f seed/seed.sql -f seed/validate_current_seed.sql
```

Confirm the database name and storage target before writes. Never run demo seed against production. Running seed again preserves existing projects and does not reset a demo.

## Create sample projects through the authenticated application

Current SampleData blueprint version is `5`. Use **Cấu hình → Dữ liệu mẫu** or the protected API, with an authenticated actor holding `sample_data.create`:

| Endpoint | Purpose |
|---|---|
| `GET /api/settings/sample-data/options` | Current published pipelines, compatibility, role coverage and progress presets |
| `POST /api/settings/sample-data/users` | Provision missing eligible sample role accounts for the selected pipeline |
| `POST /api/settings/sample-data/projects` | Generate a project using the selected current pipeline and a UUID idempotency key |

The API reports `environment_policy=available_in_all_environments`; this does not authorize demo writes in a shared or production environment. This runbook uses local/disposable targets only.

Supported profiles are `draft` (0%), `planning` (5, 10, 15, 25, 40, 65 or 90%) and `completed` (100%). Provision missing role accounts first and resolve compatibility blockers. A completed sample is synthetic demonstration data, not evidence of real operation, payment or external accounting confirmation. Imported MISA observations come from the separate authenticated import flow.

The API returns new project identifiers; do not depend on former fixed Golden project IDs. Retry the same request with the same actor, parameters and idempotency key. Use a new key for a new sample. Create another isolated sample when a fresh scenario is needed; seed is not a reset mechanism.

## PDF fixtures and changes

Retained assets under `backend/seed/uploads`, the fixture generator and manifest remain available for document/evidence checks. `make seed-fixtures-check` validates eight PDF fixtures. These illustrative files do not create accounting records. Seed no longer copies them into storage; upload through the authorized evidence/document flow.

The obsolete Golden SQL project generators and their validator have been retired. Active runbooks must not invoke them. Applied migrations and closed plans remain historical records.

Before changing demo masters, confirm the supported workflow, preserve stable references where needed, exclude production credentials/personal data and run appropriate backend checks. Use reviewed migrations or imports for production reference data, with a documented source and recovery procedure.
