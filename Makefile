.PHONY: fmt lint test testacc build install generate

build:
	go build -v -o terraform-provider-archestra

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

codegen-api-client:
	@echo "Fetching and patching OpenAPI spec (fixing numeric exclusiveMinimum for 3.0 compat)..."
	@curl -s http://localhost:9000/openapi.json | \
		python3 scripts/patch_openapi.py > /tmp/archestra-openapi-patched.json
	go tool oapi-codegen -config oapi-config.yaml /tmp/archestra-openapi-patched.json

fmt:
	gofmt -s -w -e .
	terraform fmt -recursive ./examples

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	@echo "Running acceptance tests against remote Archestra environment..."
	@echo "Using ARCHESTRA_BASE_URL: $(ARCHESTRA_BASE_URL)"
	@echo "API key configured: $(shell test -n "$(ARCHESTRA_API_KEY)" && echo "✓ Yes" || echo "✗ No")"
	TF_ACC=1 go test -v -cover -timeout=120m ./...
