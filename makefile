.PHONY: test, lint, format, build, migrate, run-db, run-cli

test:
	go test ./...

test-coverage:
	go test ./... -coverprofile=c.out
	go tool cover -html=c.out
	rm c.out

lint:
	go vet ./...

format:
	go fmt ./...

build:
	go build -o build/cli-agent cmd/cli/main.go

run-db:
	docker compose up -d

migrate:
	goose up

run-cli: build
	./build/cli-agent

	