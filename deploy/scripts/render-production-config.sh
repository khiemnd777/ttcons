#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/scripts/production-env-lib.sh
. "$script_dir/production-env-lib.sh"

output_root="${1:-}"
[[ -n "$output_root" ]] || production_fail "usage: $(basename "$0") OUTPUT_ROOT"
bash "$script_dir/validate-production-config.sh" --render >/dev/null
production_load_config

umask 077
mkdir -p "$output_root/shared/secrets"
env_output="$output_root/shared/.env.prod"

render_value() {
  local key="$1"
  case "$key" in
    RELEASE_TAG|BACKEND_IMAGE|ADMIN_IMAGE) printf '' ;;
    APP_DOMAIN) printf '%s' "$APP_DOMAIN" ;;
    FRONTEND_ORIGIN) printf 'https://%s' "$APP_DOMAIN" ;;
    POSTGRES_PASSWORD_FILE) printf '/opt/consflow/shared/secrets/postgres_password' ;;
    DATABASE_URL_FILE) printf '/opt/consflow/shared/secrets/database_url' ;;
    REDIS_PASSWORD_FILE) printf '/opt/consflow/shared/secrets/redis_password' ;;
    REDIS_URL_FILE) printf '/opt/consflow/shared/secrets/redis_url' ;;
    REDIS_CACHE_PASSWORD_FILE) printf '/opt/consflow/shared/secrets/redis_cache_password' ;;
    REDIS_CACHE_URL_FILE) printf '/opt/consflow/shared/secrets/redis_cache_url' ;;
    JWT_SECRET_FILE) printf '/opt/consflow/shared/secrets/jwt_secret' ;;
    SETTINGS_ENCRYPTION_KEY_FILE) printf '/opt/consflow/shared/secrets/settings_encryption_key' ;;
    AWS_CREDENTIALS_FILE) printf '/opt/consflow/shared/secrets/aws_credentials' ;;
    BOOTSTRAP_ADMIN_EMAIL) printf '%s' "$BOOTSTRAP_ADMIN_EMAIL" ;;
    BOOTSTRAP_ADMIN_FULL_NAME) printf '%s' "$BOOTSTRAP_ADMIN_FULL_NAME" ;;
    BOOTSTRAP_ADMIN_DEPARTMENT) printf '%s' "$BOOTSTRAP_ADMIN_DEPARTMENT" ;;
    BOOTSTRAP_ADMIN_POSITION) printf '%s' "$BOOTSTRAP_ADMIN_POSITION" ;;
    BOOTSTRAP_ADMIN_PASSWORD_FILE) printf '/opt/consflow/shared/secrets/bootstrap_admin_password' ;;
    STORAGE_S3_BUCKET) printf '%s' "$STORAGE_S3_BUCKET" ;;
    STORAGE_S3_REGION) printf '%s' "$STORAGE_S3_REGION" ;;
    STORAGE_S3_ENDPOINT) printf '%s' "$STORAGE_S3_ENDPOINT" ;;
    STORAGE_S3_USE_PATH_STYLE) printf '%s' "$STORAGE_S3_USE_PATH_STYLE" ;;
    SMTP_HOST|SMTP_USERNAME|SMTP_FROM_ADDRESS) printf '' ;;
    *) return 1 ;;
  esac
}

while IFS= read -r line || [[ -n "$line" ]]; do
  if [[ "$line" == *=* && "$line" != '#'* ]]; then
    key="${line%%=*}"
    if value="$(render_value "$key")"; then
      printf '%s=%s\n' "$key" "$value" >> "$env_output"
      continue
    fi
  fi
  printf '%s\n' "$line" >> "$env_output"
done < "$production_repo_root/.env.prod.example"
chmod 600 "$env_output"

write_secret() {
  local name="$1"
  local value="$2"
  printf '%s\n' "$value" > "$output_root/shared/secrets/$name"
  chmod 600 "$output_root/shared/secrets/$name"
}

write_secret postgres_password "$POSTGRES_PASSWORD"
write_secret database_url "postgres://consflow:${POSTGRES_PASSWORD}@postgres:5432/consflow?sslmode=disable"
write_secret redis_password "$REDIS_PASSWORD"
write_secret redis_url "redis://:${REDIS_PASSWORD}@redis:6379/0"
write_secret redis_cache_password "$REDIS_CACHE_PASSWORD"
write_secret redis_cache_url "redis://:${REDIS_CACHE_PASSWORD}@redis-cache:6379/0"
write_secret jwt_secret "$JWT_SECRET"
write_secret settings_encryption_key "$SETTINGS_ENCRYPTION_KEY"
write_secret bootstrap_admin_password "$BOOTSTRAP_ADMIN_PASSWORD"
printf '[default]\naws_access_key_id=%s\naws_secret_access_key=%s\n' "$S3_ACCESS_KEY" "$S3_SECRET_KEY" > "$output_root/shared/secrets/aws_credentials"
chmod 600 "$output_root/shared/secrets/aws_credentials"

printf 'PASS: rendered production payload at %s without printing secret values\n' "$output_root"
