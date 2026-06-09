#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FAILED=0

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  FAILED=1
}

pass() {
  printf 'OK: %s\n' "$1"
}

[ -s "$ROOT/AGENTS.md" ] || fail "AGENTS.md exists and is non-empty"
[ -d "$ROOT/.agents/skills" ] || fail ".agents/skills exists"
[ -d "$ROOT/.codex/agents" ] || fail ".codex/agents exists"

shopt -s nullglob

for dir in "$ROOT"/.agents/skills/*; do
  [ -d "$dir" ] || continue
  name="$(basename "$dir")"
  skill="$dir/SKILL.md"
  meta="$dir/agents/openai.yaml"

  [ -s "$skill" ] || { fail "$name has SKILL.md"; continue; }
  declared="$(awk -F': *' '$1 == "name" { print $2; exit }' "$skill" | tr -d '"')"
  [ "$declared" = "$name" ] || fail "$skill name matches folder ($declared != $name)"
  grep -Eq '^description: .+' "$skill" || fail "$skill has description frontmatter"
  [ -s "$meta" ] || fail "$name has agents/openai.yaml metadata"
done

for file in "$ROOT"/.codex/agents/*.toml; do
  [ -f "$file" ] || continue
  name="$(basename "$file" .toml)"
  grep -Eq "^name = \"$name\"$" "$file" || fail "$file name matches filename"
  grep -Eq '^description = ".+"' "$file" || fail "$file has description"
  grep -q '^developer_instructions = """' "$file" || fail "$file has developer_instructions"
done

if [ "$FAILED" -ne 0 ]; then
  exit 1
fi

pass "agent artifacts validated"
