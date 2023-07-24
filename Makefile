build:
	go build

test:
	go test -v -cover ./...

lint:
	golangci-lint run
