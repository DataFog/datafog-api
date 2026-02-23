# AGENTS.md

## Start Here

This file is a map, not an encyclopedia.

The system of record is `docs/`. Keep durable knowledge (specs, plans, logs, decisions, checklists) there and link to it from here.

## Golden Principles

- Prove it works: never claim completion without running the most relevant validation (tests, build, or a small end-to-end check) or explicitly recording why it could not be run.
- Keep AGENTS.md minimal and stable; detailed procedure belongs in `docs/runbooks/`.

## Source Of Truth (Table Of Contents)

- Workflow contract + artifact rules: `docs/PLANS.md`
- Specs (intent): `docs/specs/`
- Spikes (investigation findings): `docs/spikes/`
- Plans (execution + evidence): `docs/plans/`
- Runbooks (process checklists): `docs/runbooks/`
- Generated context (scratchpad/reference): `docs/generated/`
- Architecture (if present): `ARCHITECTURE.md`

## Workflow (Phases)

intake -> spike (optional) -> plan -> implement -> review -> verify-release -> learn

If this file grows beyond a compact index, move detailed guidance into `docs/` and keep links here.
