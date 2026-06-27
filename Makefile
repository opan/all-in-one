LOAD_ENV := if [ -f env.dev ]; then set -a; . ./env.dev; set +a; fi

run-backend:
	@$(LOAD_ENV); go run cmd/all-in-one/main.go server

db-migrate-up:
	@$(LOAD_ENV); go run cmd/all-in-one/main.go db:migrate up

db-migrate-down:
	@$(LOAD_ENV); go run cmd/all-in-one/main.go db:migrate down

db-seed:
	@$(LOAD_ENV); go run cmd/all-in-one/main.go db:seed

db-transfer:
	@$(LOAD_ENV); go run cmd/all-in-one/main.go db:transfer $(ARGS)

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

.PHONY: run-backend db-migrate-up db-migrate-down db-seed db-transfer gen-swagger gen-proto gen-mocks test test-handler
