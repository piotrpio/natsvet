GO ?= go
BIN := bin/natsvet

.PHONY: build test lint generate corpus plugin testdata-deps

build:
	$(GO) build -o $(BIN) ./cmd/natsvet

testdata-deps:
	cd testdata && $(GO) mod download

test: testdata-deps
	$(GO) test ./...

lint:
	@out=$$(gofmt -l . 2>/dev/null); if [ -n "$$out" ]; then echo "gofmt:"; echo "$$out"; exit 1; fi
	$(GO) vet ./...
	staticcheck ./...
	misspell -locale US -error .
	@scripts/check-license.sh

generate:
	$(GO) generate ./...

corpus: build
	scripts/corpus.sh $(BIN)

# CUSTOM_GCL=<path> checks an already built custom-gcl instead of building one.
plugin: build
	scripts/plugin.sh $(BIN) $(CUSTOM_GCL)
