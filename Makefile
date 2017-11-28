sources=$(shell find . -maxdepth 1 -name '*.go' -not -name '*_test.go')
branch=$(shell git rev-parse --abbrev-ref HEAD)

all: $(branch)

.PHONY: $(branch)
$(branch):
	go build -o $@ $(sources)

run:
	go run $(sources)

test:
	go test -v ./...
