FROM golang:1.22 AS build

WORKDIR /workspace

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY config ./config

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/datafog-api ./cmd/datafog-api
RUN mkdir -p /workspace/var/lib/datafog && chmod 0777 /workspace/var/lib/datafog

FROM gcr.io/distroless/base-debian11

WORKDIR /app
COPY --from=build /out/datafog-api /usr/local/bin/datafog-api
COPY --from=build /workspace/config/policy.json /app/config/policy.json
COPY --from=build /workspace/var/lib/datafog /var/lib/datafog

ENV DATAFOG_POLICY_PATH=/app/config/policy.json
ENV DATAFOG_RECEIPT_PATH=/var/lib/datafog/datafog_receipts.jsonl
ENV DATAFOG_ADDR=:8080

USER 65532:65532

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/datafog-api"]
