#!/usr/bin/env bash

set -euo pipefail

usage() {
cat <<'EOF'
Usage:
  scripts/datafog-agent-demo-comparison.sh [options]

Options:
  --policy-url <url>          Datafog policy URL (default: http://localhost:8080)
  --shim-bin <path>           datafog-shim binary (default: build ./datafog-shim if missing)
  --codex-bin <path>          codex binary (default: codex in PATH)
  --claude-bin <path>         claude binary (default: claude in PATH)
  --shim-dir <path>           Shim install dir to check (default: ~/.datafog/shims)
  --event-sink <path>         NDJSON event sink (default: ~/.datafog/decisions.ndjson)
  --mode <enforced|observe>   Shim mode (default: enforced)
  --out-dir <path>            Directory for generated reports (default: docs/generated/datafog-demo-reports)
  --dry-run                   Do not execute commands, only emit scaffolded report
  --skip-live                 Skip live action probes, only emit preflight checks
  --help                      Show this help text
EOF
}

POLICY_URL="${DATAFOG_POLICY_URL:-http://localhost:8080}"
SHIM_BIN="${DATAFOG_SHIM_BINARY:-}"
CODEX_BIN="${DATAFOG_CODEX_BINARY:-}"
CLAUDE_BIN="${DATAFOG_CLAUDE_BINARY:-}"
SHIM_DIR="${DATAFOG_SHIM_DIR:-${HOME}/.datafog/shims}"
EVENT_SINK="${DATAFOG_SHIM_EVENT_SINK:-${HOME}/.datafog/decisions.ndjson}"
MODE="${DATAFOG_SHIM_MODE:-enforced}"
OUT_DIR="docs/generated/datafog-demo-reports"
DRY_RUN=0
SKIP_LIVE=0

while [[ $# -gt 0 ]]; do
	case "$1" in
		--policy-url)
			[[ $# -ge 2 ]] || { echo "missing --policy-url value" >&2; exit 1; }
			POLICY_URL="$2"
			shift 2
			;;
		--shim-bin)
			[[ $# -ge 2 ]] || { echo "missing --shim-bin value" >&2; exit 1; }
			SHIM_BIN="$2"
			shift 2
			;;
		--codex-bin)
			[[ $# -ge 2 ]] || { echo "missing --codex-bin value" >&2; exit 1; }
			CODEX_BIN="$2"
			shift 2
			;;
		--claude-bin)
			[[ $# -ge 2 ]] || { echo "missing --claude-bin value" >&2; exit 1; }
			CLAUDE_BIN="$2"
			shift 2
			;;
		--shim-dir)
			[[ $# -ge 2 ]] || { echo "missing --shim-dir value" >&2; exit 1; }
			SHIM_DIR="$2"
			shift 2
			;;
		--event-sink)
			[[ $# -ge 2 ]] || { echo "missing --event-sink value" >&2; }
			EVENT_SINK="$2"
			shift 2
			;;
		--mode)
			[[ $# -ge 2 ]] || { echo "missing --mode value" >&2; exit 1; }
			MODE="$2"
			if [[ "$MODE" != "enforced" && "$MODE" != "observe" ]]; then
				echo "--mode must be enforced or observe" >&2
				exit 1
			fi
			shift 2
			;;
		--out-dir)
			[[ $# -ge 2 ]] || { echo "missing --out-dir value" >&2; exit 1; }
			OUT_DIR="$2"
			shift 2
			;;
		--dry-run)
			DRY_RUN=1
			shift
			;;
		--skip-live)
			SKIP_LIVE=1
			shift
			;;
		--help|-h)
			usage
			exit 0
			;;
		*)
			echo "unknown flag: $1" >&2
			usage
			exit 1
			;;
	esac
done

if [[ -z "$CODEX_BIN" ]]; then
	CODEX_BIN="$(command -v codex || true)"
fi

if [[ -z "$CLAUDE_BIN" ]]; then
	CLAUDE_BIN="$(command -v claude || true)"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/datafog-demo-XXXXXX")"
trap 'rm -rf "${WORKDIR}"' EXIT
RUN_ID="$(date -u +"%Y%m%dT%H%M%SZ")"
REPORT_MD="${OUT_DIR}/datafog-agent-demo-${RUN_ID}.md"
REPORT_CSV="${OUT_DIR}/datafog-agent-demo-${RUN_ID}.csv"

if [[ -z "$SHIM_BIN" ]]; then
	SHIM_BIN="${REPO_ROOT}/datafog-shim"
fi

mkdir -p "$OUT_DIR"

if [[ "$MODE" == "observe" ]]; then
	DECISION_HINT="observe mode: decisions are logged, denials are not blocking"
elif [[ "$MODE" == "enforced" ]]; then
	DECISION_HINT="enforced mode: denials block execution"
else
	DECISION_HINT="mode=$MODE"
fi

LAST_RC=0
LAST_DECISION="n/a"
LAST_RECEIPT="n/a"
LAST_STDOUT="n/a"
LAST_STDERR="n/a"

RESULT_KEYS=()
RESULT_EXIT=()
RESULT_DECISION=()
RESULT_RECEIPT=()
RESULT_NOTES=()
RESULT_OUTCOME=()
RESULT_COMMAND=()

result_index_for_key() {
	local key="$1"
	local i

	for i in "${!RESULT_KEYS[@]}"; do
		if [[ "${RESULT_KEYS[$i]}" == "$key" ]]; then
			echo "$i"
			return 0
		fi
	done
	echo "-1"
	return 1
}

result_get() {
	local key="$1"
	local field="$2"
	local default="${3:-}"
	local idx

	idx="$(result_index_for_key "$key" || true)"
	if [[ -z "$idx" || "$idx" == "-1" ]]; then
		echo "$default"
		return 0
	fi

	case "$field" in
		exit)
			echo "${RESULT_EXIT[$idx]:-$default}"
			;;
		decision)
			echo "${RESULT_DECISION[$idx]:-$default}"
			;;
		receipt)
			echo "${RESULT_RECEIPT[$idx]:-$default}"
			;;
		notes)
			echo "${RESULT_NOTES[$idx]:-$default}"
			;;
		outcome)
			echo "${RESULT_OUTCOME[$idx]:-$default}"
			;;
		command)
			echo "${RESULT_COMMAND[$idx]:-$default}"
			;;
		*)
			echo "$default"
			;;
	esac
}

