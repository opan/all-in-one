run-backend:
	go run cmd/all-in-one/main.go server

db-seed:
	go run cmd/all-in-one/main.go db:seed

gen-swagger:
# 	swag init -g cmd/all-in-one/main.go -d ./ -o docs --parseInternal --exclude vendor
	swag init -g cmd/all-in-one/main.go -o docs --parseDependency --parseInternal
