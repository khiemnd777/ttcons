#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../.." && pwd)"
requested_tag="${1:-}"

if [[ $# -gt 1 || "$requested_tag" == -h || "$requested_tag" == --help ]]; then
  cat <<EOF
Usage: $(basename "$0") [vYYYY-MM-DD.N]

With no argument, selects the next tag number for today's local calendar date.
The command requires a clean main branch whose HEAD exactly matches origin/main.
EOF
  [[ $# -le 1 && ( "$requested_tag" == -h || "$requested_tag" == --help ) ]] && exit 0
  exit 1
fi

cd "$repo_root"
command -v git >/dev/null 2>&1 || { echo "git is required" >&2; exit 1; }
[[ "$(git branch --show-current)" == main ]] || { echo "release tags may only be created from branch main" >&2; exit 1; }
[[ -z "$(git status --porcelain)" ]] || { echo "worktree must be clean before creating a release tag" >&2; git status --short; exit 1; }

git fetch --no-tags origin main
git fetch origin 'refs/tags/v*:refs/tags/v*'
[[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || { echo "local main HEAD must exactly match origin/main" >&2; exit 1; }

tag="$requested_tag"
if [[ -z "$tag" ]]; then
  tag_date="$(date +%Y-%m-%d)"
  tag_index=1
  while git rev-parse -q --verify "refs/tags/v${tag_date}.${tag_index}" >/dev/null; do
    tag_index=$((tag_index + 1))
  done
  tag="v${tag_date}.${tag_index}"
fi

bash "$repo_root/scripts/validate-release-tag.sh" "$tag" >/dev/null
git rev-parse -q --verify "refs/tags/$tag" >/dev/null 2>&1 && { echo "tag already exists: $tag" >&2; exit 1; }
git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1 && { echo "remote tag already exists: $tag" >&2; exit 1; }

git tag -a "$tag" -m "Release $tag" HEAD
git push origin "$tag"
printf 'Release tag pushed: %s\n' "$tag"
printf 'Only the tag workflow may build and deploy this release; push to main remains non-triggering.\n'
