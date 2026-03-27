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
	docker build -t infosum/set-intersection:$(GIT_SHA) -t infosum/set-intersection:latest .

docker-run:
	docker run --rm \
		-v $(shell pwd)/data:/data \
		-v $(shell pwd)/config:/config \
		infosum/set-intersection:latest --config /config/default.yaml
