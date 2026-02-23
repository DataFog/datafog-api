---
title: "Security"
use_when: "Capturing security expectations for datafog-api: threat model, auth/authorization, data sensitivity, compliance, and required controls."
---

## Scope and threat model

This service is an HTTP API that accepts untrusted text payloads and returns structured inspection or redaction decisions.

Assume public internet exposure for all request endpoints unless deployment policy specifies internal networking.

## Assets

- PII and policy-sensitive text submitted for scanning and transformation.
- API token and runtime configuration (`DATAFOG_API_TOKEN`, timeout and path settings).
- Receipt storage (`DATAFOG_RECEIPT_PATH`) containing per-request decisions.
- Container images and Git history.

## Trust boundaries

- Boundary 1: client ↔ datafog-api over HTTP.
- Boundary 2: operator/admin ↔ runtime environment (container image, host, secrets).
- Boundary 3: in-process policy and redaction engine ↔ local filesystem.

## Authentication and authorization

- API token is optional and configured via `DATAFOG_API_TOKEN`.
- When enabled, all non-public endpoints require `Authorization: Bearer <token>` or `X-API-Key: <token>`.
- Keep default deployment policy at least as strict as token-required if endpoint access is not intentionally internal.
- Use per-environment tokens; rotate on incidents and after any suspected leak.

## Input and output handling

- Never trust request bodies; all inputs are validated by request schema and handler-level parsing.
- Error messages must avoid leaking request secrets or raw internal stack traces.
- No secrets should be echoed in responses, logs, or receipts.

## Data handling and retention

- Treat text payloads and findings as confidential.
- `DATAFOG_RECEIPT_PATH` stores action receipts and should be writable only to the minimal directory required by deployment.
- Store receipts with minimal retention where possible; if retention policy is externalized, define purge windows in deployment docs.

## Operational controls

- Do not include secrets in repository history, container args, or logs.
- Use process isolation and least privilege in deployment:
  - `USER 65532` in container image.
  - `readOnlyRootFilesystem`, dropped capabilities, and no privilege escalation in orchestration.
- Enforce transport security (TLS termination before API pods/services if not done inside service).
- Keep rate limit enabled (`DATAFOG_RATE_LIMIT_RPS`) in multi-tenant or public exposures.

## Compliance expectations

- If policy data includes regulated PII categories, classify that data and map obligations before enabling production rollout.
- Maintain a documented decision log for retention/erasure support and policy schema evolution.

## Controls and hardening checks

- Dependency and static security checks are planned for the next hardening phase:
  - `gosec` for common Go security smells.
  - `govulncheck` for module vulnerability visibility.
- During MVP, include explicit operational controls (rate limiting, token auth, least-privileged container runtime, and secret hygiene) and treat the above as Phase 2 hardening.

## Incident response baseline

If compromise or data exposure is suspected:

1. Revoke `DATAFOG_API_TOKEN` and issue new token.
2. Rotate any service credentials and block traffic by policy as needed.
3. Freeze release of mutated policy files until integrity checks are revalidated.
