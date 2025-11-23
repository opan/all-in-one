run-backend:
	go run cmd/listing/main.go server

gen-swagger:
	swag init -g cmd/listing/main.go -d ./ -o docs --parseInternal --exclude vendor
