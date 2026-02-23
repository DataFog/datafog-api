#!/bin/bash
set -euo pipefail

# ── Constants ────────────────────────────────────────────────────────────────
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_CONFIG_PATH="scripts/ci/he-docs-config.json"

# ── Globals ──────────────────────────────────────────────────────────────────
ERRORS=0
WARNINGS=0

# ── Helpers ──────────────────────────────────────────────────────────────────

_env_flag() {
  local name="$1"
  local default="${2:-0}"
  local val="${!name:-$default}"
  [[ "$val" == "1" ]]
}

_load_config() {
  local config_path="${HARNESS_DOCS_CONFIG:-$DEFAULT_CONFIG_PATH}"
  local path="$REPO_ROOT/$config_path"
  if [[ ! -f "$path" ]]; then
    echo "Error: he-docs-lint missing/invalid config: Missing config '$config_path'. Fix: create it (bootstrap should do this) or set HARNESS_DOCS_CONFIG." >&2
    return 1
  fi
  # Validate it is a JSON object
  if ! jq -e 'type == "object"' "$path" >/dev/null 2>&1; then
    echo "Error: he-docs-lint missing/invalid config: Config must be a JSON object." >&2
    return 1
  fi
  cat "$path"
}

_gh_annotate() {
  local level="$1" file="$2" title="$3" msg="$4"
  if [[ -n "$file" ]]; then
    echo "::${level} file=${file},title=${title}::${msg}"
  else
    echo "::${level} title=${title}::${msg}"
  fi
}

_emit() {
  local level="$1" file="$2" title="$3" msg="$4"
  _gh_annotate "$level" "$file" "$title" "$msg"
  local upper
  upper="$(echo "$level" | tr '[:lower:]' '[:upper:]')"
  echo "${upper}: ${msg}" >&2
  if [[ "$level" == "error" ]]; then
    ERRORS=$((ERRORS + 1))
  else
    WARNINGS=$((WARNINGS + 1))
  fi
}

_has_exact_line() {
  local path="$1" needle="$2"
  grep -Fxq "$needle" "$path" 2>/dev/null
}

# ── Checks ───────────────────────────────────────────────────────────────────

_check_required_docs() {
  local cfg="$1"
  local count
  count="$(echo "$cfg" | jq -r '.required_docs | if type == "array" then length else 0 end')"
  if [[ "$count" -eq 0 ]]; then
    return
  fi
  local i doc
  for ((i = 0; i < count; i++)); do
    doc="$(echo "$cfg" | jq -r ".required_docs[$i]")"
    if [[ "$doc" == "null" ]] || [[ -z "$doc" ]]; then
      continue
    fi
    if [[ ! -e "$REPO_ROOT/$doc" ]]; then
      _emit "error" "$doc" "Required doc missing" \
        "Missing required doc: '$doc'. Fix: create it (run he-bootstrap if this repo is not bootstrapped) or adjust required_docs in config."
    fi
  done
}

