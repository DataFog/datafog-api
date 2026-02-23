#!/bin/bash
set -euo pipefail

# ── Repo root (two levels above this script) ──────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

DEFAULT_CONFIG_PATH="scripts/ci/he-docs-config.json"

# ── Default required headings ─────────────────────────────────────────
DEFAULT_REQUIRED_HEADINGS=(
  "## Purpose / Big Picture"
  "## Progress"
  "## Surprises & Discoveries"
  "## Decision Log"
  "## Outcomes & Retrospective"
  "## Context and Orientation"
  "## Milestones"
  "## Plan of Work"
  "## Concrete Steps"
  "## Validation and Acceptance"
  "## Idempotence and Recovery"
  "## Artifacts and Notes"
  "## Interfaces and Dependencies"
  "## Pull Request"
  "## Review Findings"
  "## Verify/Release Decision"
  "## Revision Notes"
)

# ── Counters ──────────────────────────────────────────────────────────
ERRORS=0
WARNINGS=0

# ── Emit a finding (GitHub annotation + stderr) ──────────────────────
emit() {
  local level="$1" file="$2" title="$3" msg="$4"
  if [[ -n "$file" ]]; then
    echo "::${level} file=${file},title=${title}::${msg}"
  else
    echo "::${level} title=${title}::${msg}"
  fi
  echo "${level^^}: ${msg}" >&2
  if [[ "$level" == "error" ]]; then
    (( ERRORS++ )) || true
  else
    (( WARNINGS++ )) || true
  fi
}

# ── Load config ───────────────────────────────────────────────────────
load_config() {
  local config_rel="${HARNESS_DOCS_CONFIG:-$DEFAULT_CONFIG_PATH}"
  local config_path="$REPO_ROOT/$config_rel"
  if [[ ! -f "$config_path" ]]; then
    echo "Error: he-plans-lint missing/invalid config: Missing config '${config_rel}'. Fix: create it (bootstrap should do this) or set HARNESS_DOCS_CONFIG." >&2
    exit 2
  fi
  # Validate it is a JSON object
  if ! jq -e 'type == "object"' "$config_path" >/dev/null 2>&1; then
    echo "Error: he-plans-lint missing/invalid config: Config must be a JSON object." >&2
    exit 2
  fi
  CONFIG_PATH="$config_path"
}

# ── Config helpers ────────────────────────────────────────────────────
cfg_get() {
  # $1 = jq expression, returns raw output
  jq -r "$1" "$CONFIG_PATH"
}

cfg_get_array() {
  # $1 = jq path to array, outputs one element per line
  jq -r "$1 // [] | if type == \"array\" then .[] else empty end" "$CONFIG_PATH" 2>/dev/null
}

