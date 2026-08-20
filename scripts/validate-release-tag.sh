#!/usr/bin/env bash
set -euo pipefail

tag="${1:-}"
if [[ ! "$tag" =~ ^v([0-9]{4}-[0-9]{2}-[0-9]{2})\.([1-9][0-9]*)$ ]]; then
  echo "invalid release tag: expected vYYYY-MM-DD.N with N >= 1" >&2
  exit 1
fi

date_part="${BASH_REMATCH[1]}"
parsed_date=""
if parsed_date="$(date -u -d "$date_part" '+%Y-%m-%d' 2>/dev/null)"; then
  :
elif parsed_date="$(date -j -u -f '%Y-%m-%d' "$date_part" '+%Y-%m-%d' 2>/dev/null)"; then
  :
else
  echo "invalid release tag: date is not a real calendar date" >&2
  exit 1
fi

if [[ "$parsed_date" != "$date_part" ]]; then
  echo "invalid release tag: date normalized to $parsed_date" >&2
  exit 1
fi

printf '%s\n' "$tag"
