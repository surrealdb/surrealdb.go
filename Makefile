GO ?= go

build:
	$(GO) build

clean:
	$(GO) clean -modcache

test:
	$(GO) test -v -cover -race ./...

lint:
	golangci-lint run

pkgsite:
	$(GO) run golang.org/x/pkgsite/cmd/pkgsite@latest
