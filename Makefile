VERSION ?= dev
LDFLAGS = -s -w -X main.version=$(VERSION)
RECALL_BENCH_REPORT_PATH ?= artifacts/recall-benchmark-report.json

.PHONY: build-server build-cli build-all clean bench-recall

build-server:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/contextify-server ./cmd/server

build-cli:
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/contextify ./cmd/contextify

build-all: build-server build-cli

clean:
	rm -rf bin/ dist/

bench-recall:
	@echo "Running recall benchmark (requires running server at localhost:8420)..."
	RECALL_BENCH_REPORT_PATH=$(RECALL_BENCH_REPORT_PATH) go test -tags e2e ./tests/e2e -run TestRecallBenchmark -v -count=1

.PHONY: release-cli
release-cli:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/contextify-darwin-amd64 ./cmd/contextify
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/contextify-darwin-arm64 ./cmd/contextify
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/contextify-linux-amd64 ./cmd/contextify
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o dist/contextify-linux-arm64 ./cmd/contextify
