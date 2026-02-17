.PHONY: build test clean docker-build docker-up docker-down test-e2e

build:
	@echo "Building Mini-KV..."
	@mkdir -p bin
	cd server && go mod download && CGO_ENABLED=0 go build -o ../bin/server .
	cd cli && go mod download && CGO_ENABLED=0 go build -o ../bin/cli .

test:
	@echo "Running unit tests..."
	cd server && go test ./...
	cd cli && go test ./...

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

test-e2e: docker-build
	@echo "Running end-to-end tests..."
	docker compose up --abort-on-container-exit mini-kv-test

clean:
	rm -rf bin
	docker compose down --remove-orphans
