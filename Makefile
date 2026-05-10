run-backend:
	@go run cmd/all-in-one/main.go server

db-migrate-up:
	@go run cmd/all-in-one/main.go db:migrate up

db-migrate-down:
	@go run cmd/all-in-one/main.go db:migrate down

db-seed:
	@go run cmd/all-in-one/main.go db:seed

gen-swagger:
	@swag init -g cmd/all-in-one/main.go -o docs --parseDependency --parseInternal

gen-proto:
	@echo "Generating proto stubs (Go + TypeScript)..."
	@buf generate
	@echo "Proto generation complete"

gen-mocks:
	@echo "Generating mocks..."
	@mockery
	@echo "Mocks generated successfully"

test:
	@go test -v ./...

test-handler:
	@go test -v ./internal/authnz/handler/...

.PHONY: run-backend db-migrate-up db-migrate-down db-seed gen-swagger gen-proto gen-mocks test test-handler
