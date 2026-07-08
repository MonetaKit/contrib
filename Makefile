GO ?= go

.PHONY: check fmt vet build test tidy

check: fmt vet build test

fmt:
	@test -z "$$(gofmt -l .)" || (gofmt -l . && echo "run gofmt -w ." && exit 1)

vet:
	$(GO) vet ./...

build:
	$(GO) build ./...

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy
