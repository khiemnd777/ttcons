#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
validator="$repo_root/scripts/validate-release-tag.sh"

valid_tags=(
  v2026-08-20.1
  v2024-02-29.12
  v2030-12-31.999
)
invalid_tags=(
  v2026-08-20.0
  v2026-08-20
  v2026-02-29.1
  v2026-13-01.1
  v2026-08-20.01
  2026-08-20.1
  v26-08-20.1
  v2026-8-20.1
  v2026-08-20.1-extra
)

for tag in "${valid_tags[@]}"; do
  bash "$validator" "$tag" >/dev/null
done

for tag in "${invalid_tags[@]}"; do
  if bash "$validator" "$tag" >/dev/null 2>&1; then
    echo "validator accepted invalid tag: $tag" >&2
    exit 1
  fi
done

echo "release tag validation passed"
