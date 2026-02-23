---
title: "Review Findings"
use_when: "Writing or interpreting review findings in docs/plans/active/<slug>-plan.md under the Review Findings section."
called_from:
  - he-review
---

# Review Findings

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

Review findings must be actionable and verifiable. The goal is to let a future reader fix issues without rediscovering context.

## Required Fields

Each finding includes:

- priority: `critical|high|medium|low`
- location: file path + symbol or short pointer
- issue summary: what is wrong
- required action: what must change or what proof is missing
- owner: who is responsible (team/name/agent)

## Priority Rubric, No-Mocks Policy, Mandatory Coverage

Canonical definitions live in `he-review` SKILL.md. Add repo-specific examples or exceptions below — do not redefine the severity levels or gate rules.

## Acceptance Rules

- Unresolved `critical` or `high` blocks progression to verify/release.
- `medium` and `low` can proceed only if explicitly accepted in writing in the plan.
