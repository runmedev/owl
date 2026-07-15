SHELL := /bin/bash
LOCAL := github.com/runmedev/owl

.PHONY: build
build:
	go test -run=^$$ ./...

.PHONY: test
test:
	go test ./...

.PHONY: fmt
fmt:
	@go tool gofumpt -w .
	@go tool goimports -local="$(LOCAL)" -w -l .

.PHONY: lint
lint:
	@# "gofumpt -d ." does not return non-zero exit code if there are changes
	test -z "$$(git ls-files '*.go' | xargs go tool gofumpt -d)"
	@# "goimports -d ." does not return non-zero exit code if there are changes
	test -z $(shell go tool goimports -local="$(LOCAL)" -l .)
	go tool revive \
		-config revive.toml \
		-formatter friendly \
		./...
	go tool staticcheck ./...
	go tool gosec -quiet -exclude=G110,G115,G204,G304,G404 -exclude-generated ./...
	go vet ./...
	go vet -vettool=$(shell go env GOPATH)/bin/checklocks ./...

.PHONY: check
check: fmt lint test

.PHONY: install/dev
install/dev:
	@# go vet -vettool expects a binary path.
	GOTOOLCHAIN=go1.26.3 go install gvisor.dev/gvisor/tools/checklocks/cmd/checklocks@go
