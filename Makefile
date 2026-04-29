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
	@# Spec is normalized through tools/oapi-patch first to repair two
	@# upstream patterns that oapi-codegen mishandles (inline
	@# collapsing-arm numeric unions and inline mixed-primitive unions).
	@# See tools/oapi-patch/main.go for details; long term, fix the
	@# platform-side zod schemas to emit named/annotated unions and drop
	@# this step.
	@mkdir -p .codegen
	curl -fsS http://localhost:9000/openapi.json -o .codegen/openapi.raw.json
	cd tools && go run ./oapi-patch \
	  -in ../.codegen/openapi.raw.json \
	  -out ../.codegen/openapi.patched.json
	go tool oapi-codegen -config oapi-config.yaml .codegen/openapi.patched.json

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
