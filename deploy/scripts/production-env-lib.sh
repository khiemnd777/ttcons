#!/usr/bin/env bash

production_script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
production_repo_root="$(cd "$production_script_dir/../.." && pwd)"
production_config_file="${PRODUCTION_CONFIG_FILE:-$production_repo_root/deploy/config/production.env}"
production_config_example="$production_repo_root/deploy/config/production.env.example"
production_ssh_private_key="${PRODUCTION_SSH_PRIVATE_KEY:-$production_repo_root/deploy/config/production_id_ed25519}"
production_ssh_public_key="${PRODUCTION_SSH_PUBLIC_KEY:-$production_repo_root/deploy/config/production_id_ed25519.pub}"
production_known_hosts="${PRODUCTION_KNOWN_HOSTS:-$production_repo_root/deploy/config/production_known_hosts}"

production_fail() {
  printf 'production config error: %s\n' "$*" >&2
  exit 1
}

production_load_env_file() {
  local env_file="$1"
  local line key

  [[ -f "$env_file" ]] || production_fail "missing file: $env_file"
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
    case "$line" in
      ''|'#'*) continue ;;
    esac
    [[ "$line" == *=* ]] || production_fail "invalid line in $env_file"
    key="${line%%=*}"
    [[ "$key" =~ ^[A-Z][A-Z0-9_]*$ ]] || production_fail "invalid key in $env_file: $key"
    export "$key=${line#*=}"
  done < "$env_file"
}

production_load_config() {
  production_load_env_file "$production_config_file"
}

production_config_value() {
  local key="$1"
  local line
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" == "$key="* ]]; then
      printf '%s' "${line#*=}"
      return 0
    fi
  done < "$production_config_file"
  return 1
}

production_env_file_value() {
  local env_file="$1"
  local key="$2"
  local line
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" == "$key="* ]]; then
      printf '%s' "${line#*=}"
      return 0
    fi
  done < "$env_file"
  return 1
}

production_set_config_value() {
  local key="$1"
  local value="$2"
  local tmp line found=false

  [[ "$value" != *$'\n'* && "$value" != *$'\r'* ]] || production_fail "$key must be a single line"
  tmp="$(mktemp "${production_config_file}.tmp.XXXXXX")"
  chmod 600 "$tmp"
  while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" == "$key="* ]]; then
      printf '%s=%s\n' "$key" "$value" >> "$tmp"
      found=true
    else
      printf '%s\n' "$line" >> "$tmp"
    fi
  done < "$production_config_file"
  if [[ "$found" != true ]]; then
    printf '%s=%s\n' "$key" "$value" >> "$tmp"
  fi
  mv "$tmp" "$production_config_file"
  chmod 600 "$production_config_file"
}

production_is_placeholder() {
  local value="${1:-}"
  [[ -z "$value" || "$value" == GENERATE || "$value" == REPLACE_WITH_* || "$value" == *'<secret>'* || "$value" == *CHANGE_ME* ]]
}

production_require_value() {
  local key="$1"
  local value="${!key:-}"
  if production_is_placeholder "$value"; then
    production_fail "$key is missing or still a placeholder"
  fi
  return 0
}

production_file_mode() {
  local path="$1"
  if stat -f '%Lp' "$path" >/dev/null 2>&1; then
    stat -f '%Lp' "$path"
  else
    stat -c '%a' "$path"
  fi
}

production_require_mode_600() {
  local path="$1"
  [[ -f "$path" ]] || production_fail "missing file: $path"
  [[ "$(production_file_mode "$path")" == 600 ]] || production_fail "$path must have mode 0600"
}

production_ssh_host_token() {
  if [[ "${VPS_PORT:-22}" == 22 ]]; then
    printf '%s' "$VPS_HOST"
  else
    printf '[%s]:%s' "$VPS_HOST" "$VPS_PORT"
  fi
}

production_require_command() {
  command -v "$1" >/dev/null 2>&1 || production_fail "required command not found: $1"
}
