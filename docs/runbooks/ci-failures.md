---
title: "Remediate CI Failures"
use_when: "A verify/release gate fails due to build/test/lint failures locally or in CI; you need a consistent triage and stop/escalate policy."
called_from:
  - he-verify-release
  - he-implement
---

# Remediate CI Failures

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

Treat CI failures as signal. The goal is not to make CI green by any means; it is to restore correctness with minimal, root-cause fixes.

## Triage Order

1. Confirm you are testing the right thing (branch, commit, env).
2. Identify failure class:
   - deterministic test failure
   - flaky test
   - lint/format/typecheck
   - build/tooling regression
3. Reduce to the smallest reproducer command.

## Deterministic Failures

- Add or adjust a real unit/e2e test when the failure indicates a missing assertion.
- Fix the underlying behavior; avoid "just loosen the test" unless the test is truly wrong.

## Flaky Failures

- If you can reproduce locally, fix like deterministic.
- If you cannot reproduce:
  - mark as `judgment required` and escalate with evidence per the calling skill's § Escalation
  - do not disable tests silently

## Tooling Failures

- Keep changes minimal and reversible.
- Prefer pinning/fixing the tool invocation over broad refactors.

## Required Evidence

- Command used to reproduce
- Short failure output excerpt
- Command/output showing the fix
