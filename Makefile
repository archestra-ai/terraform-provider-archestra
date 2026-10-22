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
	if [ -n "$${OPENAPI_SPEC:-}" ]; then cp "$${OPENAPI_SPEC}" .codegen/openapi.raw.json; else curl --retry 3 --retry-all-errors --connect-timeout 10 --max-time 120 -fsS "$${ARCHESTRA_BASE_URL:-http://localhost:9000}/openapi.json" -o .codegen/openapi.raw.json; fi
	cd tools && go run ./oapi-patch \
	  -in ../.codegen/openapi.raw.json \
	  -out ../.codegen/openapi.patched.json
	go tool oapi-codegen -config oapi-config.yaml .codegen/openapi.patched.json

fmt:
	gofmt -s -w -e .
	terraform fmt -recursive ./examples

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

# Checks every example configuration against the provider built from this tree.
validate-examples:
	scripts/validate-examples.sh

testacc:
	@echo "Running acceptance tests against remote Archestra environment..."
	@echo "Using ARCHESTRA_BASE_URL: $(ARCHESTRA_BASE_URL)"
	@echo "API key configured: $(shell test -n "$(ARCHESTRA_API_KEY)" && echo "✓ Yes" || echo "✗ No")"
	TF_ACC=1 go test -v -cover -timeout=120m ./...
