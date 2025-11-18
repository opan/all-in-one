run-backend:
	go run cmd/listing/main.go server

gen-swagger:
	swag init -g cmd/listing/main.go -o docs --parseInternal
