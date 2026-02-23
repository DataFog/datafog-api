#!/bin/bash
set -euo pipefail

# ---------------------------------------------------------------------------
# he-spikes-lint.sh — Lint spike documents under docs/spikes
# ---------------------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

DEFAULT_CONFIG_PATH="scripts/ci/he-docs-config.json"

# Default required headings (one per line for easy iteration)
DEFAULT_REQUIRED_HEADINGS=(
  "## Context"
  "## Validation Goal"
  "## Approach"
  "## Findings"
  "## Decisions"
  "## Recommendation"
  "## Impact on Upstream Docs"
  "## Spike Code"
  "## Remaining Unknowns"
  "## Time Spent"
  "## Revision Notes"
)

# Counters
errors=0
warnings=0

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

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

# Extract YAML frontmatter (text between first two --- lines, exclusive).
# Prints frontmatter to stdout. Returns 1 if no valid frontmatter found.
extract_frontmatter() {
  local file="$1"
  local first_line
  first_line="$(head -n1 "$file" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [[ "$first_line" != "---" ]]; then
    return 1
  fi
  # Find the closing --- (skip line 1, start from line 2)
  local line_num=0
  local found=0
  while IFS= read -r line; do
    line_num=$((line_num + 1))
    if [[ $line_num -eq 1 ]]; then
      continue
    fi
    local trimmed
    trimmed="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    if [[ "$trimmed" == "---" ]]; then
      found=1
      break
    fi
    echo "$line"
  done < "$file"
  if [[ $found -eq 0 ]]; then
    return 1
  fi
  return 0
}

# Extract keys from frontmatter text (stdin).
# Outputs one key per line.
frontmatter_keys() {
  while IFS= read -r raw; do
    local line
    line="$(echo "$raw" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    # skip blank lines and comments
    [[ -z "$line" ]] && continue
    [[ "$line" == \#* ]] && continue
    # must contain a colon
    [[ "$line" != *:* ]] && continue
    # extract key (everything before first colon), trimmed
    local key
    key="$(echo "$line" | cut -d: -f1 | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    echo "$key"
  done
}

# Check if a file contains an exact full line matching the needle.
has_exact_line() {
  local file="$1" needle="$2"
  grep -qFx "$needle" "$file"
}

# ---------------------------------------------------------------------------
# Config loading
# ---------------------------------------------------------------------------

load_config() {
  local config_rel="${HARNESS_DOCS_CONFIG:-$DEFAULT_CONFIG_PATH}"
  local config_path="$REPO_ROOT/$config_rel"
  if [[ ! -f "$config_path" ]]; then
    echo "Error: he-spikes-lint missing/invalid config: Missing config '${config_rel}'. Fix: create it (bootstrap should do this) or set HARNESS_DOCS_CONFIG." >&2
    exit 2
  fi
  # Validate it is a JSON object
  if ! jq -e 'type == "object"' "$config_path" >/dev/null 2>&1; then
    echo "Error: he-spikes-lint missing/invalid config: Config must be a JSON object." >&2
    exit 2
  fi
  CONFIG_PATH="$config_path"
}

# Read a JSON array from config as newline-delimited strings.
config_string_array() {
  local key="$1"
  jq -r "(.${key} // []) | if type == \"array\" then .[] else empty end" "$CONFIG_PATH" 2>/dev/null | while IFS= read -r v; do
    # only emit strings
    echo "$v"
  done
}

# ---------------------------------------------------------------------------
# Per-spike checks
# ---------------------------------------------------------------------------

check_placeholders() {
  local rel="$1" file="$2" fail_ph="$3"
  shift 3
  local patterns=("$@")
  for p in "${patterns[@]}"; do
    [[ -z "$p" ]] && continue
    if grep -qF "$p" "$file"; then
      local msg="Spike '${rel}' contains placeholder token '${p}'."
      if [[ "$fail_ph" == "1" ]]; then
        emit "error" "$rel" "Placeholder token" "$msg"
      else
        emit "warning" "$rel" "Placeholder token" "${msg} (Set HARNESS_FAIL_ON_ARTIFACT_PLACEHOLDERS=1 to enforce.)"
      fi
      break
    fi
  done
}

check_spike() {
  local file="$1"
  local rel="${file#"$REPO_ROOT"/}"

  # --- frontmatter ---
  local fm
  if ! fm="$(extract_frontmatter "$file")"; then
    emit "error" "$rel" "Missing YAML frontmatter" \
      "Spike '${rel}' must start with YAML frontmatter delimited by '---' lines."
    return
  fi

  # Check required frontmatter keys
  local fm_keys
  fm_keys="$(echo "$fm" | frontmatter_keys)"

  local required_keys
  required_keys="$(config_string_array "required_spike_frontmatter_keys")"

  if [[ -n "$required_keys" ]]; then
    while IFS= read -r k; do
      [[ -z "$k" ]] && continue
      if ! echo "$fm_keys" | grep -qFx "$k"; then
        emit "error" "$rel" "Missing frontmatter key" \
          "Spike '${rel}' missing YAML frontmatter key '${k}:'."
      fi
    done <<< "$required_keys"
  fi

  # --- required headings ---
  for h in "${DEFAULT_REQUIRED_HEADINGS[@]}"; do
    if ! has_exact_line "$file" "$h"; then
      emit "error" "$rel" "Missing heading" \
        "Spike '${rel}' missing required heading line '${h}'."
    fi
  done

  # --- placeholder tokens ---
  local placeholder_patterns=()
  while IFS= read -r p; do
    [[ -z "$p" ]] && continue
    placeholder_patterns+=("$p")
  done < <(config_string_array "artifact_placeholder_patterns")

  local fail_ph="${HARNESS_FAIL_ON_ARTIFACT_PLACEHOLDERS:-0}"
  if [[ ${#placeholder_patterns[@]} -gt 0 ]]; then
    check_placeholders "$rel" "$file" "$fail_ph" "${placeholder_patterns[@]}"
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

load_config

echo "he-spikes-lint: starting"
echo "Repro: bash scripts/ci/he-spikes-lint.sh"

spikes_dir="$REPO_ROOT/docs/spikes"
if [[ ! -d "$spikes_dir" ]]; then
  echo "he-spikes-lint: OK (docs/spikes not present)"
  exit 0
fi

# Collect spike files sorted
spike_files=()
while IFS= read -r -d '' f; do
  spike_files+=("$f")
done < <(find "$spikes_dir" -maxdepth 1 -name '*-spike.md' -print0 | sort -z)

if [[ ${#spike_files[@]} -eq 0 ]]; then
  echo "he-spikes-lint: OK (no spike files)"
  exit 0
fi

for f in "${spike_files[@]}"; do
  check_spike "$f"
done

if [[ $errors -gt 0 ]]; then
  echo "he-spikes-lint: FAIL (${errors} error(s), ${warnings} warning(s))" >&2
  exit 1
fi

echo "he-spikes-lint: OK (${warnings} warning(s))"
exit 0
