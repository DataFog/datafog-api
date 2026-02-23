---
title: "Address Review Findings"
use_when: "You have review findings in an active plan and need a consistent process to fix, re-run review, and document what changed."
called_from:
  - he-review
  - he-implement
---

# Address Review Findings

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

## Workflow

1. Triage findings by priority.
2. For each `critical`/`high`, do one of:
   - fix it (preferred), or
   - escalate per `he-review` SKILL.md § Escalation if behavior is ambiguous or risk is unclear.
3. For `medium`/`low`, either:
   - fix it, or
   - accept it explicitly in the plan with rationale and follow-up link.
4. Update evidence:
   - rerun the most relevant tests
   - update `Artifacts and Notes` with new proof
5. Update `Progress`, `Decision Log`, and `Revision Notes` in the plan to reflect what changed and why.
6. Re-run `he-review` if the change materially altered behavior or implementation.

## Re-entry Rules

See `he-review` SKILL.md § Re-entry Rules for the canonical gates (design-level issues and material behavior changes).
