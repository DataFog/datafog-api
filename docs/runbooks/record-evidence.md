---
title: "Record Evidence"
use_when: "You need screenshots or short recordings as proof of failure and proof of resolution, especially for UI or behavior changes."
called_from:
  - he-video
  - he-verify-release
  - he-implement
---

# Record Evidence

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

Evidence should be easy to review, easy to find, and tied to an artifact (plan/PR) so it does not get lost.

## What To Capture

- Failure evidence: what is broken, with a minimal reproduction
- Resolution evidence: the same reproduction after the fix
- Any relevant logs or error output (short)

## Where To Put It

- Link evidence from:
  - `docs/plans/active/<slug>-plan.md` under `Artifacts and Notes` and `Verify/Release Decision`
  - the PR description (if one exists)

## Naming Convention

Use predictable names so evidence is searchable:

- `<slug>-failure.<ext>`
- `<slug>-resolution.<ext>`

If multiple clips exist:

- `<slug>-failure-1.<ext>`, `<slug>-resolution-1.<ext>`

## Minimum Bar

- If you claim a bug exists, there is at least one artifact showing it.
- If you claim it is fixed, there is at least one artifact showing the fix under the same scenario.
- Prefer short clips (10-60s) over long walkthroughs.
