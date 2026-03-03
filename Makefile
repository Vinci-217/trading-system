.PHONY: all build run test clean proto docker-up docker-down

APP_NAME := stock-trading-system
VERSION := 1.0.0
BUILD_TIME := $(shell date +%Y-%m-%d_%H:%M:%S)
GO_VERSION := $(shell go version | awk '{print $$3}')

all: build

build:
	@echo "Building $(APP_NAME)..."
	@go build -o bin/trading-service ./cmd/trading-service
	@echo "Build complete!"

run:
	@echo "Running $(APP_NAME)..."
	@go run ./cmd/trading-service

test:
	@echo "Running tests..."
	@go test -v ./...

clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean

proto:
	@echo "Generating protobuf files..."
	@protoc --go_out=. --go-grpc_out=. api/proto/*.proto
	@echo "Protobuf files generated!"

docker-up:
	@echo "Starting Docker containers..."
	@docker-compose up -d

docker-down:
	@echo "Stopping Docker containers..."
	@docker-compose down

docker-build:
	@echo "Building Docker image..."
	@docker-compose build

docker-logs:
	@docker-compose logs -f

mysql-cli:
	@docker exec -it stock_trading_mysql mysql -uroot -ppassword stock_trading

redis-cli:
	@docker exec -it stock_trading_redis redis-cli

kafka-topics:
	@docker exec -it stock_trading_kafka kafka-topics.sh --bootstrap-server localhost:9092 --list

deps:
	@go mod download
	@go mod tidy

fmt:
	@go fmt ./...

lint:
	@golangci-lint run ./...

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make build        - Build the application"
	@echo "  make run          - Run the application"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make proto        - Generate protobuf files"
	@echo "  make docker-up    - Start Docker containers"
	@echo "  make docker-down  - Stop Docker containers"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-logs  - Show Docker logs"
	@echo "  make deps         - Download dependencies"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Run linter"
