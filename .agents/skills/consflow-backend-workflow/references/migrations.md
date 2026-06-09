# Consflow Custom Migration Contract

Consflow does not use Flyway.

Migration source:

- SQL files live in `backend/migrations`.
- Runtime code lives in `backend/internal/shared/migrations/runner.go`.
- Server startup runs migrations when `AUTO_MIGRATE=true`.
- The standalone command is `backend/cmd/migrate`.
- Docker copies migrations into `/app/migrations`.

Filename rules:

- Preferred: `V<N>__description.sql`
- Accepted by the runner: `V<N>__description.sql`, `<N>__description.sql`, or `<N>_description.sql`
- Versions must be unique.
- Numeric versions sort numerically.

Runtime rules:

- The runner creates `schema_migrations`.
- Each migration runs inside a transaction.
- The runner locks `schema_migrations` while applying one migration.
- A checksum mismatch for an applied version is an error.

Authoring rules:

- Add a new migration for every schema change.
- Do not edit migrations already applied to any shared environment.
- Prefer explicit constraints, indexes, and defaults where domain integrity requires them.
- Keep destructive data migrations out of automatic startup unless the user explicitly accepts the data risk.
