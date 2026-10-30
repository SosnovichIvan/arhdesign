#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'EOF'
Usage: bash scripts/validate-adapters.sh <adapter>

Adapters:
  frontend  Run ESLint, TypeScript and Vitest V8 coverage for src/.
  openapi   Lint both contracts, regenerate contact bindings and parse the R1
            account contract with oapi-codegen.
  postgres  Apply migrations through the isolated PostgreSQL integration test.
  api       Run Go static/race checks, PostgreSQL migration validation,
            backend coverage and the OpenAPI adapter.
  all       Run frontend and api adapters.
EOF
}

require_test_database() {
  if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
    echo "TEST_DATABASE_URL is required and must point to an isolated PostgreSQL database whose name contains 'test'." >&2
    exit 2
  fi
}

run_oapi_codegen() {
  if command -v oapi-codegen >/dev/null 2>&1; then
    oapi-codegen "$@"
    return
  fi

  go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.0 "$@"
}

validate_frontend() {
  cd "$project_root"
  npm run lint
  npm run typecheck
  npm run test:coverage
}

validate_openapi() {
  cd "$project_root"
  npm run lint:openapi

  cd "$project_root/apps/api"
  run_oapi_codegen \
    --config oapi-codegen.yaml ../../openapi/contact-api.yaml
  git -C "$project_root" diff --exit-code -- apps/api/internal/api/generated

  local account_output
  account_output="$(mktemp "${TMPDIR:-/tmp}/account-api.XXXXXX")"
  local account_config
  account_config="$(mktemp "${TMPDIR:-/tmp}/account-api-config.XXXXXX")"
  sed "s#^output:.*#output: ${account_output}#" oapi-codegen-account.yaml > "$account_config"
  run_oapi_codegen \
    --config "$account_config" \
    ../../openapi/account-api.yaml
  cmp "$account_output" internal/accountapi/generated/account_api.gen.go
  rm -f "$account_output" "$account_config"
}

validate_postgres() {
  require_test_database
  cd "$project_root/apps/api"
  go test -tags=integration -count=1 ./internal/repository
}

validate_api() {
  cd "$project_root/apps/api"
  go vet ./...
  go test -race -count=1 ./...
  validate_postgres
  bash scripts/check-coverage.sh
  validate_openapi
}

case "${1:-}" in
  frontend)
    validate_frontend
    ;;
  openapi)
    validate_openapi
    ;;
  postgres)
    validate_postgres
    ;;
  api)
    validate_api
    ;;
  all)
    validate_frontend
    validate_api
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
