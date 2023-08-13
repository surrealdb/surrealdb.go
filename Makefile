build:
	go build

clean:
	go clean -modcache

test:
	go test -v -cover ./...

lint:
	golangci-lint run

