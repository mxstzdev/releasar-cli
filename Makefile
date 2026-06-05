BINARY  := releasar
GO      := go
DIST    := dist
STORAGE := storage

.PHONY: build build-all test vet lint clean \
        build-darwin-amd64 build-darwin-arm64 \
        build-linux-amd64 build-linux-arm64

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

vet:
	$(GO) vet ./...

lint:
	golangci-lint run

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)
	rm -rf $(STORAGE)
