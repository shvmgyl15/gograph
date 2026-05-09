BINARY=gograph
BUILD_DIR=bin
CMD=./cmd/gograph

.PHONY: build test run-build clean bump-patch bump-minor bump-major install

build: bump-patch
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)
	@echo "Built $(BUILD_DIR)/$(BINARY)"

install: build
	sudo rm -f /usr/local/bin/$(BINARY)
	sudo cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/
	@echo "Installed $(BINARY) to /usr/local/bin/"

test:
	go test ./...

run-build:
	go run $(CMD) build .

clean:
	rm -rf $(BUILD_DIR)

bump-patch:
	bump2version patch --allow-dirty

bump-minor:
	bump2version minor --allow-dirty

bump-major:
	bump2version major --allow-dirty
