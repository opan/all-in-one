run-backend:
	@go run cmd/all-in-one/main.go server

db-seed:
	@go run cmd/all-in-one/main.go db:seed

gen-swagger:
	@swag init -g cmd/all-in-one/main.go -o docs --parseDependency --parseInternal

gen-mocks:
	@echo "Generating mocks..."
	@mockery
	@echo "Mocks generated successfully in internal/authnz/handler/mocks/"

test:
	@go test -v ./...

test-handler:
	@go test -v ./internal/authnz/handler/...

.PHONY: run-backend db-seed gen-swagger gen-mocks test test-handler
