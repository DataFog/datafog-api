---
title: "Data"
use_when: "Capturing data model and data-change safety rules for this repo (schemas, migrations, backfills, integrity, and operational safety)."
---

## Data Model

- Source of truth for schemas (ORM models, migrations, schema dump files) and where they live.
- Entity ownership boundaries (what owns IDs, who can write which tables/collections).

## Migrations

- Migration rules (forward-only vs reversible, locking/online migration expectations, index/constraint strategy).
- Validation steps for schema changes (commands and what to check).

## Backfills And Data Fixes

- How to run backfills safely (idempotence, batching, checkpoints).
- How to verify correctness and how to roll back (or compensate) if needed.

## Integrity And Consistency

- Constraints and invariants that must remain true (unique keys, foreign keys, referential rules).
- Concurrency expectations (transactions/isolation, retry policies) where relevant.

## Sensitive Data Notes

- Pointers to where sensitive fields live and how they must be handled (logging/redaction, retention, deletion).