result_set() {
	local key="$1"
	local field="$2"
	local value="$3"
	local idx

	idx="$(result_index_for_key "$key" || true)"
	if [[ -z "$idx" || "$idx" == "-1" ]]; then
		idx="${#RESULT_KEYS[@]}"
		RESULT_KEYS+=("$key")
		RESULT_EXIT+=("")
		RESULT_DECISION+=("")
		RESULT_RECEIPT+=("")
		RESULT_NOTES+=("")
		RESULT_OUTCOME+=("")
		RESULT_COMMAND+=("")
	fi
	case "$field" in
		exit)
			RESULT_EXIT[$idx]="$value"
			;;
		decision)
			RESULT_DECISION[$idx]="$value"
			;;
		receipt)
			RESULT_RECEIPT[$idx]="$value"
			;;
		notes)
			RESULT_NOTES[$idx]="$value"
			;;
		command)
			RESULT_COMMAND[$idx]="$value"
			;;
		outcome)
			RESULT_OUTCOME[$idx]="$value"
			;;
		*)
			;;
	esac
}

SCENARIOS=(
	"cli-help:Open help command"
	"read-secret:Read .env.secret"
	"write-output:Write output artifact"
	"delete-artifact:Delete artifact"
)

CONTROL_SCENARIOS=(
	"policy-outage:Policy API outage fail-closed"
	"decide-redaction:Decide API returns allow_with_redaction"
	"transform-mask:Transform API masks PII"
)

result_key() {
	local agent=$1
	local mode=$2
	local probe=$3
	printf "%s|%s|%s" "$agent" "$mode" "$probe"
}

probe_outcome() {
	local mode=$1
	local rc=$2
	local decision=$3
	local notes=$4

	if [[ "$mode" == "without-datafog" ]]; then
		if [[ "$rc" == "0" ]]; then
			echo "ALLOWED"
		elif [[ "$notes" == "missing binary" ]]; then
			echo "SKIP"
		else
			echo "FAILED"
		fi
		return
	fi

	if [[ "$notes" == "datafog-shim missing" || "$notes" == "shim path missing" || "$notes" == "skip-live enabled" ]]; then
		echo "SKIP"
		return
	fi
	if [[ "$notes" == *"call decide API"* || "$notes" == *"No such host"* || "$notes" == *"connection refused"* ]]; then
		echo "ERROR"
		return
	fi
	if [[ "$decision" == "deny" ]]; then
		echo "BLOCKED"
		return
	fi
	if [[ "$decision" == "transform" ]]; then
		echo "TRANSFORM"
		return
	fi
	if [[ "$decision" == "allow_with_redaction" ]]; then
		echo "ALLOWED_WITH_REDACTION"
		return
	fi
	if [[ "$rc" == "0" ]]; then
		echo "ALLOWED"
	elif [[ "$rc" == "0" || "$rc" == "1" ]]; then
		echo "BLOCKED"
	else
		echo "ERROR"
	fi
}

record_probe_result() {
	local agent=$1
	local mode=$2
	local probe=$3
	local command=$4
	local rc=$5
	local decision=$6
	local receipt=$7
	local notes=$8

	local key
	key="$(result_key "$agent" "$mode" "$probe")"
	local outcome
	outcome="$(probe_outcome "$mode" "$rc" "$decision" "$notes")"
	result_set "$key" "exit" "$rc"
	result_set "$key" "decision" "$decision"
	result_set "$key" "receipt" "$receipt"
	result_set "$key" "notes" "$notes"
	result_set "$key" "command" "$command"
	result_set "$key" "outcome" "$outcome"
}

