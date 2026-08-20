#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/scripts/production-env-lib.sh
. "$script_dir/production-env-lib.sh"

mode="${1:---all}"
case "$mode" in
  --all|--github|--render|--provision) ;;
  *) production_fail "usage: $(basename "$0") [--all|--github|--render|--provision]" ;;
esac

production_load_config
production_require_mode_600 "$production_config_file"

for key in GITHUB_REPOSITORY GITHUB_ENVIRONMENT VPS_HOST VPS_PORT VPS_USER VPS_DEPLOY_ROOT APP_DOMAIN; do
  production_require_value "$key"
done
[[ "$GITHUB_REPOSITORY" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || production_fail "GITHUB_REPOSITORY must be owner/name"
[[ "$VPS_PORT" =~ ^[0-9]+$ && "$VPS_PORT" -ge 1 && "$VPS_PORT" -le 65535 ]] || production_fail "VPS_PORT must be 1..65535"
[[ "$VPS_DEPLOY_ROOT" == /opt/consflow ]] || production_fail "VPS_DEPLOY_ROOT must remain /opt/consflow"
[[ "$APP_DOMAIN" == tt-dnh.com ]] || production_fail "APP_DOMAIN must remain tt-dnh.com"

validate_runtime() {
  local key
  for key in POSTGRES_PASSWORD REDIS_PASSWORD REDIS_CACHE_PASSWORD JWT_SECRET SETTINGS_ENCRYPTION_KEY BOOTSTRAP_ADMIN_EMAIL BOOTSTRAP_ADMIN_FULL_NAME BOOTSTRAP_ADMIN_DEPARTMENT BOOTSTRAP_ADMIN_POSITION BOOTSTRAP_ADMIN_PASSWORD STORAGE_S3_BUCKET STORAGE_S3_REGION STORAGE_S3_ENDPOINT STORAGE_S3_USE_PATH_STYLE S3_ACCESS_KEY S3_SECRET_KEY; do
    production_require_value "$key"
  done
  [[ "$POSTGRES_PASSWORD" =~ ^[A-Za-z0-9._~-]+$ ]] || production_fail "POSTGRES_PASSWORD contains URL-unsafe characters"
  [[ "$REDIS_PASSWORD" =~ ^[A-Za-z0-9._~-]+$ ]] || production_fail "REDIS_PASSWORD contains URL-unsafe characters"
  [[ "$REDIS_CACHE_PASSWORD" =~ ^[A-Za-z0-9._~-]+$ ]] || production_fail "REDIS_CACHE_PASSWORD contains URL-unsafe characters"
  [[ ${#JWT_SECRET} -ge 32 ]] || production_fail "JWT_SECRET must be at least 32 characters"
  [[ ${#SETTINGS_ENCRYPTION_KEY} -ge 32 ]] || production_fail "SETTINGS_ENCRYPTION_KEY must be at least 32 characters"
  [[ ${#BOOTSTRAP_ADMIN_PASSWORD} -ge 8 && ${#BOOTSTRAP_ADMIN_PASSWORD} -le 72 ]] || production_fail "BOOTSTRAP_ADMIN_PASSWORD must be 8..72 bytes"
  [[ "$STORAGE_S3_BUCKET" == tt-dnh-consflow-prod ]] || production_fail "unexpected S3 bucket"
  [[ "$STORAGE_S3_REGION" == vn-hcm-1 ]] || production_fail "unexpected S3 Region"
  [[ "$STORAGE_S3_ENDPOINT" == https://s3.vn-hcm-1.vietnix.cloud ]] || production_fail "unexpected S3 endpoint"
  [[ "$STORAGE_S3_USE_PATH_STYLE" == false || "$STORAGE_S3_USE_PATH_STYLE" == true ]] || production_fail "STORAGE_S3_USE_PATH_STYLE must be true or false"
}

validate_ssh() {
  local private_public public_public fingerprint host_token
  production_require_command ssh-keygen
  production_require_value VPS_HOST_KEY_FINGERPRINT
  production_require_mode_600 "$production_ssh_private_key"
  production_require_mode_600 "$production_ssh_public_key"
  production_require_mode_600 "$production_known_hosts"
  private_public="$(ssh-keygen -y -f "$production_ssh_private_key" | awk 'NF >= 2 { print $1 " " $2; exit }')"
  public_public="$(awk 'NF >= 2 { print $1 " " $2; exit }' "$production_ssh_public_key")"
  [[ "$private_public" == "$public_public" ]] || production_fail "SSH private/public deploy key mismatch"
  fingerprint="$(ssh-keygen -lf "$production_known_hosts" | awk '$NF == "(ED25519)" { print $2; exit }')"
  [[ -n "$fingerprint" ]] || production_fail "known_hosts has no ED25519 host key"
  [[ "$fingerprint" == "$VPS_HOST_KEY_FINGERPRINT" ]] || production_fail "known_hosts fingerprint does not match the console-verified fingerprint"
  host_token="$(production_ssh_host_token)"
  ssh-keygen -F "$host_token" -f "$production_known_hosts" >/dev/null || production_fail "known_hosts does not contain $host_token"
}

case "$mode" in
  --github)
    validate_ssh
    ;;
  --render)
    validate_runtime
    ;;
  --provision|--all)
    validate_runtime
    validate_ssh
    production_require_value GHCR_USERNAME
    production_require_value GHCR_READ_TOKEN
    ;;
esac

printf 'PASS: production config validation (%s)\n' "${mode#--}"
