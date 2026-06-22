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

## External Database Backup

Use `DATABASE_URL` from the deployment environment:

```bash
mkdir -p backups
pg_dump "$DATABASE_URL" -Fc > "backups/consflow-$(date +%Y%m%d-%H%M%S).dump"
```

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

Confirm the target is correct before running destructive restore commands:

```bash
pg_restore --clean --if-exists --no-owner --dbname "$DATABASE_URL" backups/consflow-YYYYMMDD-HHMMSS.dump
```

For plain SQL:

```bash
psql "$DATABASE_URL" -f backups/consflow-YYYYMMDD-HHMMSS.sql
```

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

If the app starts with `AUTO_MIGRATE=true`, confirm migration logs show expected applied/skipped counts.
