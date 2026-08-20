#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../.." && pwd)"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/consflow-production-tooling-test.XXXXXX")"
trap 'rm -rf "$test_root"' EXIT

export PRODUCTION_CONFIG_FILE="$test_root/production.env"
export PRODUCTION_SSH_PRIVATE_KEY="$test_root/production_id_ed25519"
export PRODUCTION_SSH_PUBLIC_KEY="$test_root/production_id_ed25519.pub"
export PRODUCTION_KNOWN_HOSTS="$test_root/production_known_hosts"

file_sha256() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

bash "$script_dir/bootstrap-production.sh" --skip-host-scan >/dev/null
before_config="$(file_sha256 "$PRODUCTION_CONFIG_FILE")"
before_key="$(file_sha256 "$PRODUCTION_SSH_PRIVATE_KEY")"
bash "$script_dir/bootstrap-production.sh" --skip-host-scan >/dev/null
[[ "$before_config" == "$(file_sha256 "$PRODUCTION_CONFIG_FILE")" ]]
[[ "$before_key" == "$(file_sha256 "$PRODUCTION_SSH_PRIVATE_KEY")" ]]

if bash "$script_dir/validate-production-config.sh" --all >/dev/null 2>&1; then
  echo "expected placeholder production config to fail closed" >&2
  exit 1
fi

# Use the generated public key only as deterministic test host-key material. This
# does not model trust; it lets the validator exercise exact host/fingerprint pinning.
awk 'NF >= 2 { print "160.30.113.13 " $1 " " $2; exit }' "$PRODUCTION_SSH_PUBLIC_KEY" > "$PRODUCTION_KNOWN_HOSTS"
chmod 600 "$PRODUCTION_KNOWN_HOSTS"

# shellcheck source=deploy/scripts/production-env-lib.sh
. "$script_dir/production-env-lib.sh"
production_set_config_value VPS_HOST_KEY_FINGERPRINT "$(ssh-keygen -lf "$PRODUCTION_KNOWN_HOSTS" | awk '$NF == "(ED25519)" { print $2; exit }')"
production_set_config_value GHCR_READ_TOKEN test-read-packages-token
production_set_config_value S3_ACCESS_KEY test-vietnix-access-key
production_set_config_value S3_SECRET_KEY test-vietnix-secret-key
production_load_config

bash "$script_dir/validate-production-config.sh" --all
bash "$script_dir/configure-github-secrets.sh" --scope repository --dry-run >/dev/null
bash "$script_dir/configure-github-secrets.sh" --scope environment --dry-run >/dev/null
bash "$script_dir/provision-vps.sh" --dry-run >/dev/null

rendered="$test_root/rendered"
bash "$script_dir/render-production-config.sh" "$rendered" >/dev/null
[[ -z "$(find "$rendered" -type f ! -perm 0600 -print)" ]]
grep -Fxq 'RELEASE_TAG=' "$rendered/shared/.env.prod"
grep -Fxq 'BACKEND_IMAGE=' "$rendered/shared/.env.prod"
grep -Fxq 'ADMIN_IMAGE=' "$rendered/shared/.env.prod"
grep -Fxq 'STORAGE_S3_REGION=vn-hcm-1' "$rendered/shared/.env.prod"
grep -Fxq 'STORAGE_S3_BUCKET=tt-dnh-consflow-prod' "$rendered/shared/.env.prod"
grep -Fxq 'STORAGE_S3_ENDPOINT=https://s3.vn-hcm-1.vietnix.cloud' "$rendered/shared/.env.prod"
[[ "$(tr -d '\r\n' < "$rendered/shared/secrets/postgres_password")" == "$POSTGRES_PASSWORD" ]]
[[ "$(tr -d '\r\n' < "$rendered/shared/secrets/bootstrap_admin_password")" == "$BOOTSTRAP_ADMIN_PASSWORD" ]]

env \
  RELEASE_TAG=v2026-08-20.1 \
  BACKEND_IMAGE=ghcr.io/khiemnd777/ttcons-backend@sha256:0000000000000000000000000000000000000000000000000000000000000000 \
  ADMIN_IMAGE=ghcr.io/khiemnd777/ttcons-admin@sha256:0000000000000000000000000000000000000000000000000000000000000000 \
  docker compose --env-file "$rendered/shared/.env.prod" -f "$repo_root/docker-compose.prod.yml" config --quiet

release_remote="$test_root/release-remote.git"
release_repo="$test_root/release-repo"
git init --bare --initial-branch=main "$release_remote" >/dev/null
git init -b main "$release_repo" >/dev/null
mkdir -p "$release_repo/deploy/scripts" "$release_repo/scripts"
cp "$script_dir/release-tag.sh" "$release_repo/deploy/scripts/release-tag.sh"
cp "$repo_root/scripts/validate-release-tag.sh" "$release_repo/scripts/validate-release-tag.sh"
git -C "$release_repo" config user.name 'Consflow Test'
git -C "$release_repo" config user.email 'consflow-test@example.invalid'
git -C "$release_repo" add deploy/scripts/release-tag.sh scripts/validate-release-tag.sh
git -C "$release_repo" commit -m 'production tooling fixture' >/dev/null
git -C "$release_repo" remote add origin "$release_remote"
git -C "$release_repo" push -u origin main >/dev/null 2>&1
(
  cd "$release_repo"
  bash deploy/scripts/release-tag.sh v2026-08-20.1 >/dev/null 2>&1
)
git --git-dir="$release_remote" rev-parse -q --verify refs/tags/v2026-08-20.1 >/dev/null

echo "PASS: production operator tooling is idempotent, fail-closed, secret-safe and tag-gated"
