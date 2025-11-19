run-backend:
	go run cmd/listing/main.go server

gen-swagger:
	swag init -d cmd/listing -o docs --parseInternal --exclude vendor
