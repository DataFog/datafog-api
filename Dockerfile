FROM golang:1.22 AS build

WORKDIR /workspace

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY config ./config

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/datafog-api ./cmd/datafog-api

FROM gcr.io/distroless/base-debian11

WORKDIR /app
COPY --from=build /out/datafog-api /usr/local/bin/datafog-api
COPY --from=build /workspace/config/policy.json /app/config/policy.json

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/datafog-api"]
