.PHONY: dev build test docker clean install gen-key install-dependencies

# Development
dev:
	go run ./cmd/server

# Build
build:
	go build -o bin/server ./cmd/server

# Test
test:
	go test ./...

# Docker
docker:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

# Clean
clean:
	rm -rf bin

# Install dependencies
install-dependencies:
	go mod download

# Generate encryption key
gen-key:
	@openssl rand -base64 32

help:
	@echo "Usage: make <target>"
	@echo "Targets:"
	@echo "  dev - Run the development server"
	@echo "  build - Build the binary"
	@echo "  test - Run the tests"
	@echo "  docker - Build the Docker image"
	@echo "  docker-up - Start the Docker container"
	@echo "  docker-down - Stop the Docker container"
	@echo "  docker-logs - View the Docker container logs"
	@echo "  clean - Remove the binary"
	@echo "  install-dependencies - Install the dependencies"
	@echo "  gen-key - Generate the encryption key"
	@echo "  help - Show this help message"
