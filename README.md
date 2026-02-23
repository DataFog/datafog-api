# datafog-api (Go)

This repository implements the MVP `datafog-api` service in Go.

## Local development

```sh
go mod download
go test ./...
go run ./cmd/datafog-api
```

Default configuration:

- `DATAFOG_POLICY_PATH`: `config/policy.json`
- `DATAFOG_RECEIPT_PATH`: `datafog_receipts.jsonl`
- `DATAFOG_ADDR`: `:8080`
- `DATAFOG_READ_TIMEOUT`: `5s`
- `DATAFOG_WRITE_TIMEOUT`: `10s`
- `DATAFOG_READ_HEADER_TIMEOUT`: `2s`
- `DATAFOG_IDLE_TIMEOUT`: `30s`

Durations accept Go duration syntax (for example: `1s`, `500ms`, `2m`).

## HTTP API

All examples use `localhost:8080`.

Canonical contract:

- See `docs/contracts/datafog-api-contract.md` for endpoint schemas, error codes, and idempotency semantics.
- See `docs/generated/api-schema.md` for registered routes.

### `GET /health`

```sh
curl http://localhost:8080/health
```

### `GET /v1/policy/version`

```sh
curl http://localhost:8080/v1/policy/version
```

### Idempotency

The following endpoints accept `idempotency_key`:

- `/v1/scan`
- `/v1/decide`
- `/v1/transform`
- `/v1/anonymize`

On repeated requests with the same key:
- identical payload returns the exact same response body and status.
- mismatched payload returns `409` with `code: idempotency_conflict`.

### `POST /v1/scan`

```sh
curl -X POST http://localhost:8080/v1/scan \
  -H "Content-Type: application/json" \
  -d '{"text":"email alice@example.com and card 4111111111111111"}'
```

### `POST /v1/decide`

```sh
curl -X POST http://localhost:8080/v1/decide \
  -H "Content-Type: application/json" \
  -d '{"action":{"type":"file.write","resource":"notes.txt"},"text":"email alice@example.com"}'
```

### `POST /v1/transform`

```sh
curl -X POST http://localhost:8080/v1/transform \
  -H "Content-Type: application/json" \
  -d '{"text":"email alice@example.com", "mode":"mask"}'
```

### `POST /v1/anonymize`

```sh
curl -X POST http://localhost:8080/v1/anonymize \
  -H "Content-Type: application/json" \
  -d '{"text":"email alice@example.com", "findings":[{"entity_type":"email","value":"alice@example.com","start":0,"end":17,"confidence":0.99}]}'
```

### `GET /v1/receipts/{id}`

```sh
curl http://localhost:8080/v1/receipts/<receipt-id>
```

## Tests

```sh
go test ./...
```
