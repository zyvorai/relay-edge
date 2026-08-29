# Copyright 2026 Zyvor AI Labs
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.23-bookworm AS builder
ARG VERSION=dev
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /relay-edge ./cmd/relay-edge

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /relay-edge /usr/local/bin/relay-edge
RUN mkdir -p /var/lib/relay-edge/tls /var/lib/relay-edge/data && chown -R 65532:65532 /var/lib/relay-edge
EXPOSE 18086
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/relay-edge"]
