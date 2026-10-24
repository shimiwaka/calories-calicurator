GO ?= $(shell go env GOPATH)/bin/go1.18.10

.PHONY: build dev

build:
	GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o api/api.cgi ./src

dev:
	$(GO) run ./src serve
