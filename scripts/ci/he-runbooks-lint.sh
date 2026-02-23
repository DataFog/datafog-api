#!/bin/bash
set -euo pipefail

# ---------------------------------------------------------------------------
# he-runbooks-lint.sh  --  Lint runbook frontmatter & content
#
# Exit codes: 0=OK, 1=FAIL, 2=config error
# ---------------------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

DEFAULT_CONFIG_PATH="scripts/ci/he-docs-config.json"

ERRORS=0
WARNINGS=0

# ── env helpers ────────────────────────────────────────────────────────────

env_flag() {
  local name="$1"
  local default="${2:-0}"
  local val="${!name:-$default}"
  [[ "$val" == "1" ]]
}

# ── emit / annotate ───────────────────────────────────────────────────────

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
}

emit_and_count() {
  local level="$1" file="$2" title="$3" msg="$4"
  if [[ "$level" == "error" ]]; then
    (( ERRORS++ )) || true
  else
    (( WARNINGS++ )) || true
  fi
  emit "$level" "$file" "$title" "$msg"
}

# ── config ────────────────────────────────────────────────────────────────

load_config() {
  local config_rel="${HARNESS_DOCS_CONFIG:-$DEFAULT_CONFIG_PATH}"
  local config_path="$REPO_ROOT/$config_rel"
  if [[ ! -f "$config_path" ]]; then
    echo "Error: he-runbooks-lint missing/invalid config: Missing config '${config_rel}'. Fix: create it (bootstrap should do this) or set HARNESS_DOCS_CONFIG." >&2
    return 1
  fi
  # Validate it is a JSON object
  if ! jq -e 'type == "object"' "$config_path" >/dev/null 2>&1; then
    echo "Error: he-runbooks-lint missing/invalid config: Config must be a JSON object." >&2
    return 1
  fi
  CONFIG_PATH="$config_path"
}

# ── frontmatter extraction ────────────────────────────────────────────────

# Reads file, outputs the frontmatter block (lines between first --- and
# second ---) to stdout. Returns 1 if no frontmatter found.
extract_frontmatter() {
  local file="$1"
  local in_fm=0
  local first_line=1
  local block=""

  while IFS= read -r line || [[ -n "$line" ]]; do
    local trimmed
    trimmed="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    if (( first_line )); then
      first_line=0
      if [[ "$trimmed" == "---" ]]; then
        in_fm=1
        continue
      else
        return 1
      fi
    fi
    if (( in_fm )); then
      if [[ "$trimmed" == "---" ]]; then
        printf '%s' "$block"
        return 0
      fi
      if [[ -n "$block" ]]; then
        block="${block}"$'\n'"${line}"
      else
        block="${line}"
      fi
    fi
  done < "$file"

  # Reached EOF without closing ---
  return 1
}

# ── frontmatter parsing ──────────────────────────────────────────────────

