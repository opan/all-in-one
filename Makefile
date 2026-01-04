run-backend:
	go run cmd/all-in-one/main.go server

gen-swagger:
	swag init -g cmd/all-in-one/main.go -d ./ -o docs --parseInternal --exclude vendor
