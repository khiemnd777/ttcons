# Consflow PostgreSQL Backup And Restore Runbook

This runbook keeps backup and restore steps explicit while avoiding production credentials in the repository.

## Local Docker Backup

Create a timestamped custom-format backup from the local Compose PostgreSQL service:

```bash
mkdir -p backups
docker compose exec -T postgres pg_dump -U consflow -d consflow -Fc > "backups/consflow-$(date +%Y%m%d-%H%M%S).dump"
```

For a plain SQL backup:

```bash
mkdir -p backups
docker compose exec -T postgres pg_dump -U consflow -d consflow > "backups/consflow-$(date +%Y%m%d-%H%M%S).sql"
```

Do not commit backup files.

## Production Compose Backup

Production Compose keeps the database URL/password in mounted secret files. Run `pg_dump` inside the PostgreSQL container with non-secret database/user identifiers; adjust the identifiers if `.env.prod` uses names other than the example values.

```bash
mkdir -p backups
docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres pg_dump -U consflow -d consflow -Fc > "backups/consflow-$(date +%Y%m%d-%H%M%S).dump"
```

## External Database Backup

Have the platform secret manager mount an owner-readable libpq service file and password file, then set `PGSERVICEFILE` and `PGPASSFILE` through its secret-aware process wrapper. The command line and shared logs should contain only the non-secret service name, never the database DSN or password.

```bash
mkdir -p backups
PGSERVICE=consflow-production pg_dump --dbname="service=consflow-production" -Fc > "backups/consflow-$(date +%Y%m%d-%H%M%S).dump"
```

Kubernetes production uses this external-database contract; the application manifests deliberately contain no PostgreSQL StatefulSet/PVC. Prefer the managed provider's encrypted snapshot/PITR workflow, record the snapshot/restore point in the release ticket, and run an independent restore drill. Do not launch a utility pod whose command or logs expose `DATABASE_URL`. The Kubernetes rollout/migration/rollback sequence is documented in [`deploy/k8s/README.md`](../../deploy/k8s/README.md).

Before production deploys that can affect schema or data, record:

- Backup filename
- Database host or environment name
- Application commit SHA
- Operator
- UTC timestamp

## Restore Into Local Docker

For a custom-format dump:

```bash
docker compose exec -T postgres dropdb -U consflow --if-exists consflow
docker compose exec -T postgres createdb -U consflow consflow
cat backups/consflow-YYYYMMDD-HHMMSS.dump | docker compose exec -T postgres pg_restore -U consflow -d consflow --clean --if-exists
```

For a plain SQL dump:

```bash
docker compose exec -T postgres dropdb -U consflow --if-exists consflow
docker compose exec -T postgres createdb -U consflow consflow
cat backups/consflow-YYYYMMDD-HHMMSS.sql | docker compose exec -T postgres psql -U consflow -d consflow
```

## Restore Into External Database

Confirm the target service name and backup are correct before running destructive restore commands. Use the same platform-mounted `PGSERVICEFILE`/`PGPASSFILE` secret-aware wrapper as backup; do not put a DSN in shell history or process arguments.

```bash
PGSERVICE=consflow-production pg_restore --clean --if-exists --no-owner --dbname="service=consflow-production" backups/consflow-YYYYMMDD-HHMMSS.dump
```

For plain SQL:

```bash
PGSERVICE=consflow-production psql --dbname="service=consflow-production" -f backups/consflow-YYYYMMDD-HHMMSS.sql
```

## Restore Into Production Compose

Remove the origin from the edge/load-balancer, stop `admin` and `backend`, and confirm the exact backup before this destructive operation. PostgreSQL and Redis containers remain managed by Compose; credentials stay in mounted secret files.

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml stop -t 30 admin backend
docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres dropdb -U consflow --if-exists consflow
docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres createdb -U consflow consflow
cat backups/consflow-YYYYMMDD-HHMMSS.dump | docker compose --env-file .env.prod -f docker-compose.prod.yml exec -T postgres pg_restore -U consflow -d consflow --clean --if-exists
```

After restore, run the reviewed one-shot migration service in the foreground. Start backend/admin and return the origin to the edge pool only after the migration exits zero and readiness passes.

## Post-Restore Checks

Run:

```bash
cd backend
MIGRATIONS_DIR=migrations go run ./cmd/migrate -check
```

Then start the app and check:

```bash
curl -s http://127.0.0.1:8080/api/health
```

For local/development startup with `AUTO_MIGRATE=true`, confirm backend migration logs show expected applied/skipped counts. For production Compose, run or restart the dedicated `migrate` service first, confirm its zero exit status and applied/skipped logs, then allow backend startup through the `service_completed_successfully` dependency.
