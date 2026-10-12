GO ?= go
PREFIX ?= $(HOME)/.local
VERSION ?= 1.0.0-dev
GORELEASER ?= goreleaser
GORELEASER_VERSION := 2.18.3
# Snapshot archives use the release naming path with a version no real tag uses.
SNAPSHOT_VERSION ?= 0.0.0-rc.1

.PHONY: build test smoke check demos install clean notices release-check release-snapshot

build:
	$(GO) build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o bin/projectory ./cmd/projectory

test:
	$(GO) test -race ./...

smoke: build
	python3 scripts/smoke.py

check: test smoke
	$(GO) vet ./...

demos: build
	python3 scripts/demo.py

install: build
	install -d "$(PREFIX)/bin"
	install -m 755 bin/projectory "$(PREFIX)/bin/projectory"
	@printf '\nInstalado en: %s\n\nActiva pt en esta sesión de zsh:\n  eval "$$("%s" init zsh)"\n\nAñade esa línea a ~/.zshrc para activar pt en nuevas terminales.\n' "$(PREFIX)/bin/projectory" "$(PREFIX)/bin/projectory"

clean:
	rm -rf bin

# Release pipeline targets (see .goreleaser.yaml and .github/workflows/release.yml).
notices:
	python3 scripts/third_party_notices.py --output build/THIRD_PARTY_NOTICES.md

release-check:
	@$(GORELEASER) --version 2>/dev/null | grep -Eq '^GitVersion:[[:space:]]+$(GORELEASER_VERSION)$$' || { echo "GoReleaser $(GORELEASER_VERSION) is required (GORELEASER=$(GORELEASER))" >&2; exit 1; }
	$(GORELEASER) check

release-snapshot: release-check
	GORELEASER_CURRENT_TAG=v$(SNAPSHOT_VERSION) $(GORELEASER) release --snapshot --clean
	python3 scripts/check_release_artifacts.py --dist dist --version $(SNAPSHOT_VERSION) --native
