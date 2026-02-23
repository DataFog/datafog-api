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

## HTTP API

All examples use `localhost:8080`.

### `GET /health`

```sh
curl http://localhost:8080/health
```

### `POST /v1/policy/version`

```sh
curl http://localhost:8080/v1/policy/version
```

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

