#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/scripts/production-env-lib.sh
. "$script_dir/production-env-lib.sh"

scope=""
dry_run=false

usage() {
  cat <<EOF
Usage: $(basename "$0") --scope repository|environment [--dry-run]

Uploads only VPS deploy transport secrets. Application/runtime secrets never
leave the operator machine for GitHub and are provisioned directly to the VPS.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --scope)
      [[ $# -ge 2 ]] || production_fail "--scope requires repository or environment"
      scope="$2"
      shift 2
      ;;
    --dry-run)
      dry_run=true
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

[[ "$scope" == repository || "$scope" == environment ]] || production_fail "--scope must be repository or environment"
bash "$script_dir/validate-production-config.sh" --github >/dev/null
production_load_config

secret_args=(--repo "$GITHUB_REPOSITORY")
if [[ "$scope" == environment ]]; then
  secret_args+=(--env "$GITHUB_ENVIRONMENT")
fi

secret_names=(VPS_HOST VPS_PORT VPS_USER VPS_SSH_PRIVATE_KEY VPS_SSH_KNOWN_HOSTS)
if [[ "$dry_run" == true ]]; then
  printf 'DRY RUN: GitHub secret scope=%s repository=%s\n' "$scope" "$GITHUB_REPOSITORY"
  printf 'DRY RUN: would set %s\n' "${secret_names[@]}"
  exit 0
fi

production_require_command gh

if [[ "$scope" == environment ]]; then
  gh api --method PUT "repos/$GITHUB_REPOSITORY/environments/$GITHUB_ENVIRONMENT" >/dev/null
fi

set_value_secret() {
  local name="$1"
  local value="$2"
  printf '%s' "$value" | gh secret set "$name" "${secret_args[@]}"
}

set_file_secret() {
  local name="$1"
  local path="$2"
  gh secret set "$name" "${secret_args[@]}" < "$path"
}

set_value_secret VPS_HOST "$VPS_HOST"
set_value_secret VPS_PORT "$VPS_PORT"
set_value_secret VPS_USER "$VPS_USER"
set_file_secret VPS_SSH_PRIVATE_KEY "$production_ssh_private_key"
set_file_secret VPS_SSH_KNOWN_HOSTS "$production_known_hosts"

printf 'PASS: configured five %s-scoped GitHub deploy secrets without Base64\n' "$scope"
