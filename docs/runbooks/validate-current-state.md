---
title: "Validate Current State"
use_when: "Starting an initiative and you need to confirm you understand the current behavior, repo state, and baseline signals before changing code."
called_from:
  - he-workflow
  - he-implement
---

# Validate Current State

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

This runbook defines the minimum baseline checks before claiming you understand "what's broken" (or "what exists") today.

## Repo Baseline

- Confirm you are in the intended workspace (worktree/branch):
  - `git status --short --branch`
- Confirm clean-ish state (or record intentional local changes):
  - `git diff`
- Confirm remote + default branch context:
  - `git remote -v`

## Behavior Baseline (Customize Per Repo)

Record the exact commands used and a short excerpt of the output in the active plan.

- Boot the app/service:
  - `<command>`
- Run the fastest “is it alive” check:
  - `<command>`
- Run targeted tests for the area (if they exist):
  - `<command>`

## Evidence

Link evidence from `docs/plans/active/<slug>-plan.md` under:

- `Surprises & Discoveries` (what you observed)
- `Artifacts and Notes` (logs, screenshots, recordings)

