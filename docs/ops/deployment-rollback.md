# Consflow Deployment And Rollback Runbook

This runbook is the operational checklist for deploying Consflow without changing repository behavior or storing production secrets in git.

## Pre-Deploy Checks

1. Confirm the target environment variables are provided outside the repo:
   - `DATABASE_URL`
   - `REDIS_URL`
   - `JWT_SECRET`
   - `FRONTEND_ORIGIN`
   - `UPLOAD_DIR`
   - `MAX_UPLOAD_BYTES`
2. Run local validation:
   ```bash
   make migrate-check
   make test
   git diff --check
   ```
3. If Docker Compose is the target shape, validate config before touching the stack:
   ```bash
   make docker-config
   ```
4. Confirm a fresh PostgreSQL backup exists before applying migrations.
5. Confirm the release artifact or commit SHA is recorded in the deployment ticket or release note.

## Deploy

For the local Docker Compose stack, the supported restart pipeline remains:

```bash
make restart
```

This intentionally runs:

```text
down -> build -> run
```

Do not reorder this sequence without a separate review.

Backend migrations run on startup when `AUTO_MIGRATE=true`. The backend container uses `/app/migrations` as its migration directory.

## Smoke Checks

After the stack starts:

```bash
docker compose ps
curl -s http://127.0.0.1:8080/api/health
curl -s -I http://127.0.0.1:5173
```

Review logs:

```bash
make log SERVICE=backend
make log SERVICE=admin
```

## Rollback

1. Stop the currently deployed stack or service.
2. Redeploy the last known-good image or commit.
3. Restore PostgreSQL only when the failed deployment changed data or ran migrations that cannot remain in place.
4. Preserve the failed deployment logs and migration output before cleanup.
5. Re-run smoke checks against the restored version.

## Migration Failure Handling

If startup fails with a migration checksum mismatch:

1. Do not edit an already-applied migration to make the checksum match.
2. Identify the migration version from the error.
3. Compare the deployed file against the recorded `schema_migrations.checksum_sha256`.
4. Roll back to the version that matches the database, or add a new forward migration after review.

If startup fails while applying a new migration:

1. Confirm whether the transaction rolled back.
2. Inspect `schema_migrations` for the failed version.
3. Restore from backup if partial external effects occurred.
4. Fix with a new reviewed migration or a corrected unreleased migration only when it was never applied.
