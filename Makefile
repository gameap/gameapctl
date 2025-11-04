GO               = go

.PHONY: fmt
fmt:
	@$(GO) fmt ./...

.PHONY: lint
lint:
	golangci-lint run --timeout 5m0s ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix --timeout 5m0s ./...