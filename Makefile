BINARY  := releasar
GO      := go
DIST    := dist
STORAGE := storage

.PHONY: build build-dist test test-integration vet lint clean \
        build-darwin-amd64 build-darwin-arm64 \
        build-linux-amd64 build-linux-arm64 \
        build-windows-amd64 build-windows-arm64 \
        build-macos build-linux build-windows \
        licenses-update license-check

build:
	$(GO) build -o $(BINARY) .

build-dist: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64

build-macos: build-darwin-amd64 build-darwin-arm64
build-linux: build-linux-amd64 build-linux-arm64
build-windows: build-windows-amd64 build-windows-arm64

build-darwin-amd64:
	GOOS=darwin  GOARCH=amd64 $(GO) build -o $(DIST)/$(BINARY)-darwin-amd64  .

build-darwin-arm64:
	GOOS=darwin  GOARCH=arm64 $(GO) build -o $(DIST)/$(BINARY)-darwin-arm64  .

build-linux-amd64:
	GOOS=linux   GOARCH=amd64 $(GO) build -o $(DIST)/$(BINARY)-linux-amd64   .

build-linux-arm64:
	GOOS=linux   GOARCH=arm64 $(GO) build -o $(DIST)/$(BINARY)-linux-arm64   .

build-windows-amd64:
	GOOS=windows GOARCH=amd64 $(GO) build -o $(DIST)/$(BINARY)-windows-amd64.exe .

build-windows-arm64:
	GOOS=windows GOARCH=arm64 $(GO) build -o $(DIST)/$(BINARY)-windows-arm64.exe .

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

licenses-update:
	@bash bin/update-licenses.sh

# Requires: go install github.com/google/go-licenses@latest
license-check:
	go-licenses check ./... --ignore github.com/mxstzdev/releasar-cli --allowed_licenses=MIT,Apache-2.0,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0,CC0-1.0
