---
title: "Datafog + Claude Code Agent UX setup"
use_when: "A developer wants a minimal, reliable setup flow for enforcing Datafog policy around Claude Code agent actions."
called_from:
  - he-implement
  - he-spec
  - he-review
---

# Datafog + Claude Code Agent UX setup

This runbook captures the user experience for creating a secure workflow that
adds policy checkpoints to Claude Code without changing how users run Claude Code day-to-day.

The end state is:
- a normal `claude` command still works,
- side-effect actions are checked against policy before execution,
- policy decisions and enforcement mode are explicit and reversible,
- setup can be validated in under two minutes.

## Why this UX is efficient

The onboarding experience should minimize manual glue code. A user should not have to know policy internals before trying the feature.

The flow is:

1. Start policy service.
2. Run one bootstrap command.
3. Source one generated env file.
4. Keep PATH updated with shim directory.

This makes "I have policy-aware coding agent behavior" a predictable, low-friction sequence.

## Prerequisites

- `datafog-api` running and reachable (for example `http://localhost:8080`).
- `claude` binary on PATH or available by absolute path.
- `go` installed for shim build (first run only).
- Shell startup files are optional; dry-run mode can be used first.

## Fast setup flow

From repository root, run:

```sh
chmod +x scripts/claude-datafog-setup.sh
./scripts/claude-datafog-setup.sh --policy-url http://localhost:8080
```

The script:

- Builds `datafog-shim` (if needed),
- Installs a managed shim named `claude`,
- Writes a helper env file `~/.datafog/claude-datafog.env`,
- Shows a minimal activation checklist.

## Activation steps

After bootstrap:

```sh
source ~/.datafog/claude-datafog.env
export PATH="$HOME/.datafog/shims:$PATH"
```

Expected behavior after this is that running `which claude` should resolve to the shim path in `~/.datafog/shims`.

## Verification

Run:

```sh
DATAFOG_SHIM_API_TOKEN="<token_if_configured>" claude --help
```

With policy defaults in place, if there is a matching policy rule for the command action metadata:
- allow/allow_with_redaction: command executes and emits decision info,
- deny/transform: command is blocked with a visible `PolicyDecisionError` in shim output.

Audit evidence can be checked from sink:

```sh
tail -f ~/.datafog/decisions.ndjson
```

Expected NDJSON events include action type, tool `claude`, decision, and request IDs.

## Optional hardening knobs

- `--mode observe` for non-blocking rollout.
- `--mode enforced` for hard blocking.
- `--api-token` to enforce tokened policy API requests.
- `--install-git` to additionally gate `git` through the same shim family.

## Recovery and escape hatch

If a user is blocked during onboarding, run in observe mode to collect logs:

```sh
./scripts/claude-datafog-setup.sh --policy-url http://localhost:8080 --mode observe
```

If needed, remove generated files:

```sh
rm -f ~/.datafog/claude-datafog.env
datafog-shim hooks uninstall claude --force
```

## Reuse for repeated installs

Keep a local alias in shell rc:

```sh
alias datafog-claude-setup='cd /path/to/datafog-api && ./scripts/claude-datafog-setup.sh --policy-url http://localhost:8080'
```

Then iterate with:

```sh
datafog-claude-setup
```