# Sets global variables: FM_TITLE, FM_USE_WHEN, FM_CALLED_FROM (newline-
# separated list), FM_KEYS (newline-separated list), FM_HAS_CALLED_FROM.
parse_frontmatter() {
  local block="$1"

  FM_TITLE=""
  FM_USE_WHEN=""
  FM_CALLED_FROM=""
  FM_KEYS=""
  FM_HAS_CALLED_FROM=0

  local lines=()
  while IFS= read -r line; do
    lines+=("$line")
  done <<< "$block"

  local i=0
  local count=${#lines[@]}

  while (( i < count )); do
    local raw="${lines[$i]}"
    local trimmed
    trimmed="$(echo "$raw" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"

    # skip blanks and comments
    if [[ -z "$trimmed" || "$trimmed" == \#* ]]; then
      (( i++ )) || true
      continue
    fi

    # must contain a colon to be a key
    if [[ "$trimmed" != *:* ]]; then
      (( i++ )) || true
      continue
    fi

    local key val
    key="$(echo "$trimmed" | sed 's/:.*//' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    val="$(echo "$trimmed" | sed 's/^[^:]*://' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"

    if [[ -n "$key" ]]; then
      if [[ -n "$FM_KEYS" ]]; then
        FM_KEYS="${FM_KEYS}"$'\n'"${key}"
      else
        FM_KEYS="$key"
      fi
    fi

    if [[ "$key" == "title" ]]; then
      FM_TITLE="$(echo "$val" | sed "s/^[[:space:]]*//;s/[[:space:]]*$//;s/^[\"']//;s/[\"']$//")"
      (( i++ )) || true
      continue
    fi

    if [[ "$key" == "use_when" ]]; then
      FM_USE_WHEN="$(echo "$val" | sed "s/^[[:space:]]*//;s/[[:space:]]*$//;s/^[\"']//;s/[\"']$//")"
      (( i++ )) || true
      continue
    fi

    if [[ "$key" == "called_from" ]]; then
      FM_HAS_CALLED_FROM=1
      # Inline array form: [a, b, c]
      if [[ "$val" == \[* ]]; then
        parse_called_from_inline "$val"
        (( i++ )) || true
        continue
      fi
      # YAML list form
      local items=""
      (( i++ )) || true
      while (( i < count )); do
        local sub="${lines[$i]}"
        local sub_trimmed
        sub_trimmed="$(echo "$sub" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
        if [[ -z "$sub_trimmed" ]]; then
          (( i++ )) || true
          continue
        fi
        # If it looks like a new key (has colon, doesn't start with -)
        if [[ "$sub_trimmed" == *:* && "$sub_trimmed" != -* ]]; then
          break
        fi
        if [[ "$sub_trimmed" == -* ]]; then
          local item
          item="$(echo "$sub_trimmed" | sed 's/^-[[:space:]]*//' | sed "s/^[[:space:]]*//;s/[[:space:]]*$//;s/^[\"']//;s/[\"']$//")"
          if [[ -n "$item" ]]; then
            if [[ -n "$items" ]]; then
              items="${items}"$'\n'"${item}"
            else
              items="$item"
            fi
          fi
        fi
        (( i++ )) || true
      done
      FM_CALLED_FROM="$items"
      continue
    fi

    (( i++ )) || true
  done
}

# Parses inline [a, b, c] into FM_CALLED_FROM (newline-separated).
parse_called_from_inline() {
  local val="$1"
  FM_CALLED_FROM=""
  # Strip outer brackets
  local inner
  inner="$(echo "$val" | sed 's/^[[:space:]]*\[//;s/\][[:space:]]*$//')"
  inner="$(echo "$inner" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  if [[ -z "$inner" ]]; then
    return
  fi
  local IFS=','
  local parts
  read -ra parts <<< "$inner"
  for p in "${parts[@]}"; do
    local trimmed
    trimmed="$(echo "$p" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    if [[ -n "$trimmed" ]]; then
      if [[ -n "$FM_CALLED_FROM" ]]; then
        FM_CALLED_FROM="${FM_CALLED_FROM}"$'\n'"${trimmed}"
      else
        FM_CALLED_FROM="$trimmed"
      fi
    fi
  done
}

# ── suspicious gate-waiver check ─────────────────────────────────────────

# Checks file text for patterns that suggest waiving skill gates.
# Sets SUSPICIOUS_MATCH to the matched snippet, or empty string.
check_suspicious_gate_waiver() {
  local file="$1"

  SUSPICIOUS_MATCH=""

  local patterns=(
    '\b(skip|waive|override|ignore)\b.{0,80}\b(gate|review|verify|verify-release|security|data|tests?)\b'
    '\b(disable|turn off)\b.{0,80}\b(tests?|checks?|ci)\b'
    '\b(force merge|merge anyway|ignore failing)\b'
  )

  for pat in "${patterns[@]}"; do
    local match=""
    # Use grep -ioP for PCRE; fall back to grep -ioE
    match="$(grep -ioP "$pat" "$file" 2>/dev/null | head -1)" || true
    if [[ -z "$match" ]]; then
      match="$(grep -ioE "$pat" "$file" 2>/dev/null | head -1)" || true
    fi
    if [[ -z "$match" ]]; then
      continue
    fi

    # Find byte offset to check prefix for negation
    local byte_offset=""
    byte_offset="$(grep -iobP "$pat" "$file" 2>/dev/null | head -1 | cut -d: -f1)" || true
    if [[ -z "$byte_offset" ]]; then
      byte_offset="$(grep -iobE "$pat" "$file" 2>/dev/null | head -1 | cut -d: -f1)" || true
    fi

    if [[ -n "$byte_offset" ]] && (( byte_offset > 0 )); then
      local prefix_start=$(( byte_offset > 40 ? byte_offset - 40 : 0 ))
      local prefix_len=$(( byte_offset - prefix_start ))
      local prefix
      prefix="$(dd if="$file" bs=1 skip="$prefix_start" count="$prefix_len" 2>/dev/null | tr '[:upper:]' '[:lower:]')"
      # Check negation prefixes
      local negated=0
      for neg in "do not" "don't" "must not" "never" "cannot" "can't" "should not"; do
        if [[ "$prefix" == *"$neg"* ]]; then
          negated=1
          break
        fi
      done
      if (( negated )); then
        continue
      fi
    fi

    # Clean up the snippet
    local snippet
    snippet="$(echo "$match" | tr '\n' ' ' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    SUSPICIOUS_MATCH="$snippet"
    return 0
  done

  return 1
}

# ── lint a single runbook ─────────────────────────────────────────────────

lint_runbook() {
  local path="$1"
  local fail_missing_called_from="$2"
  local fail_extra_keys="$3"

  local rel="${path#"$REPO_ROOT/"}"
  local strict=0
  env_flag "HARNESS_STRICT_RUNBOOKS" "0" && strict=1 || true

  # --- frontmatter presence ---
  local block=""
  if ! block="$(extract_frontmatter "$path")"; then
    local level="warning"
    (( strict )) && level="error"
    emit_and_count "$level" "$rel" "Runbook frontmatter" \
      "Runbook '${rel}' must start with YAML frontmatter ('---')."
    return
  fi

  # --- parse ---
  parse_frontmatter "$block"

  # --- required fields ---
  if [[ -z "$FM_TITLE" ]]; then
    local level="warning"
    (( strict )) && level="error"
    emit_and_count "$level" "$rel" "Runbook frontmatter" \
      "Runbook '${rel}' frontmatter must include a 'title:' field."
  fi

  if [[ -z "$FM_USE_WHEN" ]]; then
    local level="warning"
    (( strict )) && level="error"
    emit_and_count "$level" "$rel" "Runbook frontmatter" \
      "Runbook '${rel}' frontmatter must include a 'use_when:' field."
  fi

  # --- called_from ---
  if (( ! FM_HAS_CALLED_FROM )) || [[ -z "$FM_CALLED_FROM" ]]; then
    local level="warning"
    if (( strict )) || [[ "$fail_missing_called_from" == "1" ]]; then
      level="error"
    fi
    emit_and_count "$level" "$rel" "Runbook frontmatter" \
      "Runbook '${rel}' frontmatter should include non-empty 'called_from:' (list of skills/steps where this runbook is applied)."
  fi

  # --- extra keys ---
  local extras=""
  if [[ -n "$FM_KEYS" ]]; then
    while IFS= read -r k; do
      if [[ "$k" != "title" && "$k" != "use_when" && "$k" != "called_from" ]]; then
        if [[ -n "$extras" ]]; then
          extras="${extras}, ${k}"
        else
          extras="$k"
        fi
      fi
    done <<< "$FM_KEYS"
  fi

  if [[ -n "$extras" ]]; then
    local level="warning"
    if (( strict )) || [[ "$fail_extra_keys" == "1" ]]; then
      level="error"
    fi
    emit_and_count "$level" "$rel" "Runbook frontmatter" \
      "Runbook '${rel}' has extra frontmatter key(s): ${extras}. Prefer keeping runbooks to {title,use_when,called_from} unless you have a strong reason."
  fi

  # --- suspicious gate-waiver language ---
  if check_suspicious_gate_waiver "$path"; then
    local level="warning"
    (( strict )) && level="error"
    emit_and_count "$level" "$rel" "Potential gate waiver" \
      "Runbook '${rel}' appears to suggest waiving skill-enforced gates: '${SUSPICIOUS_MATCH}'. Runbooks are additive only; skill gates win."
  fi
}

# ── iter_runbooks ─────────────────────────────────────────────────────────

iter_runbooks() {
  local dir="$1"
  if [[ ! -d "$dir" ]]; then
    return
  fi
  find "$dir" -name '*.md' -type f | sort
}

# ── main ──────────────────────────────────────────────────────────────────

main() {
  if ! load_config; then
    return 2
  fi

  local fail_missing_called_from=0
  env_flag "HARNESS_FAIL_ON_MISSING_RUNBOOK_CALLED_FROM" "0" && fail_missing_called_from=1 || true

  local fail_extra_keys=0
  env_flag "HARNESS_FAIL_ON_EXTRA_RUNBOOK_FRONTMATTER" "0" && fail_extra_keys=1 || true

  local runbooks_dir="$REPO_ROOT/docs/runbooks"

  echo "he-runbooks-lint: starting"
  echo "Repro: bash scripts/ci/he-runbooks-lint.sh"

  # --- expected runbooks from config ---
  local expected_runbooks
  expected_runbooks="$(jq -r '(.expected_runbooks // .required_runbooks // []) | if type == "array" then .[] else empty end' "$CONFIG_PATH" 2>/dev/null)" || true

  if [[ -n "$expected_runbooks" ]]; then
    while IFS= read -r rb; do
      [[ -z "$rb" ]] && continue
      if [[ ! -f "$REPO_ROOT/$rb" ]]; then
        emit_and_count "warning" "$rb" "Expected runbook missing" \
          "Missing runbook: '${rb}'. Policy: runbooks are additive and should not block forward progress. Fix: create it (run he-bootstrap) or remove it from expected_runbooks in config."
      fi
    done <<< "$expected_runbooks"
  fi

  # --- lint each runbook ---
  while IFS= read -r path; do
    [[ -z "$path" ]] && continue
    lint_runbook "$path" "$fail_missing_called_from" "$fail_extra_keys"
  done < <(iter_runbooks "$runbooks_dir")

  # --- summary ---
  if (( ERRORS > 0 )); then
    echo "he-runbooks-lint: FAIL (${ERRORS} error(s), ${WARNINGS} warning(s))" >&2
    return 1
  fi

  echo "he-runbooks-lint: OK (${WARNINGS} warning(s))"
  return 0
}

main "$@"
