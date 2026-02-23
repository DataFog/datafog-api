#!/bin/bash
set -euo pipefail

# Select runbooks whose called_from frontmatter matches a skill or step name.
# Prints matching runbook paths (relative to repo root) to stdout.

# --- Repo root: two parents up from this script's directory ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# --- CLI argument parsing ---
SKILL=""
STEP=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skill)
      SKILL="$2"
      shift 2
      ;;
    --step)
      STEP="$2"
      shift 2
      ;;
    *)
      echo "Usage: $0 --skill <name> [--step <name>]" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$SKILL" ]]; then
  echo "Error: --skill is required" >&2
  exit 1
fi

# --- Main logic ---
RUNBOOKS_DIR="$REPO_ROOT/docs/runbooks"

if [[ ! -d "$RUNBOOKS_DIR" ]]; then
  exit 0
fi

# Extract the frontmatter block (between first --- and next ---).
# Parse called_from entries. Print the file path if skill or step matches.
process_file() {
  local file="$1"
  local in_frontmatter=0
  local in_called_from=0
  local first_line=1
  local called_from_items=()

  while IFS= read -r line || [[ -n "$line" ]]; do
    local trimmed
    trimmed="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"

    # First non-empty consideration: frontmatter must start at line 1 with ---
    if [[ "$first_line" -eq 1 ]]; then
      first_line=0
      if [[ "$trimmed" == "---" ]]; then
        in_frontmatter=1
        continue
      else
        # No frontmatter
        return
      fi
    fi

    # Inside frontmatter
    if [[ "$in_frontmatter" -eq 1 ]]; then
      # Closing delimiter
      if [[ "$trimmed" == "---" ]]; then
        break
      fi

      # Skip empty lines and comments
      if [[ -z "$trimmed" || "$trimmed" == \#* ]]; then
        # Empty lines inside a YAML list block: keep scanning
        if [[ "$in_called_from" -eq 1 && -z "$trimmed" ]]; then
          continue
        fi
        continue
      fi

      # If we're collecting YAML list items for called_from
      if [[ "$in_called_from" -eq 1 ]]; then
        # Check if this is a list item (starts with -)
        if [[ "$trimmed" == -* ]]; then
          local item
          item="$(echo "$trimmed" | sed "s/^-[[:space:]]*//;s/^[\"']//;s/[\"']$//")"
          if [[ -n "$item" ]]; then
            called_from_items+=("$item")
          fi
          continue
        else
          # Not a list item; if it contains a colon it's a new key — stop collecting
          if echo "$trimmed" | grep -q ':'; then
            in_called_from=0
            # Fall through to process this line as a new key
          else
            continue
          fi
        fi
      fi

      # Check for key: value lines
      if echo "$trimmed" | grep -q ':'; then
        local key val
        key="$(echo "$trimmed" | sed 's/^\([^:]*\):.*/\1/' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
        val="$(echo "$trimmed" | sed 's/^[^:]*://' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"

        if [[ "$key" == "called_from" ]]; then
          # Inline list: called_from: [a, b]
          if [[ "$val" == \[* ]]; then
            local inner
            inner="$(echo "$val" | sed 's/^\[//;s/\]$//' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
            if [[ -n "$inner" ]]; then
              IFS=',' read -ra parts <<< "$inner"
              for part in "${parts[@]}"; do
                local cleaned
                cleaned="$(echo "$part" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
                if [[ -n "$cleaned" ]]; then
                  called_from_items+=("$cleaned")
                fi
              done
            fi
          else
            # YAML list form — start collecting on subsequent lines
            in_called_from=1
          fi
        fi
      fi
    fi
  done < "$file"

  # Check for matches
  for item in "${called_from_items[@]+"${called_from_items[@]}"}"; do
    if [[ "$item" == "$SKILL" ]]; then
      echo "${file#"$REPO_ROOT"/}"
      return
    fi
    if [[ -n "$STEP" && "$item" == "$STEP" ]]; then
      echo "${file#"$REPO_ROOT"/}"
      return
    fi
  done
}

# Find all .md files, sorted for deterministic output
while IFS= read -r mdfile; do
  process_file "$mdfile"
done < <(find "$RUNBOOKS_DIR" -name '*.md' -type f | sort)

exit 0
