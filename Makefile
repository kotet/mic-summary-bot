.PHONY: all test clean

all: examples-bot

examples-bot:
	go build -o examples-bot examples/main.go

test:
	go test ./...

clean:
	rm -f examples-bot
