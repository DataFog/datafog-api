---
title: "Reliability"
use_when: "Capturing reliability goals, failure modes, monitoring, and operational guardrails for this repo."
---

## Reliability goals (MVP)

- Primary flow (`POST /v1/scan`): 99.9% availability, p95 latency below 250ms at steady load.
- Policy and redaction consistency (`POST /v1/decide`, `POST /v1/transform`, `POST /v1/anonymize`): 99.5% availability, p95 latency below 350ms.
- Health signal (`GET /health`): 99.99% availability for readiness/liveness checks.

Definition of degraded:

- Availability below target for 5-minute windows.
- p95 latency sustained > 1.5x target for 10 minutes.
- Error rate > 1% for any public endpoint.

## Failure Modes

Top failures and controls:

- Policy file missing or invalid JSON:
  - Signal: `policy_load_failed_total` increases, `/health` may degrade.
  - Blast radius: all scan/decide/transform calls fail.
  - Recovery: roll back to last known-good `config/policy.json`, fix schema, redeploy.

- Receipt path write failure:
  - Signal: request-level `receipt_write_failed` metric spikes, partial request successes.
  - Blast radius: observability of decisions degrades first; policy logic still runs.
  - Recovery: fix filesystem permissions, point to healthy `DATAFOG_RECEIPT_PATH`, restart.

- Rate limit configuration too low or malformed:
  - Signal: sudden `429` rise and client-side retries.
  - Blast radius: throughput reduction for bursty clients.
  - Recovery: validate and tune `DATAFOG_RATE_LIMIT_RPS`, deploy config change.

- Bad deployment image or env drift:
  - Signal: crash/restart loop, increased non-2xx responses.
  - Blast radius: endpoint unavailability.
  - Recovery: rollback image/version and redeploy after diff review.

## Monitoring

Minimum signal set:

- Error rate by endpoint and status code.
- p95/p99 latency per endpoint.
- `/health` pass/fail and startup duration.
- `DATAFOG_RATE_LIMIT_RPS` rejections.
- Receipt persistence success rate.

Alert rules:

- Page if SLO burn reaches 10% remaining over 10 minutes.
- Page on crash loop, persistent readiness failure, or error budget burn above threshold.
- Warn on sustained latency regression above 2x target for two consecutive intervals.

## Operational Guardrails

- Keep configuration centralized and immutable per release (`policy`, env vars, receipt path).
- Every change must include a verified rollback command or known Git point-in-time for the container image and config map.
- Prefer controlled rollout with canaries for policy schema changes and rate-limit changes.
