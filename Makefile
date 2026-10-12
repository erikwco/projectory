GO ?= go
PREFIX ?= $(HOME)/.local
VERSION ?= 0.1.0-dev

.PHONY: build test smoke check demos install clean

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
