---
title: "Reliability"
use_when: "Capturing reliability goals, failure modes, monitoring, and operational guardrails for this repo."
---

## Reliability Goals
- Define 1-3 critical user flows and their SLOs (availability and latency), plus what "degraded" means.
- Document the steady-state load expectations and the worst-case burst assumptions.

## Failure Modes
- Enumerate the top failure modes (dependency down, timeouts, bad deploy, data/backfill issues, config mistakes).
- For each, record: detection signal, blast radius, and the fastest safe rollback/recovery.

## Monitoring
- Alert on user-impacting symptoms (SLO burn, error rates, latency), not internal noise.
- Ensure every service has a clear health story (liveness/readiness where applicable).

## Operational Guardrails
- Every change has a rollback path (revert, flag off, config rollback) and a verification step.
- Prefer progressive delivery for risky changes (feature flags, canaries, staged rollouts).
