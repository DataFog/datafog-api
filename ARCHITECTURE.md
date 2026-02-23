# Architecture

This file is a compact map to answer: "Where do I change code to do X?"

Write only stable facts. Do not include procedures, external links, or volatile implementation details.

Keep this file small:

- Prefer bullets over paragraphs
- Keep each bullet concise
- If it grows, move detail to `docs/` and keep only pointers here

## Purpose

2-4 bullets, max 6 lines total:

- System purpose: <what this repo/system does>
- Primary users/actors: <who uses it>
- Main runtime pieces: <CLI/API/worker/UI/etc>
- Primary flows: <highest-value flows>

## Codemap (Where To Change Code)

4-8 bullets plus one flow line, max 14 lines total:

- `path/or/module` -> owns <what>; key types: <TypeA>, <TypeB>
- `path/or/module` -> owns <what>; key types: <TypeC>

Flow: `<entry>` -> `<layer>` -> `<layer>` -> `<store/service>`

## Invariants (Must Remain True)

3-7 bullets, max 10 lines total:

- `X` must not depend on `Y`.
- Side effects occur only in `<boundary/module>`.
- Business rules live in `<layer>` and not in `<layer>`.
- Security/data boundary: `<what is sensitive and where it may flow>`.

## Details Live Elsewhere

3-6 pointers, max 8 lines total. Use path + short label only:

- `docs/PLANS.md` - workflow and artifact contract
- `docs/runbooks/` - procedures and checklists
- `docs/<DOMAIN>.md` - domain-specific guardrails
- `docs/generated/` - generated context snapshots
