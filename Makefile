.PHONY: all test clean

all: examples-bot

examples-bot: ./**/*.go
	go build -o examples-bot examples/main.go

run: examples-bot
	/usr/bin/time -v ./examples-bot

format:
	go fmt ./...

lint:
	go vet ./...

test:
	go test -v ./...

test-full:
	go test -v -tags=integration ./...

test-integration: test-full

clean:
	rm -f examples-bot
