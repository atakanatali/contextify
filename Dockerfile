FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum* ./
RUN go mod download 2>/dev/null || true

COPY . .
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /contextify ./cmd/server

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /contextify /usr/local/bin/contextify
COPY internal/db/migrations /migrations

EXPOSE 8420

ENTRYPOINT ["contextify"]
