---
title: "Merge Change"
use_when: "You have a GO decision and need the minimum merge gate (checks/approvals/evidence) before merging to the main branch."
called_from:
  - he-verify-release
---

# Merge Change

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

This runbook captures the repo-specific merge gate. Keep it short and make it objective where possible.

## Preconditions

See `he-github` SKILL.md § Merge for the canonical merge gate. Add repo-specific preconditions below.

## Merge Checklist (Customize Per Repo)

- Required approvals obtained
- Required checks passing
- Versioning/release notes updated (if applicable)
- Post-merge verification steps queued (see `docs/runbooks/verify-release.md`)

## Post-Merge

- Run the post-release checks documented in the plan
- If any regression is found, open a follow-up and record it in learnings
