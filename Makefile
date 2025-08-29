GO ?= go

build:
	$(GO) build

clean:
	$(GO) clean -modcache

test:
	$(GO) test -v -cover ./...

lint:
	golangci-lint run

