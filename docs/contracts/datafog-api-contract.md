# datafog-api Contract

## Version

- API revision: `v1`
- Service revision tracked via policy file fields: `policy_id`, `policy_version`

## Base URL

- Default local: `http://localhost:8080`
- JSON request and response payloads for all API routes.

## Cross-cutting response model

All responses use JSON and include `Content-Type: application/json`.

`X-Request-ID` is always returned on every response.
- If `x-request-id` is provided by the caller, the same value is reflected in the response header and error `request_id` payload.
- If missing, the service generates a request id and returns it in `X-Request-ID` and error payloads where present.

POST requests require `Content-Type: application/json` (charset may be supplied with standard media type syntax).
A request body larger than 1 MiB (`1048576` bytes) is rejected with `request_too_large`.

If `DATAFOG_API_TOKEN` is configured, every request must include either:

- `Authorization: Bearer <token>`
- `X-API-Key: <token>`

If `DATAFOG_RATE_LIMIT_RPS` is greater than `0`, requests are subject to a service-wide token-bucket request cap with response `429` and `rate_limited` on excess bursts.

### Standard error response

```json
{
  "error": {
    "code": "invalid_request",
    "message": "descriptive error message",
    "request_id": "optional request id propagation",
    "details": "optional low-level details"
  }
}
```

### Standard codes

- `invalid_request` (400)
- `method_not_allowed` (405)
- `not_found` (404)
- `unauthorized` (401)
- `rate_limited` (429)
- `idempotency_conflict` (409)
- `unsupported_media_type` (415)
- `request_too_large` (413)
- `encode_error` (500)
- `hash_error` (500)
- `receipt_error` (500)
- `internal_error` (500)

## Endpoints

### `GET /health`

Returns process health and policy metadata.

```json
{
  "status": "ok",
  "policy_id": "string",
  "policy_version": "string",
  "started_at": "RFC3339 timestamp"
}
```

### `GET /v1/policy/version`

Returns policy identity.

```json
{
  "policy_id": "string",
  "policy_version": "string"
}
```

### `GET /metrics`

Returns coarse-grained service telemetry for operations and routing.

```json
{
  "total_requests": 42,
  "error_requests": 3,
  "by_status": {
    "200": 30,
    "400": 1,
    "404": 2,
    "500": 0
  },
  "by_path": {
    "/health": 20,
    "/v1/scan": 5,
    "/v1/decide": 3,
    "/_not_found": 3
  },
  "by_method": {
    "GET": 14,
    "POST": 28
  },
  "started_at": "RFC3339 timestamp",
  "uptime_seconds": 12.34
}
```

`by_status`, `by_path`, and `by_method` include counters for completed requests observed before each `/metrics` call. `/metrics` request details appear on subsequent polling.

### `POST /v1/scan`

Scans free text and returns deterministic findings.

#### Request

```json
{
  "text": "string (required)",
  "entity_types": ["optional", "list", "of", "entities"],
  "request_id": "optional opaque id",
  "trace_id": "optional correlation id",
  "idempotency_key": "optional key for replay-safe dedupe"
}
```

#### Response 200

```json
{
  "request_id": "string",
  "trace_id": "string",
  "findings": [
    {
      "entity_type": "email|phone|ssn|api_key|credit_card",
      "value": "string",
      "start": 0,
      "end": 5,
      "confidence": 0.0
    }
  ],
  "policy_version": "string",
  "policy_id": "string"
}
```

### `POST /v1/decide`

Evaluates action policy against findings and returns a deterministic decision.

#### Request

```json
{
  "action": {
    "type": "string (required)",
    "tool": "optional",
    "resource": "optional",
    "command": "optional",
    "args": ["optional", "args"],
    "sensitive": false
  },
  "text": "optional input text; if no findings are supplied",
  "findings": [
    {
      "entity_type": "email|phone|ssn|api_key|credit_card",
      "value": "string",
      "start": 0,
      "end": 5,
      "confidence": 0.0
    }
  ],
  "request_id": "optional opaque id",
  "trace_id": "optional correlation id",
  "tenant_id": "optional tenant context",
  "actor_id": "optional actor context",
  "session_id": "optional session context",
  "idempotency_key": "optional key for replay-safe dedupe"
}
```

