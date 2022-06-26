.PHONY: test vtest deps tidy bench

all:

test: deps
	go test -race ./...

vtest: deps
	go test -race -v ./...

deps:
	go mod download

tidy:
	go mod tidy

bench:
	go test -bench=. -count 1 -run=^# -benchtime=2s ./...
