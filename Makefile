.PHONY: fmt check-fmt lint vet test

GO_PKGS   := $(shell go list -f {{.Dir}} ./...)

fmt:
	@go list -f {{.Dir}} ./... | xargs -I{} gofmt -w -s {}

lint:
	@golangci-lint run

test:
	@go test -v $(GO_FLAGS) -count=1 $(GO_PKGS)
	@cd tests && go test -v $(GO_FLAGS) -count=1
