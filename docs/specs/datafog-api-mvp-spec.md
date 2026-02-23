---
slug: datafog-api-mvp
plan_mode: execution
status: active
owner: sidmohan
created: 2026-02-23
---

# datafog-api Go MVP Specification

## Purpose
Datafog API v2 will be a single Go service that owns policy decisioning and privacy transformations for agent action execution. It provides a stable API (`/v1/scan`, `/v1/decide`, `/v1/transform`, `/v1/anonymize`, `/v1/receipts/{id}`, `/v1/policy/version`, `/health`) and deterministic receipts for every side-effect decision.

## Scope
- In scope
  - Canonical policy schema, evaluator, and evaluation outcomes
  - Canonical entity definitions and confidence-scored detection
  - Request-scoped redaction/anonymization transforms
  - Receipt/audit trail with traceability IDs
  - Health/version endpoints for operational integration
  - Golden tests + reproducible behavior tests
- Out of scope
  - Full enterprise UI
  - Native Python/TS client SDKs in this MVP release
  - Multi-tenant authN/Z UI flows

## User-visible behavior
- Agents and services call `/v1/scan` to get entity findings.
- Agents and services call `/v1/decide` before any side-effect action.
- `decide` returns one of `allow`, `allow_with_redaction`, `transform`, or `deny` and always includes `policy_version`, `receipt_id`, and rule trace.
- If evaluator cannot make a safe decision, response defaults to `deny` with `reason` explaining failure.
- `/v1/transform` and `/v1/anonymize` return sanitized payloads using policy-bound modes.
- `/v1/receipts/{id}` returns immutable decision/transform evidence for audit.
- `/health` returns service status, policy version, and startup timestamp.

## Functional requirements

### 1) Action and policy model
- Define canonical request models with stable fields:
  - `action` metadata: `type`, `tool`, `resource`, `command`, `args`, `sensitive`.
  - `context` metadata: `tenant_id`, `actor_id`, `session_id`, `trace_id`, `request_id`.
- Define policy rule model with deterministic matching:
  - `id`, `description`, `priority`, `effect`
  - `match` on `action.type`, optional `resource`, optional `tool`, optional `resource_prefix`, optional `commands`, and optional `args` (list of required args).
  - `entity_requirements` for findings-driven gating
  - `require_sensitive_only` to scope a rule to sensitive-marked actions
  - `transform` list when effect is `transform`.
- Priority and conflict resolution:
  - evaluate all matching rules, then apply deterministic precedence: `deny` first, then `transform`, then `allow_with_redaction`, then `allow`.
  - unknown or empty action types fail closed to `deny`.

### 2) Canonical detectors
- Add built-in entity detectors for MVP:
  - `email`, `phone`, `ssn`, `api_key`, `credit_card`.
- All detections are deterministic with `[start, end)` offsets and confidence score.
- Detectors must never panic on empty or malformed UTF-8 strings.

### 3) Decision endpoint (`/v1/decide`)
- `POST /v1/decide`
- Request pipeline:
  1. validate input
  2. run scan
  3. evaluate policy
  4. generate `receipt_id`
  5. persist receipt
- Decision response includes:
  - `decision`
  - `policy_version`
  - `receipt_id`
  - `matched_rules`
  - `findings`
  - `transform_plan` if required

### 4) Transform endpoints
- `POST /v1/transform` applies masking or redaction strategies.
- `POST /v1/anonymize` applies irreversible pseudonymization for configured fields.
- Both include summary with operation counts and changed span count.

### 5) Receipt and audit
- Persist receipts in-process via file-backed append log `datafog_receipts.jsonl`.
- Receipt includes: action hash, input hash, policy id/version, decision, rule ids, timestamps, and optional sanitized summary.
- Receipts are immutable by API contract (write-once append only).

### 6) Versioning and rollout
- Return policy version from a pinned local policy snapshot file.
- API returns `policy_version` in all relevant responses.

### 7) Error model
- Standard JSON error object with `code`, `message`, `request_id`, and `details`.
- Invalid payloads return 400; internal failures return 500.

## Non-functional requirements
- Deterministic behavior for same request + same policy snapshot.
- Deterministic idempotent replays:
  - repeatable responses for the same `idempotency_key` + request content.
  - `409 conflict` with `idempotency_conflict` when the key is reused with different payload.
- Receipts must include immutable integrity metadata:
  - `action_hash` for the action object.
  - `input_hash` for action+context sans transport and request metadata.
  - `sanitized_summary` for transform decisions when transform plan is applied.
- Latency targets for local path: p95 under 200ms for `decide` and `transform` on moderate text payloads.
- Configurable policy file path and store path via environment variables.
- Basic structured logs with no raw payload or secret material.

## Production hardening addendum

### Immediate production-readiness requirements

- Add deterministic graceful shutdown behavior.
  - Handle `SIGINT` and `SIGTERM`.
  - Drain in-flight requests within a bounded timeout before process exit.
  - Provide fallback forced close if graceful shutdown fails.
- Add service-readiness and service-liveness boundaries.
  - Keep `/health` as liveness baseline.
  - Add readiness semantics if deployment requires startup dependency checks.
- Add transport hardening defaults.
  - Enforce request timeout, read-header timeout, write timeout, idle timeout, and header limits.
- Add operational controls for bounded resource use.
  - Limit body sizes at transport boundaries.
  - Cap idempotency cache/map growth and lifecycle.
  - **Done (v2):** Added optional service-wide request rate limiting via `DATAFOG_RATE_LIMIT_RPS` (0=disabled), returning `429 rate_limited`.

### Additional production readiness work before release

- Add authN/Z at the edge (or strict allowlist + service mesh policy), with explicit deny-by-default.
  - **Done (v2):** API token enforcement via `DATAFOG_API_TOKEN` (Bearer or `X-API-Key`) and `401 unauthorized` response code.
- Add metrics quality improvements:
  - Per-endpoint latency distributions, saturation/error-rate alarms.
  - Route-level and code-path attribution for policy/transform load.
- Add security hardening checks in CI:
  - Dependency vulnerability scanning.
  - Dependency/license/supply-chain policy checks.
- Add operational guardrails:
  - Non-root container execution and read-only root filesystem.
  - Explicit `DATAFOG_*` config schema docs and examples.
- Add resiliency tests:
  - Slow client / partial body / invalid content-type / malformed UTF-8 / interrupted shutdown scenarios.

## Task list for MVP implementation

1. Repo hardening
   - Move to `go.mod` module layout.
   - Remove Python runtime entrypoints.
   - Add build/test config and Go Dockerfile.

2. Core domain and policy contract
   - Add request/response structs for scan/decide/transform/anonymize/receipt.
   - Add canonical `Decision`, `Receipt`, `Rule`, `Entity` models.

3. Detector engine
   - Implement deterministic regex detectors.
   - Add tests for email/phone/SSN/API-key/credit-card.

4. Policy evaluator
   - Implement rule model and precedence logic.
   - Add tests for deny, transform, allow, allow_with_redaction.

5. Runtime and endpoints
   - Implement HTTP router and handlers for 6 endpoints.
   - Add request/validation and error handling tests.

6. Receipt store
   - Implement append-only receipt writer and read API.
   - Add retrieval tests and error path for missing receipts.

7. Integration acceptance
   - Add end-to-end test for `decide` + `transform` + `/v1/receipts/{id}`.
   - Confirm startup docs and API examples.
   - Add coverage for idempotency conflict and replay behavior.
