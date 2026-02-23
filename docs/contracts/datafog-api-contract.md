# datafog-api Contract

## Version

- API revision: `v1`
- Service revision tracked via policy file fields: `policy_id`, `policy_version`

## Base URL

- Default local: `http://localhost:8080`
- JSON request and response payloads for all API routes.

## Cross-cutting response model

All responses use JSON and include `Content-Type: application/json`.

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
- `idempotency_conflict` (409)
- `encode_error` (500)
- `hash_error` (500)
- `receipt_error` (500)

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
