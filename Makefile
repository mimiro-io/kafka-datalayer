.PHONY: test
build:
	go build -o bin/server cmd/kafkalayer/main.go

run:
	DD_AGENT_HOST=localhost LOG_LEVEL=DEBUG CONFIG_LOCATION=file://resources/default-config.json go run cmd/kafkalayer/main.go

docker:
	docker build . -t kafka-datalayer

test:
	go vet ./...
	go test ./... -v

integration:
	go test ./... -v -tags=integration
