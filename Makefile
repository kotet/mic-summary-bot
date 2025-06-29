.PHONY: all test clean

all: examples-bot

examples-bot: ./**/*.go
	go build -o examples-bot examples/main.go

format:
	go fmt ./...

lint:
	go vet ./...

test:
	go test -v ./...

test-full:
	go test -v -tags=integration ./...

clean:
	rm -f examples-bot
