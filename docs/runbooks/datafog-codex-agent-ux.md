---
title: "Datafog + Codex Agent UX setup"
use_when: "A developer wants a minimal, reliable setup flow for enforcing Datafog policy around OpenAI Codex agent actions."
called_from:
  - he-implement
  - he-spec
  - he-review
---

# Datafog + Codex Agent UX setup

This runbook captures the target user experience for creating a secure workflow that
adds policy checkpoints to OpenAI Codex without changing how users run Codex day-to-day.

The end state is:
- a normal `codex` command still works,
- every side-effect action passes through `datafog-shim` before execution,
- policy decisions and enforcement mode are explicit and reversible,
- setup can be validated in under two minutes.

## Why this UX is efficient

The onboarding experience should minimize manual glue code. A user should not have to learn policy JSON or low-level policy API endpoints before they can safely try the feature.

The flow is intentionally:

1. Start policy service.
2. Run one bootstrap command.
3. Source one generated env file.
4. Keep PATH updated with shim dir.

This makes "I have policy-aware coding agent behavior" a predictable sequence rather than a long shell script to memorize.

## Prerequisites

- `datafog-api` running and reachable (for example `http://localhost:8080`).
- `codex` binary on PATH or available by absolute path.
- `go` installed for shim build (first run only).
- Write access to shell startup files is optional: the runbook can stay in dry-run mode first.

## Fast setup flow

From repository root, run:

```sh
chmod +x scripts/codex-datafog-setup.sh
./scripts/codex-datafog-setup.sh --policy-url http://localhost:8080
```

The script:

- Builds `datafog-shim` (if needed),
- Installs a managed shim named `codex`,
- Writes a helper env file `~/.datafog/codex-datafog.env`,
- Shows a minimal activation checklist.

## Activation steps

After bootstrap:

```sh
source ~/.datafog/codex-datafog.env
export PATH="$HOME/.datafog/shims:$PATH"
```

Expected behavior after this is that running `which codex` should resolve to the shim path in `~/.datafog/shims`.

## Verification

Run:

```sh
DATAFOG_SHIM_API_TOKEN="<token_if_configured>" codex --help
```

With policy defaults in place, if there is a matching policy rule for the command action metadata:
- allow/allow_with_redaction: command executes and emits decision info,
- deny/transform: command is blocked with a visible `PolicyDecisionError` in the shim output.

Audit evidence can be checked by reading the configured sink:

```sh
tail -f ~/.datafog/decisions.ndjson
```

Expected NDJSON events include action type, tool `codex`, decision, and request IDs.

## Optional hardening knobs

- `--mode observe` for non-blocking rollout.
- `--mode enforced` for hard blocking.
- `--api-token` to enforce tokened policy API requests.
- `--install-git` to additionally gate `git` through the same shim family.

## Recovery and escape hatch

If a user is blocked during onboarding, run the shim in observe mode to collect logs:

```sh
./scripts/codex-datafog-setup.sh --policy-url http://localhost:8080 --mode observe
```

If needed, remove only generated files:

```sh
rm -f ~/.datafog/codex-datafog.env
datafog-shim hooks uninstall codex --force
```

## Reuse for repeated installs

Keep a local alias in shell rc:

```sh
alias datafog-codex-setup='cd /path/to/datafog-api && ./scripts/codex-datafog-setup.sh --policy-url http://localhost:8080'
```

Then iterate with:

```sh
datafog-codex-setup
```

This preserves a predictable bootstrap habit and makes team onboarding copy-pastable.
