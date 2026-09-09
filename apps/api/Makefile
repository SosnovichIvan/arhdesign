.PHONY: generate test test-integration

generate:
	oapi-codegen --config oapi-codegen.yaml ../../openapi/contact-api.yaml

test:
	go test -race -count=1 ./...

test-integration:
	go test -tags=integration -count=1 ./internal/repository

test-coverage:
	bash scripts/check-coverage.sh
