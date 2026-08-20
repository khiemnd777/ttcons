#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/scripts/production-env-lib.sh
. "$script_dir/production-env-lib.sh"

usage() {
  cat <<EOF
Usage: $(basename "$0") [--dry-run]

Prerequisite: install the generated public key in the VPS user's authorized_keys
through a trusted console/password session. This command never accepts or stores
the VPS password.
EOF
}

dry_run=false
while [[ $# -gt 0 ]]; do
  case "$1" in
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
      production_fail "unexpected argument: $1"
      ;;
  esac
done

bash "$script_dir/validate-production-config.sh" --provision >/dev/null
production_load_config
production_require_command ssh
production_require_command scp
production_require_command tar

umask 077
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/consflow-production-provision.XXXXXX")"
archive="$work_dir/production-bootstrap.tgz"
remote_id="$$-${RANDOM:-0}"
remote_archive="/tmp/consflow-production-bootstrap-$remote_id.tgz"
remote_stage="/tmp/consflow-production-bootstrap-$remote_id"
remote_uploaded=false
remote_execution_started=false

cleanup() {
  local exit_status=$?
  rm -rf "$work_dir"
  if [[ "$remote_uploaded" == true && "$remote_execution_started" != true ]]; then
    ssh "${ssh_options[@]}" "$target" "rm -f '$remote_archive'" >/dev/null 2>&1 || true
  fi
  return "$exit_status"
}
trap cleanup EXIT

bash "$script_dir/render-production-config.sh" "$work_dir/payload" >/dev/null
install -m 700 "$script_dir/vps-bootstrap-remote.sh" "$work_dir/payload/vps-bootstrap-remote.sh"
printf '%s\n' "$VPS_USER" > "$work_dir/payload/deploy_user"
printf '%s\n' "$GHCR_USERNAME" > "$work_dir/payload/ghcr_username"
printf '%s\n' "$GHCR_READ_TOKEN" > "$work_dir/payload/ghcr_token"
chmod 600 "$work_dir/payload/deploy_user" "$work_dir/payload/ghcr_username" "$work_dir/payload/ghcr_token"
tar -C "$work_dir/payload" -czf "$archive" .
chmod 600 "$archive"

ssh_options=(
  -i "$production_ssh_private_key"
  -o IdentitiesOnly=yes
  -o BatchMode=yes
  -o StrictHostKeyChecking=yes
  -o "UserKnownHostsFile=$production_known_hosts"
  -o ConnectTimeout=15
  -p "$VPS_PORT"
)
scp_options=(
  -i "$production_ssh_private_key"
  -o IdentitiesOnly=yes
  -o BatchMode=yes
  -o StrictHostKeyChecking=yes
  -o "UserKnownHostsFile=$production_known_hosts"
  -o ConnectTimeout=15
  -P "$VPS_PORT"
)
target="$VPS_USER@$VPS_HOST"

if [[ "$dry_run" == true ]]; then
  printf 'DRY RUN: validated and rendered the encrypted-transport VPS payload for %s:%s\n' "$target" "$VPS_PORT"
  printf 'DRY RUN: would install Docker/Compose, GHCR login and /opt/consflow/shared through pinned key-only SSH\n'
  exit 0
fi

ssh "${ssh_options[@]}" "$target" "test \"\$(id -u)\" -eq 0 || sudo -n true"
scp "${scp_options[@]}" "$archive" "$target:$remote_archive"
remote_uploaded=true

printf -v remote_command '%s\n' \
  'set -Eeuo pipefail' \
  "stage='$remote_stage'" \
  "archive='$remote_archive'" \
  'cleanup() {' \
  '  if [ "$(id -u)" -eq 0 ]; then' \
  '    rm -rf "$stage" "$archive"' \
  '  else' \
  '    sudo -n rm -rf "$stage" "$archive"' \
  '  fi' \
  '}' \
  'trap cleanup EXIT' \
  'if [ "$(id -u)" -eq 0 ]; then' \
  '  install -d -m 0700 "$stage"' \
  '  tar -xzf "$archive" -C "$stage"' \
  '  bash "$stage/vps-bootstrap-remote.sh" "$stage"' \
  'else' \
  '  sudo -n install -d -m 0700 "$stage"' \
  '  sudo -n tar -xzf "$archive" -C "$stage"' \
  '  sudo -n bash "$stage/vps-bootstrap-remote.sh" "$stage"' \
  'fi'
remote_execution_started=true
ssh "${ssh_options[@]}" "$target" "$remote_command"

printf 'PASS: provisioned %s without transmitting a VPS password through GitHub or this script\n' "$target"
printf 'NEXT: verify provider firewall TCP 80/443 and keep DNS pointed directly to %s before release.\n' "$VPS_HOST"
