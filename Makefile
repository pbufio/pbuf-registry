API_PATH=api/v1

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
