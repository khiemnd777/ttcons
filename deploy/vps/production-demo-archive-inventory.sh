#!/usr/bin/env bash
set -Eeuo pipefail

release_dir="$(readlink -f /opt/consflow/current)"
[[ "$release_dir" == /opt/consflow/releases/* ]] || exit 1
release_file="$release_dir/release.env"
test -r "$release_file"
release_tag="$(sed -n 's/^RELEASE_TAG=//p' "$release_file")"
backend_image="$(sed -n 's/^BACKEND_IMAGE=//p' "$release_file")"
admin_image="$(sed -n 's/^ADMIN_IMAGE=//p' "$release_file")"
[[ "$release_tag" =~ ^v[0-9]{4}-[0-9]{2}-[0-9]{2}\.[1-9][0-9]*$ ]] || exit 1
image_pattern='^ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}$'
[[ "$backend_image" =~ $image_pattern && "$admin_image" =~ $image_pattern ]] || exit 1
export RELEASE_TAG="$release_tag" BACKEND_IMAGE="$backend_image" ADMIN_IMAGE="$admin_image"
compose=(docker compose --project-name consflow-prod --env-file /opt/consflow/shared/.env.prod -f "$release_dir/docker-compose.prod.yml")

backup_file="$(find /opt/consflow/shared/backups -maxdepth 1 -type f \
  -name 'pre-demo-cleanup-v145-*.dump' -print | sort | tail -n 1)"
[[ -n "$backup_file" && -s "$backup_file" ]] || { echo 'No pre-cleanup archive found' >&2; exit 1; }
archive_db="consflow_demo_archive_audit_$(date -u +%Y%m%d%H%M%S)_$$"
[[ "$archive_db" =~ ^[a-z0-9_]+$ ]] || exit 1

drop_archive_db() {
  result=$?
  trap - EXIT
  if ! "${compose[@]}" exec -T postgres sh -ec \
    'exec dropdb --if-exists -U "$POSTGRES_USER" "$1"' sh "$archive_db" < /dev/null; then
    echo 'Could not remove temporary archive audit database' >&2
    exit 1
  fi
  if (( result == 0 )); then
    echo 'archive_restore_and_object_counts=verified'
  fi
  exit "$result"
}

"${compose[@]}" exec -T postgres sh -ec \
  'exec createdb -U "$POSTGRES_USER" "$1"' sh "$archive_db" < /dev/null
trap drop_archive_db EXIT
"${compose[@]}" exec -T postgres sh -ec \
  'exec pg_restore --exit-on-error --no-owner --no-acl -U "$POSTGRES_USER" -d "$1"' \
  sh "$archive_db" < "$backup_file"

"${compose[@]}" exec -T postgres sh -ec \
  'exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' \
  sh "$archive_db" <<'SQL'
BEGIN TRANSACTION READ ONLY;
SELECT 'archive_latest_migration=' || max(version::integer) FROM schema_migrations;
SELECT 'archive_storage=' || family || ':' || provider || ':' || count(*)
FROM (
  SELECT 'evidence_records' AS family,storage_provider AS provider FROM evidence_records
    WHERE storage_provider IN ('local','s3') AND storage_key IS NOT NULL
  UNION ALL
  SELECT 'workflow_form_signatures',visual_storage_provider FROM workflow_form_signatures
    WHERE visual_storage_provider IN ('local','s3') AND visual_storage_key IS NOT NULL
  UNION ALL
  SELECT 'workflow_form_pdf_exports',storage_provider FROM workflow_form_pdf_exports
    WHERE storage_provider IN ('local','s3') AND storage_key IS NOT NULL
  UNION ALL
  SELECT 'documents',storage_provider FROM documents
    WHERE storage_provider IN ('local','s3') AND storage_key IS NOT NULL
  UNION ALL
  SELECT 'seal_assets',storage_provider FROM seal_assets
    WHERE storage_provider IN ('local','s3') AND storage_key IS NOT NULL
  UNION ALL
  SELECT 'users_avatar',avatar_storage_provider FROM users
    WHERE avatar_storage_provider IN ('local','s3') AND avatar_storage_key IS NOT NULL
) AS archived_objects
GROUP BY family,provider ORDER BY family,provider;
COMMIT;
SQL
