---
title: "Reproduce Bug"
use_when: "You have a bug report and need a minimal, reliable reproduction with evidence before implementing a fix."
called_from:
  - he-implement
  - he-video
---

# Reproduce Bug

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

The goal is a smallest-possible reproducer you can run repeatedly to prove the bug exists and prove it is fixed.

## Repro Checklist

1. Write down the expected behavior vs observed behavior in plain language.
2. Reduce to one of:
   - a single command (unit/integration test, script, request), or
   - a single UI flow script (agent-browser), or
   - a single minimal fixture (input file, request payload).
3. Make it deterministic:
   - pin any randomness, time, or external dependencies when possible
   - record env/config assumptions

## Evidence Capture

- For UI/behavior: capture a short `failure` video via `he-video`.
- For non-UI: capture terminal output (command + short excerpt) and link it in the plan.

## Test Strategy (Preferred)

- Add a real unit or e2e test that fails on the current state.
- Avoid mock-only tests unless the repo explicitly documents an exception.

## Plan Updates

Update `docs/plans/active/<slug>-plan.md`:

- `Progress`: add/mark the repro artifact as complete only when repeatable
- `Artifacts and Notes`: link the repro command/script and evidence paths

