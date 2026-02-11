FROM node:22-alpine AS web-builder

WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ .
RUN npm run build

FROM golang:1.23-alpine AS builder

ARG VERSION=dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /contextify ./cmd/server

FROM alpine:3.20

LABEL org.opencontainers.image.title="Contextify" \
      org.opencontainers.image.description="Unified AI agent memory system with MCP + REST API + Web UI" \
      org.opencontainers.image.source="https://github.com/atakanatali/contextify" \
      org.opencontainers.image.licenses="MIT"

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /contextify /usr/local/bin/contextify
COPY --from=web-builder /web/dist /usr/share/contextify/web

EXPOSE 8420

ENTRYPOINT ["contextify"]
