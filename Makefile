GO               = go

.PHONY: fmt
fmt:
	@$(GO) fmt ./...

.PHONY: lint
lint:
	@$(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.58.1 run --timeout 5m0s ./...

.PHONY: lint-fix
lint-fix:
	@$(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.58.1 run --fix --timeout 5m0s ./...