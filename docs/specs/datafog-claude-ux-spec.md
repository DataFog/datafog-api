---
slug: datafog-claude-agent-ux
plan_mode: execution
status: active
owner: sidmohan
created: 2026-02-23
---

# datafog + Claude Setup UX Specification

## Purpose

Define a one-command-ish onboarding flow so a developer can make a Claude Code workflow
policy-aware quickly and reliably, while keeping Claude command usage familiar.

The user experience should answer:

- "How do I start without rewriting my current workflow?"
- "How do I verify policy controls are active on Claude actions?"

## User story

As a developer:

1. I want to keep using `claude` for normal prompts and agent tasks.
2. I want policy enforcement to apply to side-effect actions before execution.
3. I want to review decisions and adjust policy without low-level troubleshooting.

## Desired setup experience

Setup flow:

- step 1: run policy service,
- step 2: run one bootstrap helper,
- step 3: source one env file and update PATH,
- step 4: confirm status with one Claude command and one decision log line.

This should take less than five minutes in a clean machine.

## Acceptance goals

- Setup can be completed by running `scripts/claude-datafog-setup.sh` from repo root.
- The flow should be deterministic even if the shim is already installed.
- Setup should not require editing system files automatically; explicit activation steps are shown.
- Policy mode must be switchable between `enforced` and `observe` without reinstalling shim.
- A user can recover quickly with:
  - `--dry-run` to preview,
  - `--mode observe` for no-blocking validation,
  - `datafog-shim hooks uninstall claude --force` to remove the managed wrapper.

## Interaction flow

- Prerequisite: `datafog-api` is running at a known endpoint and returns `/health`.
- User runs bootstrap helper:
  - `./scripts/claude-datafog-setup.sh --policy-url http://localhost:8080`
- Helper resolves the `claude` binary and installs managed wrapper with `datafog-shim hooks install --adapter claude`.
- Helper writes `~/.datafog/claude-datafog.env`.
- User sources env + PATH and runs:
  - `claude --help`
- On first usage, user sees either allow/deny/transform behavior and decision event in sink.

## Interaction improvements

- Use adapter inference so explicit `--adapter` is not required when using normal Claude invocation.
- Provide `adapters` introspection so policy authors can align rules with canonical action families.
- Keep the managed wrapper command namespace visible (`claude`, with optional `git` if opted).
- Permit fast mode switching via env var in `~/.datafog/claude-datafog.env`:
  - `DATAFOG_SHIM_MODE=observe`.

## Scope for this release

In-scope:

- Claude bootstrap helper script.
- Runbook for onboarding and rollback.
- README and docs spec updates.

Out of scope:

- Native policy authoring UX.
- Cross-agent orchestration platform.
- GUI installer package for all shells.

## Metrics of success

- Time-to-first-policy-aware action:
  - target <= 5 minutes.
- First-run friction:
  - no manual edits required for PATH/env file creation.
- Visibility:
  - at least one decision event appears in NDJSON after first controlled action.
- Recovery time:
  - user can switch to observe mode with one env file line change.