#### Response 200

```json
{
  "request_id": "string",
  "trace_id": "string",
  "decision": "allow|allow_with_redaction|transform|deny",
  "receipt_id": "string",
  "policy_version": "string",
  "policy_id": "string",
  "matched_rules": ["rule_id"],
  "transform_plan": [
    { "entity_type": "email", "mode": "mask|tokenize|anonymize|redact" }
  ],
  "findings": [
    {
      "entity_type": "email|phone|ssn|api_key|credit_card",
      "value": "string",
      "start": 0,
      "end": 5,
      "confidence": 0.0
    }
  ],
  "reason": "optional reason for deny/fallback"
}
```

### `POST /v1/transform`

Transforms text based on per-entity transforms.

#### Request

```json
{
  "text": "string (required)",
  "findings": [],
  "mode": "mask|tokenize|anonymize|redact",
  "entity_modes": {
    "email": "mask",
    "phone": "tokenize"
  },
  "request_id": "optional opaque id",
  "trace_id": "optional correlation id",
  "idempotency_key": "optional key for replay-safe dedupe"
}

`transform` accepts only the documented modes (`mask`, `tokenize`, `anonymize`, `redact`) in both `mode` and `entity_modes` values.
Invalid transform mode values result in `400` with `code: invalid_request`.
`entity_modes` must not contain empty keys.
```

#### Response 200

```json
{
  "request_id": "string",
  "trace_id": "string",
  "output": "string",
  "policy_id": "string",
  "policy_version": "string",
  "stats": {
    "entities_transformed": 1,
    "modes_applied": "entity:mode,entity:mode"
  }
}
```

### `POST /v1/anonymize`

Applies irreversible anonymization for detected entities.

#### Request

```json
{
  "text": "string (required)",
  "findings": [],
  "request_id": "optional opaque id",
  "trace_id": "optional correlation id",
  "idempotency_key": "optional key for replay-safe dedupe"
}
```

#### Response 200

```json
{
  "request_id": "string",
  "trace_id": "string",
  "output": "string",
  "policy_id": "string",
  "policy_version": "string",
  "stats": {
    "entities_transformed": 1,
    "modes_applied": "email:anonymize"
  }
}
```

### `GET /v1/receipts/{id}`

Returns persisted decision receipts.

```json
{
  "receipt_id": "string",
  "timestamp": "RFC3339 timestamp",
  "request_id": "string",
  "trace_id": "string",
  "tenant_id": "string",
  "actor_id": "string",
  "session_id": "string",
  "policy_version": "string",
  "policy_id": "string",
  "action_hash": "sha256 hex",
  "input_hash": "sha256 hex",
  "sanitized_summary": "optional json string summary",
  "decision": "allow|allow_with_redaction|transform|deny",
  "action": {
    "type": "string",
    "tool": "string",
    "resource": "string",
    "command": "string",
    "args": ["string"],
    "sensitive": false
  },
  "matched_rules": ["rule_id"],
  "findings": [
    {
      "entity_type": "email|phone|ssn|api_key|credit_card",
      "value": "string",
      "start": 0,
      "end": 5,
      "confidence": 0.0
    }
  ],
  "transform_plan": [
    { "entity_type": "email", "mode": "mask|tokenize|anonymize|redact" }
  ],
  "reason": "optional"
}
```

## Idempotency

- Supported endpoints: `POST /v1/scan`, `POST /v1/decide`, `POST /v1/transform`, `POST /v1/anonymize`.
- Replaying the same idempotency key and identical semantic payload returns the same status and body.
- Reusing a key with different payloads returns `409` and `code: idempotency_conflict`.
