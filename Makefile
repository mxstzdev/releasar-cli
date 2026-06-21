BINARY  := releasar
GO      := go
DIST    := dist
STORAGE := storage

.PHONY: build build-all test test-integration vet lint clean \
        build-darwin-amd64 build-darwin-arm64 \
        build-linux-amd64 build-linux-arm64 \
        licenses license-check

build:
	$(GO) build -o $(BINARY) .

build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64

build-darwin-amd64:
	GOOS=darwin  GOARCH=amd64 $(GO) build -o $(DIST)/$(BINARY)-darwin-amd64  .

build-darwin-arm64:
	GOOS=darwin  GOARCH=arm64 $(GO) build -o $(DIST)/$(BINARY)-darwin-arm64  .

build-linux-amd64:
	GOOS=linux   GOARCH=amd64 $(GO) build -o $(DIST)/$(BINARY)-linux-amd64   .

build-linux-arm64:
	GOOS=linux   GOARCH=arm64 $(GO) build -o $(DIST)/$(BINARY)-linux-arm64   .

test:
	$(GO) test ./...

test-integration:
	$(GO) test -tags integration -v -timeout 5m ./...

vet:
	$(GO) vet ./...

lint:
	golangci-lint run

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
	rm -rf $(STORAGE)

# Requires: go install github.com/google/go-licenses@latest
licenses:
	@go-licenses save ./... --ignore github.com/mxstzdev/releasar-cli --save_path=.licenses/ --force 2>/dev/null
	@{ \
	  find .licenses/ -type f | LC_ALL=C sort | while read f; do \
	    echo "================================================================================"; \
	    echo "$$(basename $$(dirname $$f))/$$(basename $$f)"; \
	    echo "================================================================================"; \
	    cat "$$f"; \
	    echo ""; \
	  done; \
	} > LICENSES
	@rm -rf .licenses/
	@echo "LICENSES file updated."

license-check:
	go-licenses check ./... --ignore github.com/mxstzdev/releasar-cli --allowed_licenses=MIT,Apache-2.0,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0,CC0-1.0
