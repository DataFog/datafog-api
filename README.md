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
- `DATAFOG_API_TOKEN`: optional API token for endpoint protection
- `DATAFOG_RATE_LIMIT_RPS`: `0` (disabled, else max requests per second)
- `DATAFOG_READ_TIMEOUT`: `5s`
- `DATAFOG_WRITE_TIMEOUT`: `10s`
- `DATAFOG_READ_HEADER_TIMEOUT`: `2s`
- `DATAFOG_IDLE_TIMEOUT`: `30s`
- `DATAFOG_SHUTDOWN_TIMEOUT`: `10s`

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

## Deployment

The service is deployed as a single stateless binary with optional mounted policy and receipt storage.

### Local/container quick start

```sh
docker build -t datafog-api:v2 .
docker run --rm -p 8080:8080 \
  -e DATAFOG_API_TOKEN=changeme \
  -e DATAFOG_RATE_LIMIT_RPS=50 \
  -e DATAFOG_RECEIPT_PATH=/var/lib/datafog/datafog_receipts.jsonl \
  -v $(pwd)/config:/app/config:ro \
  -v datafog-receipts:/var/lib/datafog \
  datafog-api:v2
```

### Kubernetes-style production pattern

Use `/health` for liveness/readiness checks and mount writable storage for receipts.

## Enforcement shim (runtime gate)

`datafog-api` is a policy decision service. For runtime enforcement, use the optional shim:

```sh
go build -o datafog-shim ./cmd/datafog-shim

./datafog-shim shell --policy-url http://localhost:8080 rm -rf /tmp/test
```

The shim calls `/v1/decide` before side-effect actions and only permits actions that resolve to:

- `allow`
- `allow_with_redaction`

Actions that resolve to `transform` or `deny` are blocked until the caller applies an explicit transformation path.

Supported actions:

- `shell` (command + args)
- `read-file <path>`
- `write-file <path> <text>`

Decision receipts are returned in stderr for every executed action.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: datafog-api
spec:
  replicas: 1
  selector:
    matchLabels:
      app: datafog-api
  template:
    metadata:
      labels:
        app: datafog-api
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532
      containers:
      - name: datafog-api
        image: ghcr.io/datafog/datafog-api:v2
        ports:
        - containerPort: 8080
        env:
        - name: DATAFOG_ADDR
          value: ":8080"
        - name: DATAFOG_POLICY_PATH
          value: "/app/config/policy.json"
        - name: DATAFOG_RECEIPT_PATH
          value: "/var/lib/datafog/datafog_receipts.jsonl"
        - name: DATAFOG_RATE_LIMIT_RPS
          value: "100"
        volumeMounts:
        - name: policy
          mountPath: /app/config
          readOnly: true
        - name: receipts
          mountPath: /var/lib/datafog
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop: ["ALL"]
      volumes:
      - name: policy
        configMap:
          name: datafog-policy
      - name: receipts
        persistentVolumeClaim:
          claimName: datafog-receipts
```