# ── Extract YAML frontmatter (between first --- and second ---) ──────
# Sets FRONTMATTER variable. Returns 1 if no frontmatter found.
extract_frontmatter() {
  local file="$1"
  FRONTMATTER=""
  local first_line
  first_line="$(head -1 "$file")"
  if [[ "$(echo "$first_line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')" != "---" ]]; then
    return 1
  fi
  # Find closing --- (skip line 1, find next ---)
  local end_line
  end_line="$(awk 'NR > 1 && /^[[:space:]]*---[[:space:]]*$/ { print NR; exit }' "$file")"
  if [[ -z "$end_line" ]]; then
    return 1
  fi
  # Extract lines between line 2 and end_line-1
  FRONTMATTER="$(sed -n "2,$((end_line - 1))p" "$file")"
  return 0
}

# ── Parse frontmatter key-value pairs ────────────────────────────────
# Reads FRONTMATTER, outputs "key=value" lines
frontmatter_keys=()
frontmatter_vals=()

parse_frontmatter_kv() {
  frontmatter_keys=()
  frontmatter_vals=()
  while IFS= read -r raw; do
    local line
    line="$(echo "$raw" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    # Skip empty / comment lines
    [[ -z "$line" || "$line" == \#* ]] && continue
    # Must contain a colon
    [[ "$line" != *:* ]] && continue
    local key val
    key="$(echo "$line" | cut -d: -f1 | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    val="$(echo "$line" | cut -d: -f2- | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    frontmatter_keys+=("$key")
    frontmatter_vals+=("$val")
  done <<< "$FRONTMATTER"
}

# ── Lookup a frontmatter value by key ────────────────────────────────
fm_get() {
  local needle="$1"
  for i in "${!frontmatter_keys[@]}"; do
    if [[ "${frontmatter_keys[$i]}" == "$needle" ]]; then
      echo "${frontmatter_vals[$i]}"
      return 0
    fi
  done
  return 1
}

fm_has_key() {
  local needle="$1"
  for k in "${frontmatter_keys[@]}"; do
    [[ "$k" == "$needle" ]] && return 0
  done
  return 1
}

# ── Extract section lines (between heading and next ## heading) ──────
# Outputs section body lines to stdout
section_lines() {
  local file="$1" heading="$2"
  awk -v h="$heading" '
    BEGIN { found=0 }
    $0 == h { found=1; next }
    found && /^## / { exit }
    found { print }
  ' "$file"
}

# ── Check: exact heading line exists in file ─────────────────────────
has_exact_line() {
  local file="$1" needle="$2"
  grep -qxF "$needle" "$file"
}

# ── Check: Progress section ──────────────────────────────────────────
check_progress() {
  local file_rel="$1" file_abs="$2"
  local body
  body="$(section_lines "$file_abs" "## Progress")"

  # Check non-empty (has at least one non-blank line)
  if ! echo "$body" | grep -q '[^[:space:]]'; then
    emit "error" "$file_rel" "Missing Progress content" \
      "Plan '${file_rel}' has an empty ## Progress section."
    return
  fi

  # Check timestamped checkbox pattern
  if ! echo "$body" | grep -qE '^- \[[ xX]\] \([0-9]{4}-[0-9]{2}-[0-9]{2}[^)]*\) P[0-9]+'; then
    emit "error" "$file_rel" "Progress format" \
      "Plan '${file_rel}' must include timestamped progress checkboxes with IDs (e.g. '- [ ] (2026-02-15T12:00:00Z) P1 ...')."
  fi
}

# ── Check: checklists only in Progress ───────────────────────────────
check_checklists_only_in_progress() {
  local file_rel="$1" file_abs="$2"
  local bad=0
  local in_progress=0
  while IFS= read -r line; do
    if [[ "$line" == "## Progress" ]]; then
      in_progress=1
      continue
    fi
    if [[ "$line" == "## "* ]]; then
      in_progress=0
    fi
    if [[ $in_progress -eq 0 ]] && echo "$line" | grep -qE '^- \[[ xX]\]'; then
      bad=1
      break
    fi
  done < "$file_abs"

  if [[ $bad -eq 1 ]]; then
    emit "error" "$file_rel" "Checklist scope" \
      "Plan '${file_rel}' contains checklist items outside ## Progress."
  fi
}

# ── Check: Decision Log ──────────────────────────────────────────────
check_decision_log() {
  local file_rel="$1" file_abs="$2"
  local body
  body="$(section_lines "$file_abs" "## Decision Log")"

  if ! echo "$body" | grep -q '[^[:space:]]'; then
    emit "error" "$file_rel" "Missing Decision Log content" \
      "Plan '${file_rel}' has an empty ## Decision Log section."
    return
  fi

  if ! echo "$body" | grep -q '^- Decision:'; then
    emit "error" "$file_rel" "Decision format" \
      "Plan '${file_rel}' should record decisions using '- Decision:' entries."
  fi
}

# ── Check: Revision Notes ────────────────────────────────────────────
check_revision_notes() {
  local file_rel="$1" file_abs="$2"
  local body
  body="$(section_lines "$file_abs" "## Revision Notes")"

  if ! echo "$body" | grep -q '[^[:space:]]'; then
    emit "error" "$file_rel" "Missing Revision Notes content" \
      "Plan '${file_rel}' has an empty ## Revision Notes section."
    return
  fi

  if ! echo "$body" | grep -q '^- '; then
    emit "error" "$file_rel" "Revision Notes format" \
      "Plan '${file_rel}' should include at least one bullet in ## Revision Notes."
  fi
}

# ── Check: placeholder tokens ────────────────────────────────────────
check_placeholders() {
  local file_rel="$1" file_abs="$2"
  local fail_ph=0
  [[ "${HARNESS_FAIL_ON_ARTIFACT_PLACEHOLDERS:-0}" == "1" ]] && fail_ph=1

  local text
  text="$(cat "$file_abs")"

  while IFS= read -r pattern; do
    [[ -z "$pattern" ]] && continue
    if echo "$text" | grep -qF "$pattern"; then
      local level="warning"
      local msg="Plan '${file_rel}' contains placeholder token '${pattern}'."
      if [[ $fail_ph -eq 1 ]]; then
        level="error"
      else
        msg="${msg} (Set HARNESS_FAIL_ON_ARTIFACT_PLACEHOLDERS=1 to enforce.)"
      fi
      emit "$level" "$file_rel" "Placeholder token" "$msg"
      break
    fi
  done < <(cfg_get_array '.artifact_placeholder_patterns')
}

# ── Check a single plan file ─────────────────────────────────────────
check_plan() {
  local file_abs="$1"
  local file_rel="${file_abs#"$REPO_ROOT/"}"

  # Frontmatter
  if ! extract_frontmatter "$file_abs"; then
    emit "error" "$file_rel" "Missing YAML frontmatter" \
      "Plan '${file_rel}' must start with YAML frontmatter delimited by '---' lines."
    return
  fi

  parse_frontmatter_kv

  # Required frontmatter keys
  while IFS= read -r key; do
    [[ -z "$key" ]] && continue
    if ! fm_has_key "$key"; then
      emit "error" "$file_rel" "Missing frontmatter key" \
        "Plan '${file_rel}' missing YAML frontmatter key '${key}:'."
    fi
  done < <(cfg_get_array '.required_plan_frontmatter_keys')

  # plan_mode validation
  local plan_mode
  plan_mode="$(fm_get "plan_mode" 2>/dev/null || true)"
  plan_mode="$(echo "$plan_mode" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [[ -n "$plan_mode" && "$plan_mode" != "trivial" && "$plan_mode" != "lightweight" && "$plan_mode" != "execution" ]]; then
    emit "error" "$file_rel" "Invalid plan_mode" \
      "Plan '${file_rel}' has invalid plan_mode '${plan_mode}' (must be 'trivial', 'lightweight', or 'execution')."
  fi

  # Required headings
  for h in "${DEFAULT_REQUIRED_HEADINGS[@]}"; do
    if ! has_exact_line "$file_abs" "$h"; then
      emit "error" "$file_rel" "Missing heading" \
        "Plan '${file_rel}' missing required heading line '${h}'."
    fi
  done

  # Section-level checks
  check_progress "$file_rel" "$file_abs"
  check_checklists_only_in_progress "$file_rel" "$file_abs"
  check_decision_log "$file_rel" "$file_abs"
  check_revision_notes "$file_rel" "$file_abs"
  check_placeholders "$file_rel" "$file_abs"
}

# ══════════════════════════════════════════════════════════════════════
# Main
# ══════════════════════════════════════════════════════════════════════
load_config

echo "he-plans-lint: starting"
echo "Repro: bash scripts/ci/he-plans-lint.sh"

plans_active="$REPO_ROOT/docs/plans/active"
plans_completed="$REPO_ROOT/docs/plans/completed"

files=()
if [[ -d "$plans_active" ]]; then
  while IFS= read -r -d '' f; do
    files+=("$f")
  done < <(find "$plans_active" -maxdepth 1 -name '*.md' -print0 | sort -z)
fi

lint_completed="$(cfg_get '.lint_completed_plans // true')"
if [[ "$lint_completed" != "false" && -d "$plans_completed" ]]; then
  while IFS= read -r -d '' f; do
    files+=("$f")
  done < <(find "$plans_completed" -maxdepth 1 -name '*.md' -print0 | sort -z)
fi

if [[ ${#files[@]} -eq 0 ]]; then
  echo "he-plans-lint: OK (no plan files)"
  exit 0
fi

for f in "${files[@]}"; do
  check_plan "$f"
done

if [[ $ERRORS -gt 0 ]]; then
  echo "he-plans-lint: FAIL (${ERRORS} error(s), ${WARNINGS} warning(s))" >&2
  exit 1
fi

echo "he-plans-lint: OK (${WARNINGS} warning(s))"
exit 0
