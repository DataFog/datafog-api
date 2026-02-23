# Tech Debt Tracker

General-purpose deferred-work queue. Review findings, cleanup tasks, improvement ideas — anything we want to address later but shouldn't block now. Any skill can append to this file.

Treat this file as append-and-update: do not delete historical rows unless duplicated by mistake. When status changes, update both the index table row and the detail entry.

## Status Semantics

- `new`: captured, not yet scheduled.
- `queued`: prioritized for a future slug.
- `in_progress`: being addressed in an active plan.
- `resolved`: fixed, evidence linked.
- `wont_fix`: consciously accepted with documented rationale.

## Index

| ID | Date | Priority | Source | Status | Summary |
|---|---|---|---|---|---|

## Detail Entries

<!-- Append new entries below this line. -->
