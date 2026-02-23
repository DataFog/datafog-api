---
title: "Respond To Feedback"
use_when: "A PR has review comments or requested changes, and you need a consistent loop to address them with evidence and minimal diffs."
called_from:
  - he-github
  - he-review
  - he-implement
---

# Respond To Feedback

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

Treat feedback as new requirements. The objective is to address comments with the smallest correct change and keep the plan/evidence accurate.

## Triage

1. Group comments by theme (correctness, security/data, architecture, taste).
2. Identify which comments require code changes vs explanation-only.
3. For any comment that is ambiguous or high risk, escalate per `he-review` SKILL.md § Escalation.

## Commands (Recommended)

- Read comments:
  - `gh pr view --comments`
- Re-check CI:
  - `gh pr checks`
- Pull failed logs:
  - `gh run view --log-failed`

## Fix Loop

1. Make the root-cause fix.
2. Update tests/e2e evidence as needed.
3. Update the active plan:
   - `Progress` items
   - `Review Findings` (if you’re tracking findings there)
   - `Artifacts and Notes`
4. Push and re-check (when approved):
   - `git push`
   - `gh pr checks`

