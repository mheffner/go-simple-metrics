.PHONY: test vtest deps tidy

all:

test: deps
	go test -race ./...

vtest: deps
	go test -race -v ./...

deps:
	go mod download

tidy:
	go mod tidy