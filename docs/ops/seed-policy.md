# Consflow Seed And Demo Data Policy

Seed data exists to make local demos and development flows repeatable. It is not a production data source.

## Rules

1. Seed files must not contain production credentials, production personal data, or customer financial records.
2. Demo users may use predictable passwords only in local/dev data.
3. Stable UUIDs are allowed for demo flow references, but new seed rows must remain clearly synthetic.
4. Seed data should exercise real workflows that the app supports; avoid placeholder-only rows that make a feature look complete when it is not.
5. Production reference data should be loaded through reviewed migrations or operator-run import jobs, not by reusing demo seed SQL.

## Local Usage

When the local stack is running:

```bash
cd backend
make seed
```

When PostgreSQL is available through `DATABASE_URL`:

```bash
cd backend
DATABASE_URL="postgres://..." make seed
```

## Updating Seed Data

Before changing `backend/seed/seed.sql`:

1. Confirm the change supports a real demo workflow.
2. Keep UUIDs deterministic when existing docs or tests refer to them.
3. Avoid deleting demo rows that other demo flows depend on unless those flows are updated in the same change.
4. Run the backend tests and frontend build after changes.

## Production Use

Do not run `make seed` against production.

If a production environment needs baseline reference data:

1. Write a reviewed migration or import procedure.
2. Document the source and rollback behavior.
3. Run a database backup first.
4. Record the operator, timestamp, and data source.
