#!/usr/bin/env bash
set -Eeuo pipefail

release_tag="${1:-}"
backend_image="${2:-}"
admin_image="${3:-}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
release_dir="$(cd "$script_dir/../.." && pwd)"
deploy_root="/opt/consflow"
shared_dir="$deploy_root/shared"
env_file="$shared_dir/.env.prod"
compose_file="$release_dir/docker-compose.prod.yml"

bash "$release_dir/scripts/validate-release-tag.sh" "$release_tag" >/dev/null
if [[ "$(basename "$release_dir")" != "$release_tag" ]]; then
  echo "release directory must end with the validated release tag" >&2
  exit 1
fi
image_pattern='^ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}$'
if [[ ! "$backend_image" =~ $image_pattern || ! "$admin_image" =~ $image_pattern ]]; then
  echo "backend and admin images must be immutable GHCR sha256 references" >&2
  exit 1
fi
if [[ ! -r "$env_file" ]]; then
  echo "missing readable production environment file: $env_file" >&2
  exit 1
fi
if [[ ! -f "$compose_file" ]]; then
  echo "missing production Compose file in release bundle" >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required on the VPS" >&2
  exit 1
fi

export RELEASE_TAG="$release_tag"
export BACKEND_IMAGE="$backend_image"
export ADMIN_IMAGE="$admin_image"
compose=(docker compose --project-name consflow-prod --env-file "$env_file" -f "$compose_file")

deployment_diagnostics() {
  echo "production deployment failed; database rollback was not attempted" >&2
  "${compose[@]}" ps >&2 || true
  "${compose[@]}" logs --tail 80 migrate bootstrap-admin backend admin >&2 || true
}
trap deployment_diagnostics ERR

"${compose[@]}" config --quiet
"${compose[@]}" pull migrate bootstrap-admin backend admin
"${compose[@]}" up -d --wait --wait-timeout 120 postgres redis redis-cache

# Check the runtime UID can read each mounted secret without printing content.
"${compose[@]}" run --rm --no-deps --entrypoint /bin/sh backend -ec '
  test -r /run/secrets/database_url
  test -r /run/secrets/redis_url
  test -r /run/secrets/redis_cache_url
  test -r /run/secrets/jwt_secret
  test -r /run/secrets/settings_encryption_key
  test -r /run/secrets/aws_credentials
'

# Single-host migrations require an explicit drain. Never migrate under the old
# backend and never auto-rollback a database after a partial release failure.
"${compose[@]}" stop -t 30 admin backend
"${compose[@]}" run --rm --no-deps migrate
"${compose[@]}" --profile bootstrap run --rm --no-deps bootstrap-admin

"${compose[@]}" up -d --no-deps --force-recreate backend
backend_ready=false
for _ in $(seq 1 24); do
  if "${compose[@]}" exec -T backend wget -q -O /dev/null http://127.0.0.1:8080/api/ready; then
    backend_ready=true
    break
  fi
  sleep 5
done
if [[ "$backend_ready" != true ]]; then
  echo "backend did not become ready within 120 seconds" >&2
  exit 1
fi

"${compose[@]}" up -d --no-deps --force-recreate admin
admin_ready=false
for _ in $(seq 1 24); do
  if "${compose[@]}" exec -T admin wget -q -O /dev/null http://127.0.0.1:8081/gateway-health; then
    admin_ready=true
    break
  fi
  sleep 5
done
if [[ "$admin_ready" != true ]]; then
  echo "Caddy did not become healthy within 120 seconds" >&2
  exit 1
fi

current_link="$deploy_root/current"
if [[ -e "$current_link" && ! -L "$current_link" ]]; then
  echo "$current_link exists and is not a symlink; refusing to overwrite it" >&2
  exit 1
fi
umask 077
printf 'RELEASE_TAG=%s\nBACKEND_IMAGE=%s\nADMIN_IMAGE=%s\n' \
  "$release_tag" "$backend_image" "$admin_image" > "$release_dir/release.env"
ln -sfn "$release_dir" "$current_link"
trap - ERR
"${compose[@]}" ps
echo "deployed release $release_tag"
