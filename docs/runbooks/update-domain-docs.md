---
title: "Update Domain Docs"
use_when: "A change introduces new product/engineering policy (security, reliability, frontend, observability, design) that should be captured as durable guidance for future work."
called_from:
  - he-plan
  - he-implement
  - he-learn
  - he-doc-gardening
---

# Update Domain Docs

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

Domain docs live under `docs/` and capture stable, repo-specific policy. Update them when you learn something that will prevent future bugs, regressions, or confusion.

## What Counts As A Domain-Doc Change

- A recurring decision rule ("we always do X when Y").
- A new constraint or boundary (security model, data sensitivity, performance guardrails).
- An operational expectation (SLOs, monitoring, alerts, rollback rules).
- A UI/UX standard or accessibility requirement.

If it's a one-off procedure or checklist, prefer a runbook in `docs/runbooks/`.

## Where To Put It

- The registry: `docs/DOMAIN_DOCS.md` (what exists and why).
- The doc itself (when present): `docs/SECURITY.md`, `docs/RELIABILITY.md`, `docs/FRONTEND.md`, `docs/OBSERVABILITY.md`, `docs/DESIGN.md`, `docs/PRODUCT_SENSE.md`.

## How To Update (Minimum)

1. Add the smallest rule that will prevent the problem from recurring (short, testable language).
2. Include a concrete anchor:
   - a file path, command, config key, or observable behavior.
3. Avoid long procedures; link to a runbook if needed.
4. If the change implies enforcement, note the guardrail candidate (lint/test/CI gate) so it can be promoted later.

## When To Do This

- During `he-learn`, when converting "what happened" into durable prevention.
- During review, if you discover undocumented constraints the next contributor will trip over.