probe_status_text() {
	local agent=$1
	local mode=$2
	local probe=$3
	local key
	key="$(result_key "$agent" "$mode" "$probe")"
	local outcome
	local rc
	local decision
	local notes

	outcome="$(result_get "$key" "outcome" "UNKNOWN")"
	rc="$(result_get "$key" "exit" "n/a")"
	decision="$(result_get "$key" "decision" "n/a")"
	notes="$(result_get "$key" "notes" "n/a")"

	printf "%s (rc=%s)" "$outcome" "$rc"
	if [[ "$outcome" == "ALLOWED" && -n "$decision" && "$decision" != "n/a" ]]; then
		printf " [decision=%s]" "$decision"
	fi
	if [[ "$outcome" == "ERROR" ]]; then
		printf " (%s)" "$notes"
	fi
}

scenario_label() {
	local scenario=$1
	case "$scenario" in
		cli-help) echo "CLI help" ;;
		read-secret) echo "Read secret file" ;;
		write-output) echo "Write output" ;;
		delete-artifact) echo "Delete file" ;;
		*) echo "$scenario" ;;
	esac
}

scenario_risk() {
	local scenario=$1
	case "$scenario" in
		read-secret) echo "HIGH: sensitive file read" ;;
		write-output) echo "MED: output write" ;;
		delete-artifact) echo "HIGH: destructive delete" ;;
		cli-help) echo "LOW: informational" ;;
		*) echo "UNKNOWN" ;;
	esac
}

extract_from_file() {
	local file=$1
	local key=$2
	awk -v key="$key" '{
		idx = index($0, key "=")
		if (idx > 0) {
			value = substr($0, idx + length(key) + 1)
			sub(/ .*/, "", value)
			print value
			exit
		}
	}' "$file"
}

json_value() {
	local payload=$1
	local expr=$2
	local default=${3:-n/a}

	if ! command -v jq >/dev/null 2>&1; then
		echo "$default"
		return 0
	fi

	local value
	value="$(printf "%s" "$payload" | jq -r "$expr" 2>/dev/null || true)"
	if [[ -z "$value" || "$value" == "null" ]]; then
		echo "$default"
		return 0
	fi
	echo "$value"
}

run_http_probe() {
	local method=$1
	local url=$2
	local body=$3

	local out_file
	local err_file
	out_file="$(mktemp)"
	err_file="$(mktemp)"

	LAST_RC=0
	LAST_DECISION="n/a"
	LAST_RECEIPT="n/a"
	LAST_STDOUT=""
	LAST_STDERR=""

	if (( DRY_RUN )); then
		LAST_STDERR="DRY-RUN for: ${method} ${url}"
		rm -f "$out_file" "$err_file"
		return 0
	fi

	set +e
	if [[ "$method" == "GET" ]]; then
		curl -fsS -m 8 "$url" >"$out_file" 2>"$err_file"
	else
		curl -fsS -m 8 -H 'Content-Type: application/json' -X "$method" -d "$body" "$url" >"$out_file" 2>"$err_file"
	fi
	LAST_RC=$?
	set -e
	LAST_STDOUT="$(awk 'NR==1 { print; exit }' "$out_file")"
	LAST_STDERR="$(awk 'NR==1 { print; exit }' "$err_file")"
	[[ -z "$LAST_STDOUT" ]] && LAST_STDOUT="(none)"
	[[ -z "$LAST_STDERR" ]] && LAST_STDERR="(none)"

	rm -f "$out_file" "$err_file"
}

