.PHONY: dev build test docker clean install gen-key install-dependencies prettier prettier-check restart logs logs-recent help generate templ

# Development
dev:
	go run ./cmd/server

# Generate templ files
generate:
	~/go/bin/templ generate ./...

templ:
	~/go/bin/templ generate ./...

# Build
build: generate
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

# Frontend formatting (static assets only)
format:
	go fmt -w .
	npx --yes prettier --write "internal/web/**/*.{js,css,html}"

format-check:
	go fmt -w .
	npx --yes prettier --check "internal/web/**/*.{js,css,html}"

# Generate encryption key
gen-key:
	@openssl rand -base64 32

# Service management
restart:
	@echo "Rebuilding and restarting service..."
	@make build
	@sudo systemctl daemon-reload
	@sudo systemctl restart stockmarket
	@sleep 2
	@sudo systemctl status stockmarket --no-pager | head -15

logs:
	@sudo journalctl -u stockmarket -f

logs-recent:
	@sudo journalctl -u stockmarket -n 50 --no-pager

help:
	@echo "Usage: make <target>"
	@echo "Targets:"
	@echo "  dev - Run the development server"
	@echo "  build - Build the binary"
	@echo "  generate - Generate templ files"
	@echo "  test - Run the tests"
	@echo "  docker - Build the Docker image"
	@echo "  docker-up - Start the Docker container"
	@echo "  docker-down - Stop the Docker container"
	@echo "  docker-logs - View the Docker container logs"
	@echo "  clean - Remove the binary"
	@echo "  install-dependencies - Install the dependencies"
	@echo "  format - Format frontend static assets"
	@echo "  format-check - Check frontend formatting"
	@echo "  gen-key - Generate the encryption key"
	@echo "  restart - Rebuild, restart service, and show status"
	@echo "  logs - Follow service logs (real-time)"
	@echo "  logs-recent - Show last 50 log lines"
	@echo "  help - Show this help message"
