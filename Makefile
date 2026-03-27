BINARY_NAME=set-intersection
BUILD_DIR=build
CMD_DIR=cmd/set-intersection

.PHONY: build test clean

clean:
	rm -rf $(BUILD_DIR)

build: clean
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

test:
	go test ./...

