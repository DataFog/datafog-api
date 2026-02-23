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
		return
	fi

	run_capture "$command_label" "${command[@]}"
	append_markdown_row "$agent" "$mode" "$command_label" "$command_desc" "$LAST_RC" "${LAST_DECISION:-n/a}" "${LAST_RECEIPT:-n/a}" "$LAST_STDERR"
	append_csv_row "$agent" "$mode" "$command_label" "$command_desc" "$LAST_RC" "${LAST_DECISION:-n/a}" "${LAST_RECEIPT:-n/a}" "$LAST_STDERR"
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
		LAST_STDERR="skip-live enabled"
	else
		if [[ -x "$no_shim" ]]; then
			run_cli_probe "$adapter" "$no_shim" "with-datafog" "shim --help" "$no_shim" "--help"
		else
			LAST_RC="n/a"
			LAST_DECISION="n/a"
			LAST_RECEIPT="n/a"
			LAST_STDERR="shim path missing"
			append_markdown_row "$adapter" "with-datafog" "shim-wrapper" "install + datafog shim required" "n/a" "n/a" "n/a" "$LAST_STDERR"
			append_csv_row "$adapter" "with-datafog" "shim-wrapper" "install + datafog shim required" "n/a" "n/a" "n/a" "$LAST_STDERR"
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
	printf "\n## Interpretation notes\n\n" >>"$REPORT_MD"
	printf "%s\n" "- Use the two with-datafog columns to confirm decision behavior under shim enforcement." >>"$REPORT_MD"
	printf "%s\n" "- If a risky command is denied, look for \`decision=deny\` with a receipt ID in the right-most columns." >>"$REPORT_MD"
	printf "%s\n\n" "- If both Datafog paths are \`n/a\`, install wrappers or point \`--shim-bin\` at a built \`datafog-shim\` binary." >>"$REPORT_MD"
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
	echo_markdown_summary
	cat <<EOF > /tmp/datafog_demo_report_notice.txt
Datafog demo report generated in preflight-only mode.
EOF
else
	for adapter in codex claude; do
		emit_action_matrix_rows "$adapter" ""
	done
	echo_markdown_summary
fi

printf "\nReport generated:\n%s\n" "$REPORT_MD"
printf "\nCSV generated:\n%s\n" "$REPORT_CSV"
