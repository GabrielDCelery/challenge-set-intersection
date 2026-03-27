BINARY_NAME=set-intersection
BUILD_DIR=build
CMD_DIR=cmd/set-intersection
GIT_SHA=$(shell git rev-parse --short HEAD)

.PHONY: build test clean docker-build docker-run

clean:
	rm -rf $(BUILD_DIR)

build: clean
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)

test:
	go test ./...

docker-build:
	docker build -t set-intersection:$(GIT_SHA) -t set-intersection:latest .

docker-run:
	docker run --rm -v $(shell pwd)/data:/data set-intersection:latest
