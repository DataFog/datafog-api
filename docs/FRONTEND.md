---
title: "Frontend"
use_when: "Documenting frontend stack, conventions, component architecture, performance budgets, and accessibility requirements for this repo."
---

## Stack
- Define supported browsers/platforms and the minimum accessibility target.
- Prefer a small set of core dependencies and consistent build tooling across the app.

## Conventions
- Keep components small and named by what they do; avoid "utils soup" without ownership.
- Centralize shared UI primitives; avoid duplicating patterns across pages.

## Component Architecture
- Separate UI rendering from data fetching/mutations where practical.
- Prefer explicit data flow and local state; introduce global state only with a clear boundary.

## Performance
- Avoid unnecessary client work: minimize re-renders, split code on route/feature boundaries, and lazy-load heavy modules.
- Measure before optimizing; keep a short list of performance budgets that matter to users.

## Accessibility
- Keyboard navigation works for all interactive controls; focus states are visible.
- Use semantic HTML first; ARIA is for filling gaps, not replacing semantics.