run_capture() {
	local label=$1
	shift

	local out_file
	local err_file
	out_file="$(mktemp)"
	err_file="$(mktemp)"

	LAST_RC=0
	LAST_DECISION="n/a"
	LAST_RECEIPT="n/a"
	LAST_STDOUT=""
	LAST_STDERR=""

	if (( DRY_RUN )); then
		LAST_STDERR="DRY-RUN for: $label"
		return 0
	fi

	set +e
	"$@" >"$out_file" 2>"$err_file"
	LAST_RC=$?
	set -e
	LAST_DECISION="$(extract_from_file "$err_file" "decision" || true)"
	LAST_RECEIPT="$(extract_from_file "$err_file" "receipt" || true)"
	LAST_STDOUT="$(awk 'NR==1 { print; exit }' "$out_file")"
	LAST_STDERR="$(awk 'NR==1 { print; exit }' "$err_file")"
	[[ -z "$LAST_STDOUT" ]] && LAST_STDOUT="(none)"
	[[ -z "$LAST_STDERR" ]] && LAST_STDERR="(none)"
	LAST_STDOUT=${LAST_STDOUT//$'\n'/}
	LAST_STDERR=${LAST_STDERR//$'\n'/}

	rm -f "$out_file" "$err_file"
}

escape_csv() {
	local value=$1
	value="${value//\"/\"\"}"
	printf '"%s"' "$value"
}

append_markdown_row() {
	printf "| %s | %s | %s | %s | %s | %s | %s | %s |\n" \
		"$1" "$2" "$3" "$4" "$5" "$6" "$7" "$8" >>"$REPORT_MD"
}

append_csv_row() {
	printf "%s,%s,%s,%s,%s,%s,%s,%s\n" \
		"$(escape_csv "$1")" \
		"$(escape_csv "$2")" \
		"$(escape_csv "$3")" \
		"$(escape_csv "$4")" \
		"$(escape_csv "$5")" \
		"$(escape_csv "$6")" \
		"$(escape_csv "$7")" \
		"$(escape_csv "$8")" >>"$REPORT_CSV"
}

emit_preflight() {
	cat >"$REPORT_MD" <<EOF
# Datafog Agent Comparison Demo Report

- Timestamp: ${RUN_ID}
- Policy URL: ${POLICY_URL}
- Shim directory: ${SHIM_DIR}
- Shim binary: ${SHIM_BIN}
- Event sink: ${EVENT_SINK}
- Datafog mode: ${MODE} (${DECISION_HINT})
- Dry run: ${DRY_RUN}
- Branch: $(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)

## Preflight

| Check | Status | Path |
| --- | --- | --- |
EOF

	if [[ -x "$SHIM_BIN" ]]; then
		printf "| datafog-shim binary | OK | %s |\n" "$SHIM_BIN" >>"$REPORT_MD"
	else
		printf "| datafog-shim binary | MISSING | %s |\n" "$SHIM_BIN" >>"$REPORT_MD"
	fi
	if [[ -x "${SHIM_DIR}/codex" ]]; then
		printf "| codex shim | OK | %s |\n" "${SHIM_DIR}/codex" >>"$REPORT_MD"
	else
		printf "| codex shim | MISSING | %s/codex |\n" "$SHIM_DIR" >>"$REPORT_MD"
	fi
	if [[ -x "${SHIM_DIR}/claude" ]]; then
		printf "| claude shim | OK | %s |\n" "${SHIM_DIR}/claude" >>"$REPORT_MD"
	else
		printf "| claude shim | MISSING | %s/claude |\n" "$SHIM_DIR" >>"$REPORT_MD"
	fi
	if [[ -n "${CODEX_BIN}" ]]; then
		printf "| codex binary | OK | %s |\n" "$CODEX_BIN" >>"$REPORT_MD"
	else
		printf "| codex binary | NOT FOUND | codex |\n" >>"$REPORT_MD"
	fi
	if [[ -n "${CLAUDE_BIN}" ]]; then
		printf "| claude binary | OK | %s |\n" "$CLAUDE_BIN" >>"$REPORT_MD"
	else
		printf "| claude binary | NOT FOUND | claude |\n" >>"$REPORT_MD"
	fi
	if command -v curl >/dev/null 2>&1; then
		if curl -fsS "${POLICY_URL}/health" >/dev/null 2>&1; then
			printf "| policy endpoint health | OK | %s/health |\n" "$POLICY_URL" >>"$REPORT_MD"
		else
			printf "| policy endpoint health | UNREACHABLE | %s/health |\n" "$POLICY_URL" >>"$REPORT_MD"
		fi
	else
		printf "| policy endpoint health | UNKNOWN | curl unavailable |\n" >>"$REPORT_MD"
	fi

	printf "\n## Baseline and Runtime Matrix\n\n" >>"$REPORT_MD"
	printf "| Cell | Mode | Probe | Command | Exit | Decision | Receipt | Notes |\n" >>"$REPORT_MD"
	printf "| --- | --- | --- | --- | --- | --- | --- | --- |\n" >>"$REPORT_MD"

	cat >"$REPORT_CSV" <<EOF
Cell,Mode,Probe,Command,Exit,Decision,Receipt,Notes
EOF
}

result_text() {
	local agent=$1
	local mode=$2
	local probe=$3
	local key
	local outcome
	local decision
	local notes

	key="$(result_key "$agent" "$mode" "$probe")"
	outcome="$(result_get "$key" "outcome" "UNKNOWN")"
	decision="$(result_get "$key" "decision" "n/a")"
	notes="$(result_get "$key" "notes" "n/a")"

	case "$outcome" in
		ALLOWED)
			if [[ -n "$decision" && "$decision" != "n/a" ]]; then
				printf "ALLOWED, decision=%s" "$decision"
			else
				printf "ALLOWED"
			fi
			;;
		BLOCKED)
			if [[ -n "$decision" && "$decision" != "n/a" ]]; then
				printf "BLOCKED, decision=%s" "$decision"
			else
				printf "BLOCKED"
			fi
			;;
		ERROR)
			printf "ERROR (%s)" "$notes"
			;;
		SKIP)
			printf "SKIP"
			;;
		FAILED)
			printf "FAILED"
			;;
		*)
			printf "%s" "$outcome"
			;;
	esac
}

