BINARY = smartclaw
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS = -ldflags "-s -w -X github.com/instructkr/smartclaw/internal/cli.Version=$(VERSION) -X github.com/instructkr/smartclaw/internal/cli.Commit=$(COMMIT) -X github.com/instructkr/smartclaw/internal/cli.Date=$(DATE)"

.PHONY: build install clean test cross-build dist lint fmt vet dmg exe web web-build

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/smartclaw

install: build
	cp $(BINARY) /usr/local/bin/

clean:
	rm -f $(BINARY) dist/

test:
	go test ./...

cross-build:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)_darwin_amd64 ./cmd/smartclaw
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)_darwin_arm64 ./cmd/smartclaw
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)_linux_amd64 ./cmd/smartclaw
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)_linux_arm64 ./cmd/smartclaw
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)_windows_amd64.exe ./cmd/smartclaw
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)_windows_arm64.exe ./cmd/smartclaw

dist:
	goreleaser release --clean --snapshot

dmg:
	./scripts/build-dmg.sh $(VERSION)

exe:
	./scripts/build-exe.sh $(VERSION)

web:
	go run ./cmd/smartclaw web --port 8080

web-build:
	cd internal/web/static && npm install && npm run build

lint:
	go vet ./...

fmt:
	gofmt -w .

vet:
	go vet ./...
