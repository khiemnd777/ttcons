# Consflow Deployment And Rollback Runbook

This runbook is the operational checklist for deploying Consflow without changing repository behavior or storing production secrets in git.

This document keeps the controlled Docker Compose single-host procedure below. For the multi-node Kubernetes application baseline, external managed PostgreSQL/Redis/S3 prerequisites, release-unique migration Job, expand-contract gate, rollout, PDB/topology behavior, secret rotation and rollback checks, use [`deploy/k8s/README.md`](../../deploy/k8s/README.md). Do not mix the two deployment procedures in one release.

## Pre-Deploy Checks

1. Confirm production secret files are provided outside source control and match the paths in `.env.prod`:
   - `POSTGRES_PASSWORD_FILE`
   - `DATABASE_URL_FILE`
   - `REDIS_PASSWORD_FILE`
   - `REDIS_URL_FILE`
   - `JWT_SECRET_FILE`

   The backend accepts either each direct variable or its `_FILE` counterpart, never both. Production Compose uses only `_FILE`; PostgreSQL and Redis password files must match the credentials embedded in their respective URL files.

   Keep the parent secret directory at mode `0700`, but keep each referenced source file at mode `0644`. Compose mounts a `file:` secret read-only with the source file ownership/mode; it cannot remap `uid`, `gid`, or `mode` for file sources. Mode `0644` lets the non-root PostgreSQL, Redis and backend users read the mount, while the non-traversable parent directory remains the host confidentiality boundary. Host root and anyone with Docker socket access remain trusted. See the [Docker Compose secrets reference](https://docs.docker.com/reference/compose-file/services/#secrets).

   Confirm the remaining non-secret environment variables:
   - `FRONTEND_ORIGIN`
   - `STORAGE_PROVIDER=s3`
   - `STORAGE_S3_BUCKET`
   - `STORAGE_S3_REGION`
   - optional `STORAGE_S3_ENDPOINT` and path-style settings
   - `MAX_UPLOAD_BYTES`

   Production does not mount a local object directory. Confirm the S3
   credential chain is available and that backend startup storage preflight can
   create, read, and delete its probe object before returning the origin to the
   load balancer.
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

For production Compose, validate only after the secret files exist and keep rendered configuration out of shared logs:

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml config --quiet
```

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

Local/development may still run backend startup migrations with `AUTO_MIGRATE=true`. Production Compose instead runs the one-shot `migrate` service and starts `backend` only after that service exits successfully. Production fixes backend `AUTO_MIGRATE=false`; do not bypass this dependency when scaling or rolling out replicas.

The migration process holds one PostgreSQL session advisory lock across the complete ordered sequence. `MIGRATION_LOCK_TIMEOUT` bounds how long it waits for another runner, while `MIGRATION_TIMEOUT` bounds connection, lock wait and migration execution together. Lock timeout must be strictly less than total timeout so the runner can emit the specialized lock error first. A lock timeout or non-zero `migrate` exit blocks backend startup and must be investigated before retrying the release.

For an in-place single-host production deploy, use this explicit sequence. A plain full-stack `up -d` does not stop an already-running old backend before a new one-shot migration starts; `depends_on` only gates newly created containers.

1. Remove the origin from the managed edge/load-balancer pool and wait for active requests to drain.
2. Build the new release while the old containers still serve drained traffic:
   ```bash
   docker compose --env-file .env.prod -f docker-compose.prod.yml build
   docker compose --env-file .env.prod -f docker-compose.prod.yml run --rm --no-deps --entrypoint /bin/sh backend -ec 'test -r /run/secrets/database_url && test -r /run/secrets/redis_url && test -r /run/secrets/redis_cache_url && test -r /run/secrets/jwt_secret'
   docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T --user postgres postgres test -r /run/secrets/postgres_password
   docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T --user redis redis test -r /run/secrets/redis_password
   ```
   These checks execute as the actual non-root container users and disclose no secret content.
3. Stop the old gateway/backend gracefully. Expect API downtime from this point until the new backend and gateway are healthy. Zero-downtime rollout requires a separately reviewed second origin and backward-compatible migrations.
   ```bash
   docker compose --env-file .env.prod -f docker-compose.prod.yml stop -t 30 admin backend
   docker compose --env-file .env.prod -f docker-compose.prod.yml ps postgres redis
   ```
4. Run only the migration service in the foreground and stop immediately if its exit code is non-zero. `--no-deps` intentionally leaves the already-running PostgreSQL/Redis containers untouched.
   ```bash
   docker compose --env-file .env.prod -f docker-compose.prod.yml up --no-deps --force-recreate --exit-code-from migrate migrate
   docker compose --env-file .env.prod -f docker-compose.prod.yml ps migrate
   ```
5. Only after migration success, recreate backend without re-running dependencies, wait for it to report healthy, then recreate the gateway:
   ```bash
   docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --no-deps --force-recreate backend
   docker compose --env-file .env.prod -f docker-compose.prod.yml ps backend
   docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T backend wget -q -O /dev/null http://127.0.0.1:8080/api/ready
   docker compose --env-file .env.prod -f docker-compose.prod.yml up -d --no-deps --force-recreate admin
   ```
6. Complete smoke checks, then return the origin to the managed edge/load-balancer pool.

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

For production Compose, also retain the one-shot migration result without rendering secret configuration:

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml ps migrate
docker compose --env-file .env.prod -f docker-compose.prod.yml logs --no-log-prefix migrate
```

## Rollback

1. Stop the currently deployed stack or service.
2. Confirm the target rollback image contains every migration version already recorded in `schema_migrations`. The runner fails closed when the database contains a version absent from the image manifest; in that case restore a compatible backup or deploy a forward-compatible image instead of forcing the old backend to start.
3. Redeploy the compatible last known-good image or commit using the same stop -> migrate -> backend -> admin sequence.
4. Restore the last known-good secret-file set or secret-manager version together with that release; preserve the `0700` parent / `0644` source-file permission contract and do not fall back to plaintext secret environment variables.
5. Restore PostgreSQL only when the failed deployment changed data or ran migrations that cannot remain in place.
6. Preserve the failed deployment logs and migration output before cleanup. Never attach secret files or rendered secret values.
7. Re-run smoke checks against the restored version.

## Migration Failure Handling

If startup fails with a migration checksum mismatch:

1. Do not edit an already-applied migration to make the checksum match.
2. Identify the migration version from the error.
3. Compare the deployed file against the recorded `schema_migrations.checksum_sha256`.
4. Roll back to the version that matches the database, or add a new forward migration after review.

If startup fails while applying a new migration:

1. Inspect the one-shot `migrate` service exit status and logs; do not start backend manually around a failed dependency.
2. Confirm whether the transaction rolled back.
3. Inspect `schema_migrations` for the failed version.
4. Restore from backup if partial external effects occurred.
5. Fix with a new reviewed migration or a corrected unreleased migration only when it was never applied.
