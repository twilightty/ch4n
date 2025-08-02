# RegProxy Makefile

# Build variables
BINARY_DAEMON=regproxy-daemon
BINARY_CLI=regproxy-cli
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=${VERSION}"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

.PHONY: all build clean test deps install uninstall help

# Default target
all: build

# Build both binaries
build: build-daemon build-cli

# Build daemon
build-daemon:
	@echo "Building daemon..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_DAEMON) main.go

# Build CLI
build-cli:
	@echo "Building CLI..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_CLI) cmd/cli/main.go

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_DAEMON)
	rm -f $(BINARY_CLI)

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Install system-wide (requires sudo)
install: build
	@echo "Installing RegProxy..."
	sudo mkdir -p /opt/regproxy
	sudo cp $(BINARY_DAEMON) /opt/regproxy/
	sudo cp $(BINARY_CLI) /opt/regproxy/
	sudo cp config.yaml /opt/regproxy/config.yaml.example
	sudo cp regproxy.service /etc/systemd/system/
	sudo useradd -r -s /bin/false regproxy || true
	sudo chown -R regproxy:regproxy /opt/regproxy
	sudo systemctl daemon-reload
	@echo "Installation complete!"
	@echo "1. Edit /opt/regproxy/config.yaml with your settings"
	@echo "2. Run: sudo systemctl enable regproxy"
	@echo "3. Run: sudo systemctl start regproxy"

# Uninstall
uninstall:
	@echo "Uninstalling RegProxy..."
	sudo systemctl stop regproxy || true
	sudo systemctl disable regproxy || true
	sudo rm -f /etc/systemd/system/regproxy.service
	sudo rm -rf /opt/regproxy
	sudo userdel regproxy || true
	sudo systemctl daemon-reload
	@echo "Uninstallation complete!"

# Development targets
dev-daemon: build-daemon
	@echo "Running daemon in development mode..."
	./$(BINARY_DAEMON) -config config.yaml

dev-cli: build-cli
	@echo "Running CLI in development mode..."
	./$(BINARY_CLI) -help

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DAEMON)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DAEMON)-linux-arm64 main.go
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DAEMON)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DAEMON)-darwin-arm64 main.go
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_DAEMON)-windows-amd64.exe main.go

# Create release package
package: build-all
	@echo "Creating release packages..."
	mkdir -p releases
	tar -czf releases/regproxy-linux-amd64.tar.gz $(BINARY_DAEMON)-linux-amd64 config.yaml README.md
	tar -czf releases/regproxy-linux-arm64.tar.gz $(BINARY_DAEMON)-linux-arm64 config.yaml README.md
	tar -czf releases/regproxy-darwin-amd64.tar.gz $(BINARY_DAEMON)-darwin-amd64 config.yaml README.md
	tar -czf releases/regproxy-darwin-arm64.tar.gz $(BINARY_DAEMON)-darwin-arm64 config.yaml README.md
	zip -r releases/regproxy-windows-amd64.zip $(BINARY_DAEMON)-windows-amd64.exe config.yaml README.md

# Help
help:
	@echo "RegProxy Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build        - Build both daemon and CLI"
	@echo "  build-daemon - Build only the daemon"
	@echo "  build-cli    - Build only the CLI"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  deps         - Download dependencies"
	@echo "  install      - Install system-wide (requires sudo)"
	@echo "  uninstall    - Uninstall system-wide (requires sudo)"
	@echo "  dev-daemon   - Run daemon in development mode"
	@echo "  dev-cli      - Run CLI in development mode"
	@echo "  build-all    - Build for multiple platforms"
	@echo "  package      - Create release packages"
	@echo "  help         - Show this help"
