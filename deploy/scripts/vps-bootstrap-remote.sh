#!/usr/bin/env bash
set -Eeuo pipefail

stage_dir="${1:-}"
[[ -d "$stage_dir/shared/secrets" ]] || { echo "invalid production bootstrap payload" >&2; exit 1; }
[[ "${EUID:-$(id -u)}" -eq 0 ]] || { echo "VPS bootstrap must run as root or passwordless sudo" >&2; exit 1; }

deploy_user="$(tr -d '\r\n' < "$stage_dir/deploy_user")"
[[ "$deploy_user" =~ ^[a-z_][a-z0-9_-]*[$]?$ || "$deploy_user" == root ]] || { echo "invalid deploy user" >&2; exit 1; }
id "$deploy_user" >/dev/null 2>&1 || { echo "deploy user does not exist: $deploy_user" >&2; exit 1; }

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
  if ! command -v apt-get >/dev/null 2>&1; then
    echo "Docker/Compose is missing and automatic installation supports apt-based Linux only" >&2
    exit 1
  fi
  apt-get update
  if apt-cache show docker-compose-v2 >/dev/null 2>&1; then
    compose_package=docker-compose-v2
  elif apt-cache show docker-compose-plugin >/dev/null 2>&1; then
    compose_package=docker-compose-plugin
  else
    echo "no Docker Compose v2 package is available from the configured apt repositories" >&2
    exit 1
  fi
  runtime_packages=("$compose_package" ca-certificates curl)
  if ! command -v docker >/dev/null 2>&1; then
    runtime_packages=(docker.io "${runtime_packages[@]}")
  fi
  apt-get install -y "${runtime_packages[@]}"
fi
systemctl enable --now docker

deploy_group="$(id -gn "$deploy_user")"
install -d -m 0750 -o "$deploy_user" -g "$deploy_group" /opt/consflow
install -d -m 0750 -o "$deploy_user" -g "$deploy_group" /opt/consflow/releases
install -d -m 0700 -o "$deploy_user" -g "$deploy_group" /opt/consflow/shared
install -d -m 0700 -o "$deploy_user" -g "$deploy_group" /opt/consflow/shared/secrets
install -m 0600 -o "$deploy_user" -g "$deploy_group" "$stage_dir/shared/.env.prod" /opt/consflow/shared/.env.prod

for secret_file in "$stage_dir"/shared/secrets/*; do
  install -m 0644 -o "$deploy_user" -g "$deploy_group" "$secret_file" "/opt/consflow/shared/secrets/$(basename "$secret_file")"
done

if [[ "$deploy_user" != root ]]; then
  usermod -aG docker "$deploy_user"
fi

test -r /opt/consflow/shared/.env.prod
test -r /opt/consflow/shared/secrets/database_url
test -r /opt/consflow/shared/secrets/aws_credentials
docker compose version >/dev/null
echo "PASS: VPS runtime and /opt/consflow production inputs provisioned"
