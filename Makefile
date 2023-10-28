GO               = go

.PHONY: fmt
fmt:
	@$(GO) fmt ./...

.PHONY: lint
lint:
	@$(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.1 run --timeout 5m0s ./...