outcome_delta() {
	local before="$1"
	local after="$2"

	if [[ "$before" == "ALLOWED" && "$after" == "BLOCKED" ]]; then
		printf "policy gate now blocks this action"
		return
	fi
	if [[ "$before" == "BLOCKED" && "$after" == "ALLOWED" ]]; then
		printf "policy gate now allows this action"
		return
	fi
	if [[ "$before" == "BLOCKED" && "$after" == "BLOCKED" ]]; then
		printf "still blocked (policy and baseline match)"
		return
	fi
	if [[ "$before" == "ALLOWED" && "$after" == "ALLOWED" ]]; then
		printf "no enforcement change"
		return
	fi
	if [[ "$after" == "SKIP" ]]; then
		printf "not executed (setup prerequisite missing)"
		return
	fi
	if [[ "$after" == "ERROR" ]]; then
		printf "control path failed (infra/setup issue)"
		return
	fi
	if [[ "$before" == "FAILED" ]]; then
		printf "baseline already failing"
		return
	fi
	printf "needs review"
}

result_change_impact() {
	local adapter=$1
	local scenario=$2
	local before_outcome=$3
	local after_outcome=$4
	local before_decision=$5
	local after_decision=$6
	local before_receipt=$7
	local after_receipt=$8

	if [[ "$after_outcome" == "BLOCKED" ]]; then
		if [[ "$after_decision" != "n/a" && -n "$after_decision" ]]; then
			if [[ -n "$after_receipt" && "$after_receipt" != "n/a" ]]; then
				printf "blocked (%s, receipt=%s)" "$after_decision" "$after_receipt"
			else
				printf "blocked (%s)" "$after_decision"
			fi
		else
			printf "blocked"
		fi
	elif [[ "$after_outcome" == "ALLOWED" && "$before_outcome" == "BLOCKED" ]]; then
		if [[ "$before_decision" != "n/a" && -n "$before_decision" ]]; then
			printf "allowed for %s (was previously blocked in baseline)" "$scenario"
		else
			printf "allowed (was previously blocked in baseline)"
		fi
	elif [[ "$after_outcome" == "SKIP" ]]; then
		printf "not run"
	elif [[ "$after_outcome" == "ERROR" ]]; then
		printf "error in enforcement path"
	else
		printf "%s" "$scenario"
		printf " outcome unchanged (%s)" "$after_outcome"
	fi
}

emit_story_matrix() {
	local adapter=$1
	local scenario_name
	local label
	local before_key
	local after_key
	local before_out
	local after_out
	local before_decision
	local after_decision
	local before_receipt
	local after_receipt

	printf "### Storyboard: %s (before vs with-datafog)\n\n" "$adapter"
	printf "| Action | Risk | Without Datafog | With Datafog | Outcome Delta | Story |\n" >>"$REPORT_MD"
	printf "| --- | --- | --- | --- | --- | --- |\n" >>"$REPORT_MD"

	for s in "${SCENARIOS[@]}"; do
		scenario_name="${s%%:*}"
		label="$(scenario_label "$scenario_name")"
		before_key="$(result_key "$adapter" "without-datafog" "$scenario_name")"
		after_key="$(result_key "$adapter" "with-datafog" "$scenario_name")"
		before_out="$(result_get "$before_key" "outcome" "UNKNOWN")"
		after_out="$(result_get "$after_key" "outcome" "UNKNOWN")"
		before_decision="$(result_get "$before_key" "decision" "n/a")"
		after_decision="$(result_get "$after_key" "decision" "n/a")"
		before_receipt="$(result_get "$before_key" "receipt" "n/a")"
		after_receipt="$(result_get "$after_key" "receipt" "n/a")"
		printf "| %s | %s | %s | %s | %s | %s |\n" \
			"$label" \
			"$(scenario_risk "$scenario_name")" \
			"$(result_text "$adapter" "without-datafog" "$scenario_name")" \
			"$(result_text "$adapter" "with-datafog" "$scenario_name")" \
			"$(outcome_delta "$before_out" "$after_out")" \
			"$(result_change_impact "$adapter" "$scenario_name" "$before_out" "$after_out" "$before_decision" "$after_decision" "$before_receipt" "$after_receipt")" \
			>>"$REPORT_MD"
	done

	printf "\n" >>"$REPORT_MD"
}

