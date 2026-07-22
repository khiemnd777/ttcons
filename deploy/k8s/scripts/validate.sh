#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
k8s_dir=$(CDPATH= cd -- "$script_dir/.." && pwd)
rendered=$(mktemp "${TMPDIR:-/tmp}/consflow-k8s-render.XXXXXX")
migration_rendered=$(mktemp "${TMPDIR:-/tmp}/consflow-k8s-migrate-render.XXXXXX")
storage_migration_rendered=$(mktemp "${TMPDIR:-/tmp}/consflow-k8s-storage-migrate-render.XXXXXX")
trap 'rm -f "$rendered" "$migration_rendered" "$storage_migration_rendered"' EXIT HUP INT TERM

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required for the bundled Kustomize render check" >&2
  exit 1
fi
if ! command -v ruby >/dev/null 2>&1; then
  echo "ruby is required for deterministic manifest assertions" >&2
  exit 1
fi

kubectl kustomize "$k8s_dir/base" > "$rendered"
kubectl kustomize "$k8s_dir/migrate" > "$migration_rendered"
kubectl kustomize "$k8s_dir/storage-migrate" > "$storage_migration_rendered"
ruby "$script_dir/validate_render.rb" \
  "$rendered" \
  "$migration_rendered" \
  "$storage_migration_rendered" \
  "$k8s_dir/optional/hpa.yaml" \
  "$k8s_dir/optional/ingress.example.yaml"

echo "Kustomize render and deterministic YAML contract checks completed; no cluster request was made."
