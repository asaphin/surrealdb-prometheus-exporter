.PHONY: build run test clean docker-build docker-run docker-run-with-config docker-stop docker-logs docker-push

# Go build variables
BINARY_NAME=exporter

# Docker variables
DOCKER_IMAGE=surrealdb-prometheus-exporter
DOCKER_TAG=latest
DOCKER_CONTAINER=surrealdb-prometheus-exporter
DOCKER_REGISTRY?=asaphin
CONFIG_PATH?=./config.yaml

# Go targets
build:
	go build -o $(BINARY_NAME) ./cmd/exporter

run: build
	./$(BINARY_NAME)

run-with-config: build
	./$(BINARY_NAME) -config.file=./config.yaml

test:
	go test -v ./...

clean:
	rm -f $(BINARY_NAME)

# Docker targets
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f Dockerfile .

docker-build-no-cache:
	docker build --no-cache -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f Dockerfile .

# Run with external config file mounted as volume
docker-run-with-config:
	docker run -d \
		--name $(DOCKER_CONTAINER) \
		-p 9224:9224 \
		-v $(PWD)/config.yaml:/config/config.yaml:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Run with environment variables (no config file needed)
docker-run-env:
	docker run -d \
		--name $(DOCKER_CONTAINER) \
		-p 9224:9224 \
		-e SURREALDB_URI=ws://localhost:8000 \
		-e SURREALDB_USERNAME=root \
		-e SURREALDB_PASSWORD=root \
		$(DOCKER_IMAGE):$(DOCKER_TAG) \
		-config.file=

# Run with custom config path
docker-run-custom-config:
	docker run -d \
		--name $(DOCKER_CONTAINER) \
		-p 9224:9224 \
		-v $(CONFIG_PATH):/config/config.yaml:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Run in foreground for testing
docker-run-fg:
	docker run --rm \
		--name $(DOCKER_CONTAINER) \
		-p 9224:9224 \
		-v $(PWD)/config.yaml:/config/config.yaml:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Stop the container
docker-stop:
	docker stop $(DOCKER_CONTAINER) || true
	docker rm $(DOCKER_CONTAINER) || true

# View container logs
docker-logs:
	docker logs -f $(DOCKER_CONTAINER)

# Push to registry
docker-push:
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_REGISTRY)/$(DOCKER_IMAGE):$(DOCKER_TAG)

# Complete workflow: build, stop old container, and run new one
docker-restart: docker-stop docker-build docker-run-with-config

# Development workflow with logs
docker-dev: docker-stop docker-build docker-run-with-config docker-logs

# Clean up all Docker resources
docker-clean: docker-stop
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG) || true

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

lint-strict:
	golangci-lint run --max-issues-per-linter=0 --max-same-issues=0

lint-strict-fix:
	golangci-lint run --max-issues-per-linter=0 --max-same-issues=0 --fix
