---
name: consflow-docker-ops
description: Operate or modify Consflow Docker Compose and Makefile workflows, including admin frontend container, backend startup migrations, Postgres, Redis, logs, and restart pipeline.
---

# Consflow Docker Ops

Use this skill for Docker, Makefile, ports, environment variables, or service startup issues.

## Stack Contract

- Frontend service/container name: `admin`
- Frontend port: `5173`
- Backend service: `backend`
- Backend port: `8080`
- PostgreSQL port: `5432`
- Redis port: `6379`
- Backend migrations run on startup when `AUTO_MIGRATE=true`
- Backend migration directory in container: `/app/migrations`

## Make Pipeline

Preserve these commands:

```bash
make up
make down
make restart
make log
make log SERVICE=admin
```

`make up` and `make restart` must run `down -> build -> run` in that order.

## Workflow

1. Read `Makefile` and `docker-compose.yml`.
2. Check whether the task changes service name, port, build context, env var, volume, or startup order.
3. For Docker or Makefile code/config changes, present a review package and wait for explicit user confirmation before implementation.
4. Validate Compose shape before running the stack:
   ```bash
   docker compose config
   ```
5. For runtime validation, use the Make targets unless a narrower Docker command is required.
6. If Docker socket access is sandboxed, request scoped approval with the exact reason.

## Verification

```bash
docker compose config
make restart
docker compose ps
curl -s -I http://127.0.0.1:5173
curl -s http://127.0.0.1:8080/api/health
```

Run only the checks needed for the change.

## Output Expectations

- Report service names and ports affected.
- Mention whether migrations ran.
- Mention whether `admin` and `backend` are up.
- Include the URL for manual admin testing when relevant.
