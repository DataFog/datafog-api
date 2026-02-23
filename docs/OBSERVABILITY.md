---
title: "Observability"
use_when: "Documenting logging, metrics, tracing, and health check conventions for this repo, including how agents can access signals to self-verify behavior."
---

## Logging Strategy
- Prefer structured logs with consistent fields (service, env, request_id/trace_id, user_id when safe).
- Never log secrets; be deliberate about PII.
- Log at boundaries and on errors; avoid noisy per-loop logging in hot paths.

## Metrics
- Track the golden signals: latency, traffic, errors, saturation.
- Prefer histograms for latency; keep label cardinality low.

## Traces
- Propagate trace context across service boundaries.
- Trace the critical paths (requests, background jobs) with stable span names.

## Health Checks
- Health checks are fast and deterministic; readiness reflects dependency availability when needed.
- Document expected status codes and what "unhealthy" means operationally.

## Agent Access
- Provide at least one concrete way to query each signal (logs, metrics, traces) without tribal knowledge.
- Include 1-2 copy-pastable examples per signal once the stack is known (commands, URLs, or queries).
