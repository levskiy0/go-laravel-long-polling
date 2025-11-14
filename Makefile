.PHONY: build run test clean docker-build docker-run

# Build the application
build:
	go build -o longpoll-server ./cmd/longpoll-server

# Run the application
run:
	go run ./cmd/longpoll-server

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f longpoll-server
	go clean

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build Docker image
docker-build:
	docker build -t go-laravel-long-polling:latest .

# Run Docker container
docker-run:
	docker run --rm -p 8085:8085 --env-file .env go-laravel-long-polling:latest

# Development with live reload (requires air)
dev:
	air
