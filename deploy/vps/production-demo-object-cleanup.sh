#!/usr/bin/env bash
set -Eeuo pipefail

[[ "$(uname -m)" == x86_64 ]] || { echo 'Unexpected VPS architecture' >&2; exit 1; }
release_dir="$(readlink -f /opt/consflow/current)"
[[ "$release_dir" == /opt/consflow/releases/* ]] || exit 1
release_file="$release_dir/release.env"
test -r "$release_file"
release_tag="$(sed -n 's/^RELEASE_TAG=//p' "$release_file")"
backend_image="$(sed -n 's/^BACKEND_IMAGE=//p' "$release_file")"
admin_image="$(sed -n 's/^ADMIN_IMAGE=//p' "$release_file")"
[[ "$release_tag" == v2026-10-20.10 ]] || { echo 'Unexpected production release' >&2; exit 1; }
image_pattern='^ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}$'
[[ "$backend_image" =~ $image_pattern && "$admin_image" =~ $image_pattern ]] || exit 1
export RELEASE_TAG="$release_tag" BACKEND_IMAGE="$backend_image" ADMIN_IMAGE="$admin_image"
compose=(docker compose --project-name consflow-prod --env-file /opt/consflow/shared/.env.prod -f "$release_dir/docker-compose.prod.yml")
operator=/opt/consflow/shared/operators/demo-object-cleanup
[[ -x "$operator" ]] || { echo 'One-time S3 operator missing' >&2; exit 1; }
backend_id="$("${compose[@]}" ps --all --quiet backend)"
admin_id="$("${compose[@]}" ps --all --quiet admin)"
[[ "$backend_id" =~ ^[a-f0-9]{64}$ && "$admin_id" =~ ^[a-f0-9]{64}$ ]] || exit 1
umask 077
workdir="$(mktemp -d /opt/consflow/shared/demo-object-cleanup.XXXXXXXX)"
archive_db="consflow_s3_archive_audit_$(date -u +%Y%m%d%H%M%S)_$$"
[[ "$archive_db" =~ ^[a-z0-9_]+$ ]] || exit 1
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
  rm -rf -- "$workdir"
  rm -f -- "$operator"
  exit "$result"
}
trap cleanup EXIT

archive_file="$(find /opt/consflow/shared/backups -maxdepth 1 -type f \
  -name 'pre-demo-cleanup-v145-*.dump' -print | sort | tail -n 1)"
[[ -n "$archive_file" && -s "$archive_file" ]] || { echo 'Pre-demo archive missing' >&2; exit 1; }
"${compose[@]}" exec -T postgres sh -ec \
  'exec createdb -U "$POSTGRES_USER" "$1"' sh "$archive_db" < /dev/null
archive_created=1
"${compose[@]}" exec -T postgres sh -ec \
  'exec pg_restore --exit-on-error --no-owner --no-acl -U "$POSTGRES_USER" -d "$1"' \
  sh "$archive_db" < "$archive_file"
"${compose[@]}" exec -T postgres sh -ec \
  'exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"' \
  sh "$archive_db" > "$workdir/keys" <<'SQL'
SELECT 'evidence_records' || E'\t' || storage_key
FROM evidence_records WHERE storage_provider='s3' AND storage_key IS NOT NULL
UNION ALL
SELECT 'workflow_form_pdf_exports' || E'\t' || storage_key
FROM workflow_form_pdf_exports WHERE storage_provider='s3' AND storage_key IS NOT NULL
UNION ALL
SELECT 'workflow_form_signatures' || E'\t' || visual_storage_key FROM workflow_form_signatures
WHERE visual_storage_provider='s3' AND visual_storage_key IS NOT NULL
ORDER BY 1;
SQL
"${compose[@]}" exec -T postgres sh -ec \
  'exec dropdb -U "$POSTGRES_USER" "$1"' sh "$archive_db" < /dev/null
archive_created=0

"${compose[@]}" config --format json > "$workdir/compose.json"
"$operator" --check "$workdir/keys" < "$workdir/compose.json"
stopped=1
docker stop "$backend_id" "$admin_id" >/dev/null
"${compose[@]}" exec -T postgres sh -ec \
  'exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
DO $$
BEGIN
  IF (SELECT max(version::integer) FROM schema_migrations) <> 158
     OR (SELECT count(*) FROM users) <> 3
     OR (SELECT count(*) FROM projects) <> 0
     OR (SELECT count(*) FROM evidence_records WHERE storage_provider='s3') <> 0
     OR (SELECT count(*) FROM workflow_form_pdf_exports WHERE storage_provider='s3') <> 0
     OR (SELECT count(*) FROM workflow_form_signatures WHERE visual_storage_provider='s3') <> 0
     OR (SELECT count(*) FROM documents WHERE storage_provider='s3') <> 0
     OR (SELECT count(*) FROM seal_assets WHERE storage_provider='s3') <> 0
     OR (SELECT count(*) FROM users WHERE avatar_storage_provider='s3') <> 0 THEN
    RAISE EXCEPTION 'Current production still references S3 objects or readiness state changed';
  END IF;
END $$;
SQL
"$operator" "$workdir/keys" < "$workdir/compose.json"