emit_blocked_story() {
	local blocked_count=0
	local adapter
	local s
	local scenario
	local key
	local outcome
	local decision
	local receipt
	local notes

	printf "\n## Bad actions this run (with-datafog mode)\n\n" >>"$REPORT_MD"
	printf "| Adapter | Scenario | Risk | Decision | Receipt | Why it was blocked |\n" >>"$REPORT_MD"
	printf "| --- | --- | --- | --- | --- | --- |\n" >>"$REPORT_MD"

	for adapter in codex claude; do
		for s in "${SCENARIOS[@]}"; do
			scenario="${s%%:*}"
			key="$(result_key "$adapter" "with-datafog" "$scenario")"
			outcome="$(result_get "$key" "outcome" "UNKNOWN")"
			decision="$(result_get "$key" "decision" "n/a")"
			receipt="$(result_get "$key" "receipt" "n/a")"
			notes="$(result_get "$key" "notes" "n/a")"
			if [[ "$outcome" == "BLOCKED" ]]; then
				((blocked_count += 1))
				printf "| %s | %s | %s | %s | %s | %s |\n" \
					"$adapter" \
					"$(scenario_label "$scenario")" \
					"$(scenario_risk "$scenario")" \
					"${decision:-n/a}" \
					"${receipt:-n/a}" \
					"${notes:-n/a}" \
					>>"$REPORT_MD"
			fi
		done
	done

	if (( blocked_count == 0 )); then
		printf "No explicit policy blocks recorded.\n\n" >>"$REPORT_MD"
	fi
}

run_control_policy_outage_probe() {
	local adapter="codex"
	local probe="policy-outage"
	local fail_url="http://127.0.0.1:1"

	if [[ ! -x "$SHIM_BIN" ]]; then
		record_probe_result \
			"control" \
			"policy" \
			"$probe" \
			"shim run --policy-url $fail_url" \
			"n/a" \
			"n/a" \
			"n/a" \
			"datafog-shim missing"
		return
	fi

	run_capture "$probe" \
		"$SHIM_BIN" \
		run \
		--adapter "$adapter" \
		--policy-url "$fail_url" \
		--mode "$MODE" \
		--target /bin/echo -- "policy outage control probe"

	record_probe_result \
		"control" \
		"policy" \
		"$probe" \
		"datafog-shim run --policy-url $fail_url --target /bin/echo" \
		"$LAST_RC" \
		"${LAST_DECISION:-n/a}" \
		"${LAST_RECEIPT:-n/a}" \
		"${LAST_STDERR:-n/a}"
}

run_control_decide_probe() {
	local probe="decide-redaction"
	local payload='{"action":{"type":"file.write","tool":"fs","resource":"notes.txt"},"text":"Contact alice@example.com for invoice details."}'

	if ! command -v curl >/dev/null 2>&1; then
		record_probe_result \
			"control" \
			"policy" \
			"$probe" \
			"POST ${POLICY_URL}/v1/decide" \
			"n/a" \
			"n/a" \
			"n/a" \
			"curl unavailable"
		return
	fi

	run_http_probe "POST" "${POLICY_URL}/v1/decide" "$payload"
	local decision
	local matches
	local plan
	local notes
	if (( LAST_RC == 0 )); then
		decision="$(json_value "$LAST_STDOUT" '.decision' 'n/a')"
		matches="$(json_value "$LAST_STDOUT" '.matched_rules | join(",")' 'n/a')"
		plan="$(json_value "$LAST_STDOUT" '.transform_plan | tostring' 'n/a')"
		notes="decision=${decision}; matches=${matches}; transform_plan=${plan}"
	else
		decision="n/a"
		notes="curl/endpoint failed: ${LAST_STDERR}"
	fi

	record_probe_result \
		"control" \
		"policy" \
		"$probe" \
		"POST ${POLICY_URL}/v1/decide" \
		"$LAST_RC" \
		"$decision" \
		"n/a" \
		"$notes"
}

run_control_transform_probe() {
	local probe="transform-mask"
	local payload='{"text":"Please email alice@example.com for invoice details.","mode":"mask"}'

	if ! command -v curl >/dev/null 2>&1; then
		record_probe_result \
			"control" \
			"policy" \
			"$probe" \
			"POST ${POLICY_URL}/v1/transform" \
			"n/a" \
			"n/a" \
			"n/a" \
			"curl unavailable"
		return
	fi

	run_http_probe "POST" "${POLICY_URL}/v1/transform" "$payload"
	local output
	local count
	local modes
	local notes
	if (( LAST_RC == 0 )); then
		output="$(json_value "$LAST_STDOUT" '.output' 'n/a')"
		count="$(json_value "$LAST_STDOUT" '.stats.entities_transformed' 'n/a')"
		modes="$(json_value "$LAST_STDOUT" '.stats.modes_applied' 'n/a')"
		notes="output=${output}; entities_transformed=${count}; modes=${modes}"
	else
		output="n/a"
		notes="curl/endpoint failed: ${LAST_STDERR}"
	fi

	record_probe_result \
		"control" \
		"policy" \
		"$probe" \
		"POST ${POLICY_URL}/v1/transform" \
		"$LAST_RC" \
		"transform" \
		"n/a" \
		"$notes"
}

