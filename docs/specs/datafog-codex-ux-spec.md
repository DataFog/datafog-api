---
slug: datafog-codex-agent-ux
plan_mode: execution
status: active
owner: sidmohan
created: 2026-02-23
---

# datafog + OpenAI Codex Setup UX Specification

## Purpose

Define a single-command onboarding flow that lets a user make a new Codex-driven coding workflow policy-aware quickly and safely, while keeping Codex command usage familiar.

The user experience should answer two things immediately:

- "How do I start without rewriting existing agent habits?"
- "How do I know policy controls are active and acting on Codex actions?"

## User story

As a developer:

1. I want to keep using `codex` for normal prompts and task execution.
2. I want policy enforcement to apply to risky actions before they hit tools.
3. I want to review decisions and adjust policy without dropping into low-level networking details.

## Desired setup experience

The workflow should feel like:

- step 1: run policy service,
- step 2: run one bootstrap command,
- step 3: source a generated env file and update PATH,
- step 4: confirm status with one `codex` command and one decision sink line.

This should take less than five minutes in a clean machine state.

## Acceptance goals

- Setup can be completed by running `scripts/codex-datafog-setup.sh` from repo root.
- Command completion should be deterministic even if the shim is already installed.
- The setup should not require editing system files automatically; it should produce explicit manual activation steps.
- Policy mode must be switchable between `enforced` and `observe` without reinstalling the shim.
- A new user can recover quickly with:
  - `--dry-run` to preview,
  - `--mode observe` for non-blocking validation,
  - `datafog-shim hooks uninstall codex --force` to remove the managed wrapper.

## Interaction flow

- Prerequisite: `datafog-api` is running at a known endpoint and returns `/health`.
- User runs bootstrap helper:
  - `./scripts/codex-datafog-setup.sh --policy-url http://localhost:8080`
- Helper resolves binary path for `codex`, installs managed wrapper using `datafog-shim hooks install --adapter codex`.
- Helper writes `~/.datafog/codex-datafog.env`.
- User sources env + PATH and runs:
  - `codex --help`
- On first execution, user sees either allow/deny output or policy decision message in command output and a sink event in `~/.datafog/decisions.ndjson`.

## Interaction improvements

- Use `run` adapters inferred by name so explicit `--adapter` is not required for common binaries like `codex`.
- Provide an explicit `adapters list` command so policy authors can align policy rules with shim-inferred canonical families.
- Keep a default shim command namespace visible (`codex`, with optional `git` if opted).
- Offer policy-mode switching through environment to avoid binary reinstall:
  - set `DATAFOG_SHIM_MODE=observe` in `~/.datafog/codex-datafog.env`.

## Scope for this release

In-scope:

- CLI bootstrap helper.
- Runbook for onboarding and rollback.
- Documentation updates to `README.md`.

Out of scope:

- Deep policy authoring UX inside this repo.
- Multi-agent federation or centralized policy profile UI.
- Non-shell platform-specific installer packaging.

## Metrics of successful UX

- Time-to-first-policy-aware action:
  - target: <= 5 minutes.
- First-run friction:
  - zero manual edits required for PATH/env file creation.
- Visibility:
  - decision event appears for at least one invoked action.
- Recovery time:
  - user can disable enforcement with observe mode within one edit to one env file line.
