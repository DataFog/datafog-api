#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_CONFIG_PATH="scripts/ci/he-docs-config.json"

config_path="${HARNESS_DOCS_CONFIG:-$DEFAULT_CONFIG_PATH}"
config_file="${REPO_ROOT}/${config_path}"

if [[ ! -f "$config_file" ]]; then
  echo "Error: he-docs-drift missing/invalid config: Missing config '${config_path}'. Fix: create it (bootstrap should do this) or set HARNESS_DOCS_CONFIG." >&2
  exit 2
fi

cfg="$(cat "$config_file")"
if ! echo "$cfg" | jq -e 'type == "object"' >/dev/null 2>&1; then
  echo "Error: he-docs-drift missing/invalid config: Config must be a JSON object." >&2
  exit 2
fi

base_ref="${GITHUB_BASE_REF:-}"
head_ref="${GITHUB_HEAD_REF:-}"

if [[ -n "$base_ref" ]]; then
  diff_range="origin/${base_ref}...HEAD"
else
  if git -C "$REPO_ROOT" rev-parse -q --verify HEAD~1 >/dev/null 2>&1; then
    diff_range="HEAD~1...HEAD"
  else
    diff_range=""
  fi
fi

echo "he-docs-drift: starting" >&2
echo "Repro: bash scripts/ci/he-docs-drift.sh" >&2
if [[ -n "$base_ref" ]]; then
  echo "PR context: base_ref='${base_ref}' head_ref='${head_ref}' diff='${diff_range}'" >&2
else
  echo "Local context: diff='${diff_range}'" >&2
fi

if [[ -n "$diff_range" ]]; then
  changed="$(git -C "$REPO_ROOT" diff --name-only "$diff_range" 2>/dev/null || true)"
else
  changed="$(git -C "$REPO_ROOT" diff-tree --no-commit-id --name-only -r HEAD 2>/dev/null || true)"
fi

# Trim empty lines
changed="$(echo "$changed" | sed '/^[[:space:]]*$/d')"

if [[ -z "$changed" ]]; then
  echo "he-docs-drift: no changes detected"
  exit 0
fi

# Build list of changed docs (files starting with docs/)
changed_docs="$(echo "$changed" | grep '^docs/' || true)"

# Extract drift_rules array; default to empty array if missing or wrong type
drift_rules="$(echo "$cfg" | jq -c '.drift_rules // [] | if type == "array" then . else [] end')"
rule_count="$(echo "$drift_rules" | jq 'length')"

missing=0

for ((i = 0; i < rule_count; i++)); do
  rule="$(echo "$drift_rules" | jq -c ".[$i]")"

  # Skip non-object entries
  if ! echo "$rule" | jq -e 'type == "object"' >/dev/null 2>&1; then
    continue
  fi

  regex="$(echo "$rule" | jq -r '.regex // empty')"
  doc="$(echo "$rule" | jq -r '.doc // empty')"

  if [[ -z "$regex" || -z "$doc" ]]; then
    continue
  fi

  # Validate regex by testing it
  if ! echo "" | grep -qE "$regex" 2>/dev/null && [[ $? -eq 2 ]]; then
    echo "Error: invalid drift rule regex: ${regex}" >&2
    missing=1
    continue
  fi

  # Find changed files matching the regex
  matching="$(echo "$changed" | grep -E "$regex" || true)"

  if [[ -z "$matching" ]]; then
    continue
  fi

  # Check if the required doc is in the changed docs list
  if ! echo "$changed_docs" | grep -qxF "$doc" 2>/dev/null; then
    sample="$(echo "$matching" | head -n 10 | sed 's/^/- /')"
    echo "::error file=${doc},title=Docs drift gate::Missing required doc update '${doc}' when files match /${regex}/ (see job logs for matching files)."
    echo "Missing doc update: '${doc}' should change when files match /${regex}/." >&2
    echo "Matching files (up to 10):" >&2
    echo "$sample" >&2
    echo "Fix: update '${doc}' in this PR, or edit drift_rules in '${DEFAULT_CONFIG_PATH}' (or HARNESS_DOCS_CONFIG) if this mapping is wrong." >&2
    missing=1
  fi
done

if [[ "$missing" -ne 0 ]]; then
  echo "Error: docs drift gate failed (see missing doc updates above)" >&2
  exit 1
fi

echo "he-docs-drift: OK"
exit 0