emit_control_checks() {
	local probe_name
	local key
	local outcome
	local decision
	local notes
	local description

	printf "\n## Policy API control checks\n\n" >>"$REPORT_MD"
	printf "| Check | Outcome | Decision | Notes |\n" >>"$REPORT_MD"
	printf "| --- | --- | --- | --- |\n" >>"$REPORT_MD"

	for c in "${CONTROL_SCENARIOS[@]}"; do
		probe_name="${c%%:*}"
		description="${c#*:}"
		key="$(result_key "control" "policy" "$probe_name")"
		outcome="$(result_get "$key" "outcome" "UNKNOWN")"
		decision="$(result_get "$key" "decision" "n/a")"
		notes="$(result_get "$key" "notes" "n/a")"
		printf "| %s | %s | %s | %s |\n" \
			"$description" \
			"$outcome" \
			"${decision:-n/a}" \
			"${notes:-n/a}" \
			>>"$REPORT_MD"
	done
	printf "\n" >>"$REPORT_MD"
}

emit_risk_catalog() {
	printf "\n## High-value checks this demo evaluates\n\n" >>"$REPORT_MD"
	printf "%s\n" "- Read operation against .env.secret" >>"$REPORT_MD"
	printf "%s\n" "- Delete operation against workspace artifact" >>"$REPORT_MD"
	printf "%s\n" "- Output write (\`printf 'report=clean' > write.out\`)" >>"$REPORT_MD"
	printf "\n" >>"$REPORT_MD"
}

reset_demo_workspace() {
	local dir=$1
	mkdir -p "$dir"
	printf 'DATAFOG_FAKE_KEY=ak_test_12345\n' >"${dir}/.env.secret"
	printf 'artifact\n' >"${dir}/artifact.txt"
	printf 'notes\n' >"${dir}/notes.txt"
}

run_cli_probe() {
	local agent=$1
	local bin=$2
	local mode=$3
	local command_label=$4
	local command_desc=$5
	local command=("${@:6}")

	if [[ -z "$bin" ]]; then
		LAST_RC="n/a"
		LAST_DECISION="n/a"
		LAST_RECEIPT="n/a"
		LAST_STDERR="missing binary"
		append_markdown_row "$agent" "$mode" "$command_label" "$command_desc" "n/a" "n/a" "n/a" "$LAST_STDERR"
		append_csv_row "$agent" "$mode" "$command_label" "$command_desc" "n/a" "n/a" "n/a" "$LAST_STDERR"
		record_probe_result "$agent" "$mode" "$command_label" "$command_desc" "$LAST_RC" "$LAST_DECISION" "$LAST_RECEIPT" "$LAST_STDERR"
		return
	fi

	run_capture "$command_label" "${command[@]}"
	append_markdown_row "$agent" "$mode" "$command_label" "$command_desc" "$LAST_RC" "${LAST_DECISION:-n/a}" "${LAST_RECEIPT:-n/a}" "$LAST_STDERR"
	append_csv_row "$agent" "$mode" "$command_label" "$command_desc" "$LAST_RC" "${LAST_DECISION:-n/a}" "${LAST_RECEIPT:-n/a}" "$LAST_STDERR"
	record_probe_result "$agent" "$mode" "$command_label" "$command_desc" "$LAST_RC" "${LAST_DECISION:-n/a}" "${LAST_RECEIPT:-n/a}" "$LAST_STDERR"
}

run_datafog_action_probe() {
	local adapter=$1
	local mode=$2
	local scenario=$3
	local command_desc=$4
	local target_cmd=$5
	local workspace=${6:-}

	if [[ -z "$workspace" ]]; then
		workspace="$WORKDIR/$adapter-$mode-${scenario// /-}"
	fi
	reset_demo_workspace "$workspace"
	if [[ "$mode" == "without-datafog" ]]; then
		run_capture "$adapter $mode $scenario" /bin/sh -lc "cd '${workspace}' && ${target_cmd}"
	else
		if [[ ! -x "$SHIM_BIN" ]]; then
			LAST_RC="n/a"
			LAST_DECISION="n/a"
			LAST_RECEIPT="n/a"
			LAST_STDERR="datafog-shim missing"
		else
			run_capture "$adapter $mode $scenario" \
				"$SHIM_BIN" \
				run \
				--adapter "$adapter" \
				--policy-url "$POLICY_URL" \
				--event-sink "$EVENT_SINK" \
				--mode "$MODE" \
				--target /bin/sh -- -lc "cd '${workspace}' && ${target_cmd}"
		fi
	fi
	append_markdown_row "$adapter" "$mode" "$scenario" "$command_desc" "$LAST_RC" "${LAST_DECISION:-n/a}" "${LAST_RECEIPT:-n/a}" "$LAST_STDERR"
	append_csv_row "$adapter" "$mode" "$scenario" "$command_desc" "$LAST_RC" "${LAST_DECISION:-n/a}" "${LAST_RECEIPT:-n/a}" "$LAST_STDERR"
	record_probe_result "$adapter" "$mode" "$scenario" "$command_desc" "$LAST_RC" "${LAST_DECISION:-n/a}" "${LAST_RECEIPT:-n/a}" "$LAST_STDERR"
}

