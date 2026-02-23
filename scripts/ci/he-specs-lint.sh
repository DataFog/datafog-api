#!/bin/bash
set -euo pipefail

# ── Repo root relative to script location (scripts/ci/he-specs-lint.sh) ──
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEFAULT_CONFIG_PATH="scripts/ci/he-docs-config.json"

# ── Default required headings ──
DEFAULT_REQUIRED_HEADINGS=(
  "## Purpose / Big Picture"
  "## Scope"
  "## Non-Goals"
  "## Risks"
  "## Rollout"
  "## Validation and Acceptance Signals"
  "## Requirements"
  "## Success Criteria"
  "## Priority"
  "## Initial Milestone Candidates"
  "## Revision Notes"
)

DEFAULT_TRIVIAL_REQUIRED_HEADINGS=(
  "## Purpose / Big Picture"
  "## Requirements"
  "## Success Criteria"
)

# ── Counters ──
errors=0
warnings=0

# ── Helpers ──

gh_annotate() {
  local level="$1" file="$2" title="$3" msg="$4"
  if [[ -n "$file" ]]; then
    echo "::${level} file=${file},title=${title}::${msg}"
  else
    echo "::${level} title=${title}::${msg}"
  fi
}

emit() {
  local level="$1" file="$2" title="$3" msg="$4"
  gh_annotate "$level" "$file" "$title" "$msg"
  local upper
  upper="$(echo "$level" | tr '[:lower:]' '[:upper:]')"
  echo "${upper}: ${msg}" >&2
  if [[ "$level" == "error" ]]; then
    (( errors++ )) || true
  else
    (( warnings++ )) || true
  fi
}

# Extract frontmatter block (content between first --- and second ---).
# Returns via stdout; returns 1 if no valid frontmatter found.
extract_frontmatter() {
  local file="$1"
  local first_line
  first_line="$(head -n1 "$file")"
  # Trim whitespace
  first_line="$(echo "$first_line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [[ "$first_line" != "---" ]]; then
    return 1
  fi
  # Print lines between first --- and second ---, exclusive
  awk 'NR==1 && /^[[:space:]]*---[[:space:]]*$/ { found=1; next }
       found && /^[[:space:]]*---[[:space:]]*$/ { exit }
       found { print }' "$file"
  # Verify we actually found a closing ---
  local count
  count="$(awk '/^[[:space:]]*---[[:space:]]*$/ { c++ } c==2 { print c; exit }' "$file")"
  if [[ "$count" != "2" ]]; then
    return 1
  fi
  return 0
}

