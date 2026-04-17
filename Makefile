BINARY_NAME=manul
INSTALL_DIR_LOCAL=$(HOME)/.local/bin
INSTALL_DIR_SYSTEM=/usr/local/bin

.PHONY: all build test clean install install-system uninstall

all: build

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./cmd/manul

test:
	@echo "Running tests..."
	go test ./...

clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)

install: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR_LOCAL)..."
	install -d $(INSTALL_DIR_LOCAL)
	install -m 0755 $(BINARY_NAME) $(INSTALL_DIR_LOCAL)/$(BINARY_NAME)
	@echo "Done. Make sure $(INSTALL_DIR_LOCAL) is in your PATH."

install-system: build
	@echo "Installing $(BINARY_NAME) to $(INSTALL_DIR_SYSTEM)..."
	sudo install -m 0755 $(BINARY_NAME) $(INSTALL_DIR_SYSTEM)/$(BINARY_NAME)
	@echo "Done."

uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	rm -f $(INSTALL_DIR_LOCAL)/$(BINARY_NAME)
	sudo rm -f $(INSTALL_DIR_SYSTEM)/$(BINARY_NAME)
