API_PATH=api/v1
REGISTRY_VERSION?=latest

.PHONY: vendor
# vendor modules
vendor:
	pbuf-cli vendor

.PHONY: vendor-gen
# gen modules
vendor-gen:
	buf generate --template buf.modules.gen.yaml --exclude-path ${API_PATH}

.PHONY: vendor-all
vendor-all:
	make vendor
	make vendor-gen

.PHONY: api
# generate api proto
api:
	buf generate --path ${API_PATH} --exclude-path third_party/google

.PHONY: lint
# lint
lint:
	golangci-lint run -v --timeout 10m

.PHONY: mocks
# generate mocks
mocks:
	mockery --all --dir=internal --output=internal/mocks --case=underscore

.PHONY: test
# tests
test:
	go test -v -cover ./...

.PHONY: build
# build
build:
	mkdir -p bin/ && go build -o ./bin/pbuf-registry ./cmd/...

.PHONY: build-migrations
# build migrations
build-migrations:
	mkdir -p bin/ && go build -o ./bin/pbuf-migrations ./.

.PHONY: build-in-docker
# build in docker
build-in-docker:
	docker run --rm \
      -v ".:/app" \
      -v "./bin:/app/bin" \
      -v "${HOME}/.netrc:/root/.netrc" \
      -w /app \
      golang:1.21 \
      sh -c "CGO_ENABLED=0 GOOS=linux make build && CGO_ENABLED=0 GOOS=linux make build-migrations"

.PHONY: docker
# docker
docker:
	docker build -t registry.digitalocean.com/pbuf/registry:${REGISTRY_VERSION} .

.PHONY: run
# run
run:
	docker-compose -f docker-compose.dev.yml build --no-cache && docker-compose -f docker-compose.dev.yml up --force-recreate -d

.PHONY: stop
# stop
stop:
	docker-compose -f docker-compose.dev.yml down

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