_check_domain_doc_headings() {
  local cfg="$1"
  local is_obj
  is_obj="$(echo "$cfg" | jq -r '.required_headings | type')"
  if [[ "$is_obj" != "object" ]]; then
    return
  fi

  local docs
  docs="$(echo "$cfg" | jq -r '.required_headings | keys[]')"
  if [[ -z "$docs" ]]; then
    return
  fi

  local doc
  while IFS= read -r doc; do
    [[ -z "$doc" ]] && continue
    local path="$REPO_ROOT/$doc"
    if [[ ! -f "$path" ]]; then
      continue  # on-demand domain docs
    fi

    local headings_count
    headings_count="$(echo "$cfg" | jq -r --arg d "$doc" '.required_headings[$d] | if type == "array" then length else 0 end')"
    if [[ "$headings_count" -eq 0 ]]; then
      _emit "error" "$doc" "Missing config headings" \
        "No required headings configured for '$doc'. Fix: add required_headings['$doc'] in config or remove the entry."
      continue
    fi

    local missing=()
    local j heading
    for ((j = 0; j < headings_count; j++)); do
      heading="$(echo "$cfg" | jq -r --arg d "$doc" ".required_headings[\$d][$j]")"
      if [[ "$heading" == "null" ]] || [[ -z "$heading" ]]; then
        continue
      fi
      if ! _has_exact_line "$path" "$heading"; then
        missing+=("$heading")
      fi
    done

    if [[ ${#missing[@]} -gt 0 ]]; then
      local joined
      joined="$(printf "%s; " "${missing[@]}")"
      joined="${joined%; }"  # trim trailing "; "
      _emit "error" "$doc" "Missing headings" \
        "Missing required headings in '$doc': ${joined}. Fix: add them."
    fi
  done <<< "$docs"
}

_check_seed_markers() {
  local cfg="$1"
  local fail_level="warning"
  if _env_flag "HARNESS_FAIL_ON_SEED_MARKERS" "0"; then
    fail_level="error"
  fi

  local count
  count="$(echo "$cfg" | jq -r '.domain_docs | if type == "array" then length else 0 end')"
  if [[ "$count" -eq 0 ]]; then
    return
  fi

  local i doc path
  for ((i = 0; i < count; i++)); do
    doc="$(echo "$cfg" | jq -r ".domain_docs[$i]")"
    if [[ "$doc" == "null" ]] || [[ -z "$doc" ]]; then
      continue
    fi
    path="$REPO_ROOT/$doc"
    if [[ ! -f "$path" ]]; then
      continue
    fi
    if grep -q '<!-- seed:' "$path" 2>/dev/null; then
      _emit "$fail_level" "$doc" "Seed markers present" \
        "Template seed markers remain in '$doc'. Fix: replace/remove <!-- seed: ... --> blocks once this repo has real domain context."
    fi
  done
}

_check_generated_last_updated() {
  local gen_dir="$REPO_ROOT/docs/generated"
  if [[ ! -d "$gen_dir" ]]; then
    return
  fi

  local fail_level="warning"
  if _env_flag "HARNESS_FAIL_ON_GENERATED_PLACEHOLDERS" "0"; then
    fail_level="error"
  fi

  local path rel
  for path in "$gen_dir"/*.md; do
    [[ -e "$path" ]] || continue  # handle no-match glob
    rel="${path#"$REPO_ROOT/"}"
    # Skip known non-generated docs
    if [[ "$rel" == "docs/generated/README.md" ]] || [[ "$rel" == "docs/generated/memory.md" ]]; then
      continue
    fi
    local text
    text="$(cat "$path")"

    # Check for missing last_updated line (BSD/GNU portable; avoid grep -P)
    if ! echo "$text" | grep -Eq '^[[:space:]]*-[[:space:]]*last_updated:[[:space:]]*'; then
      _emit "error" "$rel" "Missing last_updated" \
        "Generated doc '$rel' must include a 'last_updated' line. Fix: add e.g. '- last_updated: 2026-02-15 12:34'."
    fi

    # Check for placeholder last_updated value
    if echo "$text" | grep -Eq 'last_updated:[[:space:]]*<YYYY-'; then
      _emit "$fail_level" "$rel" "Placeholder last_updated" \
        "Generated doc '$rel' has a placeholder last_updated value. Fix: replace with a real timestamp."
    fi
  done
}

# ── Main ─────────────────────────────────────────────────────────────────────

main() {
  local cfg
  if ! cfg="$(_load_config)"; then
    exit 2
  fi

  echo "he-docs-lint: starting"
  echo "Repro: bash scripts/ci/he-docs-lint.sh"

  _check_required_docs "$cfg"

  # Runbooks lint is its own script. Fail fast if it fails.
  if ! bash "$REPO_ROOT/scripts/ci/he-runbooks-lint.sh"; then
    exit 1
  fi

  _check_domain_doc_headings "$cfg"
  _check_seed_markers "$cfg"
  _check_generated_last_updated

  if [[ "$ERRORS" -gt 0 ]]; then
    echo "he-docs-lint: FAIL ($ERRORS error(s), $WARNINGS warning(s))" >&2
    exit 1
  fi

  echo "he-docs-lint: OK ($WARNINGS warning(s))"
  exit 0
}

main "$@"