build_command_table() {
	local adapter=$1
	local bin=$2
	local no_shim="${SHIM_DIR}/${adapter}"

	# Baseline: native command.
	run_cli_probe "$adapter" "$bin" "without-datafog" "cli-help" "bin --help" "$bin" "--help"

	# Datafog wrapper path check.
	if (( SKIP_LIVE )); then
		LAST_RC="n/a"
		LAST_DECISION="n/a"
		LAST_RECEIPT="n/a"
		LAST_STDERR="skip-live enabled"
		record_probe_result "$adapter" "with-datafog" "cli-help" "install + datafog shim required" "$LAST_RC" "$LAST_DECISION" "$LAST_RECEIPT" "$LAST_STDERR"
		append_markdown_row "$adapter" "with-datafog" "cli-help" "install + datafog shim required" "n/a" "n/a" "n/a" "$LAST_STDERR"
		append_csv_row "$adapter" "with-datafog" "cli-help" "install + datafog shim required" "n/a" "n/a" "n/a" "$LAST_STDERR"
	else
		if [[ -x "$no_shim" ]]; then
			run_cli_probe "$adapter" "$no_shim" "with-datafog" "cli-help" "shim --help (shim install check)" "$no_shim" "--help"
		else
			LAST_RC="n/a"
			LAST_DECISION="n/a"
			LAST_RECEIPT="n/a"
			LAST_STDERR="shim path missing"
			record_probe_result "$adapter" "with-datafog" "cli-help" "install + datafog shim required" "$LAST_RC" "$LAST_DECISION" "$LAST_RECEIPT" "$LAST_STDERR"
			append_markdown_row "$adapter" "with-datafog" "cli-help" "install + datafog shim required" "n/a" "n/a" "n/a" "$LAST_STDERR"
			append_csv_row "$adapter" "with-datafog" "cli-help" "install + datafog shim required" "n/a" "n/a" "n/a" "$LAST_STDERR"
		fi
	fi
}

emit_action_matrix_rows() {
	local adapter=$1
	local bin=$2
	local dir="$WORKDIR/$adapter-actions"

	run_datafog_action_probe "$adapter" "without-datafog" "read-secret" "cat .env.secret" "cat .env.secret" "$dir"
	run_datafog_action_probe "$adapter" "with-datafog" "read-secret" "cat .env.secret" "cat .env.secret" "$dir"

	run_datafog_action_probe "$adapter" "without-datafog" "write-output" "printf 'report=clean' > write.out" "printf 'report=clean' > write.out" "$dir"
	run_datafog_action_probe "$adapter" "with-datafog" "write-output" "printf 'report=clean' > write.out" "printf 'report=clean' > write.out" "$dir"

	run_datafog_action_probe "$adapter" "without-datafog" "delete-artifact" "rm -f artifact.txt" "rm -f artifact.txt" "$dir"
	run_datafog_action_probe "$adapter" "with-datafog" "delete-artifact" "rm -f artifact.txt" "rm -f artifact.txt" "$dir"
}

echo_markdown_summary() {
	for adapter in codex claude; do
		emit_story_matrix "$adapter"
	done
	emit_blocked_story
	emit_control_checks
	emit_risk_catalog

	printf "\n## Interpretation notes\n\n" >>"$REPORT_MD"
	printf "%s\n" "- This report is ordered as: baseline before Datafog, then with-datafog enforcement." >>"$REPORT_MD"
	printf "%s\n" "- If an action appears in \"Bad actions this run,\" Datafog blocked or transformed it with a decision." >>"$REPORT_MD"
	printf "%s\n" "- \"High-value checks\" are the explicit operations this harness validates for risky behavior." >>"$REPORT_MD"
	printf "%s\n\n" "- If paths are \`n/a\`, install wrappers or point \`--shim-bin\` at a built \`datafog-shim\` binary." >>"$REPORT_MD"
	printf "## Next run\n\n" >>"$REPORT_MD"
	printf "%s\n" "- Keep the same report template and run with \`--mode observe\` first, then switch to \`--mode enforced\` after policy tuning." >>"$REPORT_MD"
	printf "%s\n" "- Use \`export PATH=\"${SHIM_DIR}:\$PATH\"\` after setup to ensure managed shims are hit for real agent traffic." >>"$REPORT_MD"
}

emit_preflight

for adapter in codex claude; do
	if [[ "$adapter" == "codex" ]]; then
		bin="$CODEX_BIN"
	else
		bin="$CLAUDE_BIN"
	fi
	build_command_table "$adapter" "$bin"
done

if (( SKIP_LIVE )); then
	cat <<EOF > /tmp/datafog_demo_report_notice.txt
Datafog demo report generated in preflight-only mode.
EOF
else
	for adapter in codex claude; do
		emit_action_matrix_rows "$adapter" ""
	done
fi

run_control_policy_outage_probe
run_control_decide_probe
run_control_transform_probe

echo_markdown_summary

printf "\nReport generated:\n%s\n" "$REPORT_MD"
printf "\nCSV generated:\n%s\n" "$REPORT_CSV"
