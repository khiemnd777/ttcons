#!/usr/bin/env bash
set -Eeuo pipefail

release_dir="$(readlink -f /opt/consflow/current)"
[[ "$release_dir" == /opt/consflow/releases/* ]] || exit 1
release_file="$release_dir/release.env"
test -r "$release_file"
release_tag="$(sed -n 's/^RELEASE_TAG=//p' "$release_file")"
backend_image="$(sed -n 's/^BACKEND_IMAGE=//p' "$release_file")"
admin_image="$(sed -n 's/^ADMIN_IMAGE=//p' "$release_file")"
[[ "$release_tag" == 'v2026-10-20.10' ]] || { echo 'Unexpected production release' >&2; exit 1; }
image_pattern='^ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}$'
[[ "$backend_image" =~ $image_pattern && "$admin_image" =~ $image_pattern ]] || exit 1
export RELEASE_TAG="$release_tag" BACKEND_IMAGE="$backend_image" ADMIN_IMAGE="$admin_image"
compose=(docker compose --project-name consflow-prod --env-file /opt/consflow/shared/.env.prod -f "$release_dir/docker-compose.prod.yml")

backup_dir=/opt/consflow/shared/backups
install -d -m 0700 "$backup_dir"
backup_file="$backup_dir/pre-demo-identity-cleanup-v158-$(date -u +%Y%m%dT%H%M%SZ).dump"
archive_db="consflow_identity_archive_audit_$(date -u +%Y%m%d%H%M%S)_$$"
[[ "$archive_db" =~ ^[a-z0-9_]+$ ]] || exit 1
backend_id="$("${compose[@]}" ps --all --quiet backend)"
admin_id="$("${compose[@]}" ps --all --quiet admin)"
[[ "$backend_id" =~ ^[a-f0-9]{64}$ && "$admin_id" =~ ^[a-f0-9]{64}$ ]] || exit 1
stopped=0
archive_created=0

cleanup() {
  result=$?
  trap - EXIT
  if (( archive_created == 1 )); then
    if ! "${compose[@]}" exec -T postgres sh -ec \
      'exec dropdb --if-exists -U "$POSTGRES_USER" "$1"' sh "$archive_db" < /dev/null; then
      echo 'Could not remove temporary archive database' >&2
      result=1
    fi
  fi
  if (( stopped == 1 )); then
    if ! docker start "$backend_id" "$admin_id" >/dev/null; then
      echo 'Could not restart original app containers' >&2
      result=1
    fi
  fi
  exit "$result"
}
trap cleanup EXIT

# Preflight returns counts only and refuses a changed production state.
"${compose[@]}" exec -T postgres sh -ec \
  'exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
DO $$
DECLARE protected_users integer;
BEGIN
  IF (SELECT max(version::integer) FROM schema_migrations) <> 158
     OR (SELECT count(*) FROM schema_migrations) <> 158
     OR (SELECT count(*) FROM projects) <> 0
     OR (SELECT count(*) FROM pipeline_runs) <> 0
     OR (SELECT count(*) FROM sample_data_generations) <> 0
     OR (SELECT count(*) FROM users) <> 23
     OR (SELECT count(*) FROM staff) <> 23
     OR (SELECT count(*) FROM audit_logs) <> 47
     OR (SELECT count(*) FROM event_logs) <> 4
     OR (SELECT count(*) FROM staff_account_imports) <> 1
     OR (SELECT count(*) FROM notification_actor_revisions) <> 8
     OR (SELECT count(*) FROM project_explorer_invalidation_outbox) <> 42 THEN
    RAISE EXCEPTION 'Production identity cleanup preflight counts changed';
  END IF;
  SELECT count(*) INTO protected_users FROM users account
  WHERE account.status='active'
    AND EXISTS (SELECT 1 FROM staff person WHERE person.user_id=account.id AND person.status='active')
    AND EXISTS (
      SELECT 1 FROM user_roles assignment JOIN roles role ON role.id=assignment.role_id
      WHERE assignment.user_id=account.id AND assignment.project_id IS NULL
        AND role.system_role AND role.name IN ('admin','developer')
    );
  IF protected_users <> 3 THEN
    RAISE EXCEPTION 'Protected Admin/Developer identity count changed';
  END IF;
END $$;
SQL

umask 077
"${compose[@]}" exec -T postgres sh -ec \
  'exec pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < /dev/null > "$backup_file"
test -s "$backup_file"
"${compose[@]}" exec -T postgres pg_restore --list < "$backup_file" > /dev/null
"${compose[@]}" exec -T postgres sh -ec \
  'exec createdb -U "$POSTGRES_USER" "$1"' sh "$archive_db" < /dev/null
archive_created=1
"${compose[@]}" exec -T postgres sh -ec \
  'exec pg_restore --exit-on-error --no-owner --no-acl -U "$POSTGRES_USER" -d "$1"' \
  sh "$archive_db" < "$backup_file"
"${compose[@]}" exec -T postgres sh -ec \
  'exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' \
  sh "$archive_db" <<'SQL'
DO $$
BEGIN
  IF (SELECT count(*) FROM users) <> 23 OR (SELECT count(*) FROM staff) <> 23
     OR (SELECT max(version::integer) FROM schema_migrations) <> 158 THEN
    RAISE EXCEPTION 'Identity cleanup archive restore mismatch';
  END IF;
END $$;
SQL
"${compose[@]}" exec -T postgres sh -ec \
  'exec dropdb -U "$POSTGRES_USER" "$1"' sh "$archive_db" < /dev/null
archive_created=0
echo 'pre_identity_cleanup_archive_restore=verified'

stopped=1
docker stop "$backend_id" "$admin_id" >/dev/null
"${compose[@]}" exec -T postgres sh -ec \
  'exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
BEGIN;
SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='120s';
LOCK TABLE users, staff, audit_logs, auth_sessions, event_logs,
  notification_actor_revisions, staff_account_imports,
  project_explorer_invalidation_outbox IN EXCLUSIVE MODE;
CREATE TEMP TABLE protected_users ON COMMIT DROP AS
SELECT account.id FROM users account
WHERE account.status='active'
  AND EXISTS (SELECT 1 FROM staff person WHERE person.user_id=account.id AND person.status='active')
  AND EXISTS (
    SELECT 1 FROM user_roles assignment JOIN roles role ON role.id=assignment.role_id
    WHERE assignment.user_id=account.id AND assignment.project_id IS NULL
      AND role.system_role AND role.name IN ('admin','developer')
  );
CREATE UNIQUE INDEX ON protected_users(id);
CREATE TEMP TABLE protected_staff ON COMMIT DROP AS
SELECT person.id FROM staff person JOIN protected_users account ON account.id=person.user_id;
CREATE UNIQUE INDEX ON protected_staff(id);
DO $$
DECLARE dependency record; target_rows bigint;
BEGIN
  IF (SELECT count(*) FROM protected_users) <> 3
     OR (SELECT count(*) FROM protected_staff) <> 3
     OR (SELECT count(*) FROM users) <> 23
     OR (SELECT count(*) FROM staff) <> 23
     OR (SELECT count(*) FROM projects) <> 0
     OR (SELECT count(*) FROM pipeline_runs) <> 0
     OR (SELECT count(*) FROM audit_logs) <> 47
     OR (SELECT count(*) FROM event_logs) <> 4
     OR (SELECT count(*) FROM staff_account_imports) <> 1
     OR (SELECT count(*) FROM notification_actor_revisions) <> 8
     OR (SELECT count(*) FROM project_explorer_invalidation_outbox) <> 42 THEN
    RAISE EXCEPTION 'Identity cleanup transaction guard changed';
  END IF;
  FOR dependency IN
    SELECT fk.conrelid::regclass AS child_table, col.attname AS child_column,
           fk.confrelid::regclass AS parent_table, fk.confdeltype AS delete_action
    FROM pg_constraint fk
    JOIN pg_attribute col ON col.attrelid=fk.conrelid AND col.attnum=fk.conkey[1]
    WHERE fk.contype='f' AND cardinality(fk.conkey)=1
      AND fk.confrelid IN ('public.users'::regclass,'public.staff'::regclass)
  LOOP
    IF dependency.delete_action='c' OR dependency.delete_action='n' THEN CONTINUE; END IF;
    IF dependency.child_table IN ('public.staff'::regclass,'public.audit_logs'::regclass) THEN CONTINUE; END IF;
    IF dependency.parent_table='public.users'::regclass THEN
      EXECUTE format('SELECT count(*) FROM %s child JOIN users target ON child.%I=target.id WHERE NOT EXISTS (SELECT 1 FROM protected_users p WHERE p.id=target.id)',
        dependency.child_table,dependency.child_column) INTO target_rows;
    ELSE
      EXECUTE format('SELECT count(*) FROM %s child JOIN staff target ON child.%I=target.id WHERE NOT EXISTS (SELECT 1 FROM protected_staff p WHERE p.id=target.id)',
        dependency.child_table,dependency.child_column) INTO target_rows;
    END IF;
    IF target_rows>0 THEN
      RAISE EXCEPTION 'Unexpected dependent rows in % for %: %',dependency.child_table,dependency.parent_table,target_rows;
    END IF;
  END LOOP;
END $$;
DELETE FROM audit_logs;
DELETE FROM event_logs;
DELETE FROM auth_sessions;
DELETE FROM staff_account_imports;
DELETE FROM notification_actor_revisions;
DELETE FROM staff person WHERE NOT EXISTS (SELECT 1 FROM protected_staff p WHERE p.id=person.id);
DELETE FROM users account WHERE NOT EXISTS (SELECT 1 FROM protected_users p WHERE p.id=account.id);
DELETE FROM project_explorer_invalidation_outbox;
DO $$
BEGIN
  IF (SELECT count(*) FROM users) <> 3 OR (SELECT count(*) FROM staff) <> 3
     OR (SELECT count(*) FROM audit_logs) <> 0 OR (SELECT count(*) FROM event_logs) <> 0
     OR (SELECT count(*) FROM auth_sessions) <> 0
     OR (SELECT count(*) FROM staff_account_imports) <> 0
     OR (SELECT count(*) FROM notification_actor_revisions) <> 0
     OR (SELECT count(*) FROM project_explorer_invalidation_outbox) <> 0
     OR (SELECT count(*) FROM project_explorer_actor_versions) <> 3
     OR (SELECT count(*) FROM pipeline_templates) <> 10
     OR (SELECT count(*) FROM workflow_form_templates) <> 59
     OR (SELECT count(*) FROM workflow_form_versions) <> 145 THEN
    RAISE EXCEPTION 'Identity cleanup postcondition failed';
  END IF;
END $$;
COMMIT;
SQL
echo 'demo_identity_cleanup=committed'
