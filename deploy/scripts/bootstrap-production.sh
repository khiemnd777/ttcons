#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/scripts/production-env-lib.sh
. "$script_dir/production-env-lib.sh"

legacy_bundle=""
skip_host_scan=false

usage() {
  cat <<EOF
Usage: $(basename "$0") [--import-legacy-bundle PATH] [--skip-host-scan]

Creates or completes the ignored mode-0600 production config and dedicated
SSH deploy key. Existing populated values and keys are never overwritten.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --import-legacy-bundle)
      [[ $# -ge 2 ]] || production_fail "--import-legacy-bundle requires a path"
      legacy_bundle="$2"
      shift 2
      ;;
    --skip-host-scan)
      skip_host_scan=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      production_fail "unknown argument: $1"
      ;;
  esac
done

production_require_command openssl
production_require_command ssh-keygen
umask 077
mkdir -p "$(dirname "$production_config_file")"
if [[ ! -f "$production_config_file" ]]; then
  install -m 600 "$production_config_example" "$production_config_file"
fi
production_require_mode_600 "$production_config_file"

import_secret_file() {
  local key="$1"
  local path="$2"
  local value
  [[ -f "$path" ]] || return 0
  value="$(tr -d '\r\n' < "$path")"
  production_is_placeholder "$value" || production_set_config_value "$key" "$value"
}

if [[ -n "$legacy_bundle" ]]; then
  legacy_bundle="$(cd "$legacy_bundle" && pwd)"
  legacy_secrets="$legacy_bundle/vps/shared/secrets"
  legacy_env="$legacy_bundle/vps/shared/.env.prod"
  [[ -d "$legacy_secrets" && -f "$legacy_env" ]] || production_fail "invalid legacy bundle: $legacy_bundle"

  import_secret_file POSTGRES_PASSWORD "$legacy_secrets/postgres_password"
  import_secret_file REDIS_PASSWORD "$legacy_secrets/redis_password"
  import_secret_file REDIS_CACHE_PASSWORD "$legacy_secrets/redis_cache_password"
  import_secret_file JWT_SECRET "$legacy_secrets/jwt_secret"
  import_secret_file SETTINGS_ENCRYPTION_KEY "$legacy_secrets/settings_encryption_key"
  import_secret_file BOOTSTRAP_ADMIN_PASSWORD "$legacy_secrets/bootstrap_admin_password"

  legacy_admin_email="$(production_env_file_value "$legacy_env" BOOTSTRAP_ADMIN_EMAIL || true)"
  legacy_admin_name="$(production_env_file_value "$legacy_env" BOOTSTRAP_ADMIN_FULL_NAME || true)"
  legacy_region="$(production_env_file_value "$legacy_env" STORAGE_S3_REGION || true)"
  [[ -z "$legacy_admin_email" ]] || production_set_config_value BOOTSTRAP_ADMIN_EMAIL "$legacy_admin_email"
  [[ -z "$legacy_admin_name" ]] || production_set_config_value BOOTSTRAP_ADMIN_FULL_NAME "$legacy_admin_name"
  [[ -z "$legacy_region" ]] || production_set_config_value STORAGE_S3_REGION "$legacy_region"

  legacy_aws="$legacy_secrets/aws_credentials"
  if [[ -f "$legacy_aws" ]]; then
    legacy_access="$(production_env_file_value "$legacy_aws" aws_access_key_id || true)"
    legacy_secret="$(production_env_file_value "$legacy_aws" aws_secret_access_key || true)"
    production_is_placeholder "$legacy_access" || production_set_config_value S3_ACCESS_KEY "$legacy_access"
    production_is_placeholder "$legacy_secret" || production_set_config_value S3_SECRET_KEY "$legacy_secret"
  fi

  if [[ ! -f "$production_ssh_private_key" && -f "$legacy_bundle/github-environment/VPS_SSH_PRIVATE_KEY" ]]; then
    install -m 600 "$legacy_bundle/github-environment/VPS_SSH_PRIVATE_KEY" "$production_ssh_private_key"
  fi
  if [[ ! -f "$production_ssh_public_key" && -f "$legacy_bundle/github-environment/VPS_SSH_PUBLIC_KEY.pub" ]]; then
    install -m 600 "$legacy_bundle/github-environment/VPS_SSH_PUBLIC_KEY.pub" "$production_ssh_public_key"
  fi
  if [[ ! -f "$production_known_hosts" && -f "$legacy_bundle/github-environment/VPS_SSH_KNOWN_HOSTS" ]]; then
    awk '$2 == "ssh-ed25519" { print; exit }' "$legacy_bundle/github-environment/VPS_SSH_KNOWN_HOSTS" > "$production_known_hosts"
    chmod 600 "$production_known_hosts"
  fi
fi

generate_if_needed() {
  local key="$1"
  local bytes="$2"
  local current
  current="$(production_config_value "$key" || true)"
  if production_is_placeholder "$current"; then
    production_set_config_value "$key" "$(openssl rand -hex "$bytes")"
  fi
}

generate_if_needed POSTGRES_PASSWORD 24
generate_if_needed REDIS_PASSWORD 24
generate_if_needed REDIS_CACHE_PASSWORD 24
generate_if_needed JWT_SECRET 48
generate_if_needed SETTINGS_ENCRYPTION_KEY 48
generate_if_needed BOOTSTRAP_ADMIN_PASSWORD 24

production_load_config
if [[ ! -f "$production_ssh_private_key" ]]; then
  ssh-keygen -q -t ed25519 -N '' -C consflow-production-deploy -f "$production_ssh_private_key"
fi
if [[ ! -f "$production_ssh_public_key" ]]; then
  ssh-keygen -y -f "$production_ssh_private_key" > "$production_ssh_public_key"
fi
chmod 600 "$production_ssh_private_key" "$production_ssh_public_key"

if [[ ! -s "$production_known_hosts" && "$skip_host_scan" != true ]]; then
  candidate="$(mktemp "${production_known_hosts}.tmp.XXXXXX")"
  if ssh-keyscan -T 5 -p "$VPS_PORT" -t ed25519 "$VPS_HOST" > "$candidate" 2>/dev/null; then
    install -m 600 "$candidate" "$production_known_hosts"
  else
    printf 'warning: could not capture the VPS ED25519 host key; retry after SSH is reachable\n' >&2
  fi
  rm -f "$candidate"
fi

printf 'Production operator config: %s\n' "$production_config_file"
printf 'SSH public key: %s\n' "$production_ssh_public_key"
if [[ -s "$production_known_hosts" ]]; then
  printf 'Candidate VPS host-key fingerprint (verify through the VPS console):\n'
  ssh-keygen -lf "$production_known_hosts"
fi

pending=false
production_load_config
for key in VPS_HOST_KEY_FINGERPRINT GHCR_READ_TOKEN S3_ACCESS_KEY S3_SECRET_KEY; do
  if production_is_placeholder "${!key:-}"; then
    printf 'PENDING: %s\n' "$key"
    pending=true
  fi
done
if [[ "$pending" == true ]]; then
  printf 'Bootstrap complete with external inputs still pending. Run make production-check after filling them.\n'
else
  printf 'Bootstrap complete. Run make production-check.\n'
fi
