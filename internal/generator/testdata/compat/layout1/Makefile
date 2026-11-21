.PHONY: run build test vet tidy swag migrate

run:
	go run . serve

build:
	go build -o bin/layout1 .

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

# Regenerates docs/ from the swag comments on main.go and the handlers.
swag:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init

migrate:
	go run . migrate up
