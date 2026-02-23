- last_updated: 2026-02-23 12:00

# API Schema Snapshot

Discovered from `internal/server/server.go` route registration.

## Routes

- `GET /health`
- `GET /v1/policy/version`
- `POST /v1/scan`
- `POST /v1/decide`
- `POST /v1/transform`
- `POST /v1/anonymize`
- `GET /v1/receipts/{id}`

## Route behavior

- Method validation returns `method_not_allowed` on unsupported methods.
- Unknown routes return `not_found`.
- All successful and error responses are JSON with `Content-Type: application/json`.
