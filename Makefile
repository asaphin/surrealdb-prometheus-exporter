.PHONY: build run test clean

BINARY_NAME=exporter
GO=go

build:
	$(GO) build -o $(BINARY_NAME) ./cmd/exporter

run: build
	./$(BINARY_NAME)

run-with-config: build
	./$(BINARY_NAME) -config.file=./config.yaml

test:
	$(GO) test -v ./...

clean:
	rm -f $(BINARY_NAME)

docker-build:
	docker build -t surrealdb-exporter:latest .
