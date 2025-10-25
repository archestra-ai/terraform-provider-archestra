.PHONY: fmt lint test testacc build install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

codegen-api-client:
	go tool oapi-codegen -config oapi-config.yaml http://localhost:9000/openapi.json

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...
