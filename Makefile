# Jenkins Flow CLI - Makefile
# Run `make help` to see available targets

BINARY_NAME=jenkins-flow
MAIN_PATH=cmd/jenkins-flow/main.go

.PHONY: all build run clean test deps help

## build-web: Build the Vue frontend
build-web:
	cd web && npm install && npm run build
	rm -rf pkg/server/static/*
	mkdir -p pkg/server/static
	cp -r web/dist/* pkg/server/static/

## build: Build the binary (includes frontend)
build: build-web
	go build -o $(BINARY_NAME) $(MAIN_PATH)

## run: Run the CLI with default configs
run: build
	./$(BINARY_NAME) run -instances instances.yaml -workflow workflow.yaml

## serve: Run the dashboard server
serve: build
	./$(BINARY_NAME) serve

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
