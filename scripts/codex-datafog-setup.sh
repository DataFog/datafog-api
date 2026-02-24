#!/usr/bin/env bash

set -euo pipefail

usage() {
	cat <<'EOF'
Usage:
  scripts/codex-datafog-setup.sh [options]

Options:
  --policy-url <url>     Policy service endpoint (default: http://localhost:8080)
  --shim-bin <path>      Path to datafog-shim binary (default: build ./cmd/datafog-shim)
  --codex-bin <path>     Path to codex binary (default: codex in PATH)
  --shim-dir <path>      Shim install dir (default: $DATAFOG_SHIM_DIR or ~/.datafog/shims)
  --event-sink <path>    NDJSON sink path (default: ~/.datafog/decisions.ndjson)
  --api-token <token>    Optional token forwarded to policy checks
  --mode <enforced|observe>  Enforcement mode for shim calls (default: enforced)
  --install-git          Also gate git commands through the same shim
  --dry-run              Print planned actions without changing files
  --help                 Show this help text
EOF
}

POLICY_URL="http://localhost:8080"
SHIM_BIN="${DATAFOG_SHIM_BINARY:-}"
CODEX_BIN="${DATAFOG_CODEX_BINARY:-}"
SHIM_DIR="${DATAFOG_SHIM_DIR:-${HOME}/.datafog/shims}"
EVENT_SINK="${DATAFOG_SHIM_EVENT_SINK:-${HOME}/.datafog/decisions.ndjson}"
MODE="enforced"
API_TOKEN="${DATAFOG_API_TOKEN:-${DATAFOG_SHIM_API_TOKEN:-}}"
DRY_RUN=0
INSTALL_GIT=0

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
		--shim-dir)
			[[ $# -ge 2 ]] || { echo "missing --shim-dir value" >&2; exit 1; }
			SHIM_DIR="$2"
			shift 2
			;;
		--event-sink)
			[[ $# -ge 2 ]] || { echo "missing --event-sink value" >&2; exit 1; }
			EVENT_SINK="$2"
			shift 2
			;;
		--api-token)
			[[ $# -ge 2 ]] || { echo "missing --api-token value" >&2; exit 1; }
			API_TOKEN="$2"
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
		--install-git)
			INSTALL_GIT=1
			shift
			;;
		--dry-run)
			DRY_RUN=1
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

if [[ -z "${CODEX_BIN}" ]]; then
	CODEX_BIN="$(command -v codex || true)"
	if [[ -z "${CODEX_BIN}" ]]; then
		echo "codex binary not found in PATH. Pass --codex-bin <path>." >&2
		exit 1
	fi
fi

if [[ -z "${SHIM_BIN}" ]]; then
	ROOT_DIR="$(git rev-parse --show-toplevel)"
	SHIM_BIN="${ROOT_DIR}/datafog-shim"
fi

if [[ ! -x "${SHIM_BIN}" ]]; then
	if (( DRY_RUN )); then
		echo "[dry-run] would build datafog-shim at ${SHIM_BIN}"
	else
		echo "building datafog-shim at ${SHIM_BIN}"
		go build -o "${SHIM_BIN}" ./cmd/datafog-shim
	fi
fi

if (( DRY_RUN )); then
	echo "[dry-run] would install shim for codex"
else
	mkdir -p "${SHIM_DIR}"
fi

shim_args=(
	"--force"
	"--policy-url" "${POLICY_URL}"
	"--mode" "${MODE}"
	"--event-sink" "${EVENT_SINK}"
	"--shim-dir" "${SHIM_DIR}"
)

if [[ -n "${API_TOKEN}" ]]; then
	shim_args+=( "--api-token" "${API_TOKEN}" )
fi

run_cmd=(
	"${SHIM_BIN}"
	"hooks"
	"install"
	"${shim_args[@]}"
	"--adapter" "codex"
	"--target" "${CODEX_BIN}"
	"codex"
)

if (( DRY_RUN )); then
	printf '[dry-run] %q ' "${run_cmd[@]}"
	printf '\n'
else
	echo "installing codex shim: ${SHIM_DIR}/codex"
	"${run_cmd[@]}"
fi

if (( INSTALL_GIT )); then
	if (( DRY_RUN )); then
		printf '[dry-run] %q --adapter vcs --target "$(command -v git)" git\n' \
			"${SHIM_BIN} hooks install"
	else
		"${SHIM_BIN}" hooks install "${shim_args[@]}" --adapter vcs --target "$(command -v git)" git || {
			echo "failed to install git shim" >&2
			exit 1
		}
	fi
fi

if (( DRY_RUN )); then
	echo "[dry-run] would prepend ${SHIM_DIR} to PATH in shell startup file"
else
	echo "shim install complete"
fi

CONFIG_DIR="${HOME}/.datafog"
mkdir -p "${CONFIG_DIR}"
ENV_FILE="${CONFIG_DIR}/codex-datafog.env"

cat >"${ENV_FILE}" <<EOF
export DATAFOG_SHIM_POLICY_URL="${POLICY_URL}"
export DATAFOG_SHIM_MODE="${MODE}"
export DATAFOG_SHIM_EVENT_SINK="${EVENT_SINK}"
export DATAFOG_SHIM_DIR="${SHIM_DIR}"
EOF

if [[ -n "${API_TOKEN}" ]]; then
	printf 'export DATAFOG_SHIM_API_TOKEN=%q\n' "${API_TOKEN}" >>"${ENV_FILE}"
fi

if (( DRY_RUN )); then
	echo "[dry-run] would write ${ENV_FILE}"
else
	echo "wrote environment helper: ${ENV_FILE}"
fi

cat <<EOF
setup summary:
  policy url : ${POLICY_URL}
  codex shim : ${SHIM_DIR}/codex
  event sink : ${EVENT_SINK}
  mode       : ${MODE}
  env file   : ${ENV_FILE}

To activate this setup in your shell:
  source ${ENV_FILE}
  export PATH="${SHIM_DIR}:\$PATH"

Then run:
  codex --help
EOF
