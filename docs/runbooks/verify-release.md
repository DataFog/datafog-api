---
title: "Verify/Release"
use_when: "Running he-verify-release to decide GO/NO-GO with evidence, rollback readiness, and post-release checks recorded in the active plan."
called_from:
  - he-verify-release
---

# Verify/Release

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

The skill `he-verify-release` enforces the stable invariants; this document carries the details that change per project. Inputs: active plan (`docs/plans/active/<slug>-plan.md` with `## Verify/Release Decision`) and review findings (populated by `he-review`).

## Output

Fill in `## Verify/Release Decision` with:

- decision: `GO` or `NO-GO`
- date:
- open findings by priority (if any):
- evidence: links/paths to test output and E2E artifacts
- rollback: exact steps or pointers
- post-release checks: exact checks/queries/URLs
- owner:

## Verification Ladder (Customize Per Repo)

Define the repo's minimum ladder here. Keep it short and ordered.

1. Fast checks: format/lint/typecheck (if applicable)
2. Targeted tests for changed area
3. Full relevant suite (unit/e2e)
4. Manual/E2E scenario (required for user-visible changes)

Document the exact commands for this repo:

    # From repo root:
    <command>

## Evidence Requirements

- Prefer evidence that a reviewer can reproduce (commands + short transcripts).
- For UI changes, include screenshots or a short recording (see `docs/runbooks/record-evidence.md`).
- For regressions, include a "before vs after" behavior description in plain language.

## Rollback And Recovery

Record the rollback plan for this repo:

- What to revert (commit/flag/config)
- How to detect failure
- How to restore service/data (if relevant)

## Post-Release Checks

Record the minimum set of checks to run after merge/release:

- health checks / smoke path
- key metrics / dashboards (if any)
- error logs / alerts (if any)

## Escalation

If any of these apply, stop and escalate per `he-verify-release` SKILL.md § Escalation:

- Unclear risk to users/data
- Flaky or non-deterministic failures
- Rollback steps are missing or untested
- Evidence is incomplete but time pressure exists
