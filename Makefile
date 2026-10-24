# frpdesk: `make` builds the UI and the binary for this machine; `make release` the tarballs for
# linux amd64/arm64 and darwin arm64 into dist/, each with its sha256
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS := -trimpath -buildvcs=false

.PHONY: all ui build release clean test

all: build

ui:
	cd ui && npm ci --no-fund --no-audit && npm run build

build: ui
	CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o frpdesk ./cmd/frpdesk

test:
	go vet ./... && go test ./...

release: ui
	rm -rf dist && mkdir -p dist
	for t in linux/amd64 linux/arm64 darwin/arm64; do \
	  os=$${t%/*}; arch=$${t#*/}; d=dist/frpdesk-$(VERSION)-$$os-$$arch; mkdir -p $$d; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $$d/frpdesk ./cmd/frpdesk || exit 1; \
	  cp LICENSE README.md frpdesk.example.toml $$d/; \
	  (cd dist && tar --owner=0 --group=0 --numeric-owner -czf frpdesk-$(VERSION)-$$os-$$arch.tar.gz frpdesk-$(VERSION)-$$os-$$arch) ; \
	  rm -rf $$d; \
	done
	cd dist && sha256sum *.tar.gz > sha256sums.txt && cat sha256sums.txt

clean:
	rm -rf frpdesk dist ui/dist/assets
