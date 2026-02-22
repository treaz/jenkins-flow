# Jenkins Flow CLI - Makefile
# Run `make help` to see available targets

BINARY_NAME=jenkins-flow
MAIN_PATH=cmd/jenkins-flow/main.go

.PHONY: all build run clean test deps help serve stop-server mock-jenkins

## build-web: Build the Vue frontend
build-web:
	cd web && npm install && npm run build
	rm -rf pkg/server/static/*
	mkdir -p pkg/server/static
	cp -r web/dist/* pkg/server/static/

## generate-api: Generate Go server code from OpenAPI spec
generate-api:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	mkdir -p pkg/api
	oapi-codegen -config api/config.yaml api/openapi.yaml


## build: Build the binary (includes frontend)
build: build-web
	go build -o $(BINARY_NAME) $(MAIN_PATH)

## run: Run the server (alias for serve)
run: serve

## serve: Run the dashboard server
serve: build
	./$(BINARY_NAME)

## stop-server: Stop the dashboard server
stop-server:
	@if pgrep -f "$(BINARY_NAME)" >/dev/null; then \
		pkill -f "$(BINARY_NAME)"; \
		echo "Stopped $(BINARY_NAME) process."; \
	else \
		echo "No $(BINARY_NAME) process found."; \
	fi

## mock-jenkins: Run a local mock Jenkins server for smoke testing (port 9090)
mock-jenkins:
	go run ./cmd/mock-jenkins

## deps: Download and tidy dependencies
deps:
	go mod tidy
	go mod download

## test: Run all tests
test:
	go test -v ./...

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -rf pkg/server/static/*
	go clean

## fmt: Format Go source files
fmt:
	go fmt ./...

## vet: Run go vet on source files
vet:
	go vet ./...

## lint: Run fmt and vet
lint: fmt vet

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/ /'