# Parse frontmatter key-value pairs into an associative array.
# Usage: parse_frontmatter "$frontmatter_text"
# Sets global associative array FM_KV.
parse_frontmatter() {
  local fm_text="$1"
  FM_KV=()
  while IFS= read -r raw_line; do
    # Trim
    local line
    line="$(echo "$raw_line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    # Skip empty lines and comments
    [[ -z "$line" || "$line" == \#* ]] && continue
    # Must contain a colon
    [[ "$line" != *:* ]] && continue
    local key val
    key="$(echo "$line" | cut -d: -f1 | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    val="$(echo "$line" | cut -d: -f2- | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    FM_KV["$key"]="$val"
  done <<< "$fm_text"
}

# Check if file contains an exact line match.
has_exact_line() {
  local file="$1" needle="$2"
  grep -qFx "$needle" "$file"
}

# Check for placeholder tokens in file text.
check_placeholders() {
  local file_rel="$1" file_path="$2" fail_ph="$3"
  shift 3
  local patterns=("$@")
  for p in "${patterns[@]}"; do
    [[ -z "$p" ]] && continue
    if grep -qF "$p" "$file_path"; then
      local msg="Spec '${file_rel}' contains placeholder token '${p}'."
      if [[ "$fail_ph" == "1" ]]; then
        emit "error" "$file_rel" "Placeholder token" "$msg"
      else
        emit "warning" "$file_rel" "Placeholder token" "${msg} (Set HARNESS_FAIL_ON_ARTIFACT_PLACEHOLDERS=1 to enforce.)"
      fi
      break
    fi
  done
}

# ── Load config ──
load_config() {
  local config_rel="${HARNESS_DOCS_CONFIG:-$DEFAULT_CONFIG_PATH}"
  local config_path="${REPO_ROOT}/${config_rel}"
  if [[ ! -f "$config_path" ]]; then
    echo "Error: he-specs-lint missing/invalid config: Missing config '${config_rel}'. Fix: create it (bootstrap should do this) or set HARNESS_DOCS_CONFIG." >&2
    exit 2
  fi
  # Validate it's a JSON object
  if ! jq -e 'type == "object"' "$config_path" > /dev/null 2>&1; then
    echo "Error: he-specs-lint missing/invalid config: Config must be a JSON object." >&2
    exit 2
  fi
  CONFIG_PATH="$config_path"
}

# ── Check a single spec file ──
check_spec() {
  local file_path="$1"
  local rel="${file_path#"${REPO_ROOT}"/}"

  # Extract frontmatter
  local fm_text
  if ! fm_text="$(extract_frontmatter "$file_path")"; then
    emit "error" "$rel" "Missing YAML frontmatter" \
      "Spec '${rel}' must start with YAML frontmatter delimited by '---' lines."
    return
  fi

  # Parse frontmatter key-value pairs
  declare -A FM_KV
  parse_frontmatter "$fm_text"

  # Required frontmatter keys from config
  local required_keys_json
  required_keys_json="$(jq -r '(.required_spec_frontmatter_keys // []) | if type == "array" then .[] else empty end' "$CONFIG_PATH" 2>/dev/null)" || true
  if [[ -n "$required_keys_json" ]]; then
    while IFS= read -r k; do
      [[ -z "$k" ]] && continue
      if [[ -z "${FM_KV[$k]+x}" ]]; then
        emit "error" "$rel" "Missing frontmatter key" \
          "Spec '${rel}' missing YAML frontmatter key '${k}:'."
      fi
    done <<< "$required_keys_json"
  fi

  # Validate plan_mode
  local plan_mode="${FM_KV[plan_mode]:-}"
  plan_mode="$(echo "$plan_mode" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [[ -n "$plan_mode" && "$plan_mode" != "trivial" && "$plan_mode" != "lightweight" && "$plan_mode" != "execution" ]]; then
    emit "error" "$rel" "Invalid plan_mode" \
      "Spec '${rel}' has invalid plan_mode '${plan_mode}' (must be 'trivial', 'lightweight', or 'execution')."
  fi

  # Validate spike_recommended
  local spike_rec="${FM_KV[spike_recommended]:-}"
  spike_rec="$(echo "$spike_rec" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [[ -n "$spike_rec" && "$spike_rec" != "yes" && "$spike_rec" != "no" ]]; then
    emit "error" "$rel" "Invalid spike_recommended" \
      "Spec '${rel}' has invalid spike_recommended '${spike_rec}' (must be 'yes' or 'no')."
  fi

  # Required headings
  local -a required_headings
  if [[ "$plan_mode" == "trivial" ]]; then
    required_headings=("${DEFAULT_TRIVIAL_REQUIRED_HEADINGS[@]}")
  else
    required_headings=("${DEFAULT_REQUIRED_HEADINGS[@]}")
  fi
  for h in "${required_headings[@]}"; do
    if ! has_exact_line "$file_path" "$h"; then
      emit "error" "$rel" "Missing heading" \
        "Spec '${rel}' missing required heading line '${h}'."
    fi
  done

  # Placeholder patterns
  local -a placeholder_patterns=()
  local patterns_json
  patterns_json="$(jq -r '(.artifact_placeholder_patterns // []) | if type == "array" then .[] else empty end' "$CONFIG_PATH" 2>/dev/null)" || true
  if [[ -n "$patterns_json" ]]; then
    while IFS= read -r p; do
      [[ -n "$p" ]] && placeholder_patterns+=("$p")
    done <<< "$patterns_json"
  fi

  local fail_ph="${HARNESS_FAIL_ON_ARTIFACT_PLACEHOLDERS:-0}"
  if [[ ${#placeholder_patterns[@]} -gt 0 ]]; then
    check_placeholders "$rel" "$file_path" "$fail_ph" "${placeholder_patterns[@]}"
  fi
}

# ── Main ──
main() {
  load_config

  echo "he-specs-lint: starting"
  echo "Repro: bash scripts/ci/he-specs-lint.sh"

  local specs_dir="${REPO_ROOT}/docs/specs"
  if [[ ! -d "$specs_dir" ]]; then
    echo "he-specs-lint: OK (docs/specs not present)"
    exit 0
  fi

  # Collect spec files (*.md excluding README.md and index.md), sorted
  local -a files=()
  while IFS= read -r -d '' f; do
    local basename
    basename="$(basename "$f")"
    [[ "$basename" == "README.md" || "$basename" == "index.md" ]] && continue
    files+=("$f")
  done < <(find "$specs_dir" -maxdepth 1 -name '*.md' -print0 | sort -z)

  if [[ ${#files[@]} -eq 0 ]]; then
    echo "he-specs-lint: OK (no spec files)"
    exit 0
  fi

  for f in "${files[@]}"; do
    check_spec "$f"
  done

  if [[ $errors -gt 0 ]]; then
    echo "he-specs-lint: FAIL (${errors} error(s), ${warnings} warning(s))" >&2
    exit 1
  fi
  echo "he-specs-lint: OK (${warnings} warning(s))"
  exit 0
}

main "$@"
