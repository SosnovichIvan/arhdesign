#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
  echo "TEST_DATABASE_URL is required for backend coverage: the repository integration test must run." >&2
  exit 1
fi

# Generated OpenAPI bindings have no authored control flow. The composition root
# is exercised by container health checks; unit coverage applies to the authored
# domain packages below, including the PostgreSQL integration package.
coverage_packages="$(go list ./internal/... | grep -v '/internal/api/generated' | paste -sd, -)"
coverage_profile="${TMPDIR:-/tmp}/arhdesign-api-coverage.out"

go test -tags=integration -covermode=atomic -coverpkg="$coverage_packages" -coverprofile="$coverage_profile" ./internal/...
coverage="$(go tool cover -func="$coverage_profile" | awk '/^total:/ { sub(/%$/, "", $3); print $3 }')"

awk -v coverage="$coverage" 'BEGIN {
  if (coverage + 0 < 90) {
    printf "Go coverage %.1f%% is below the required 90%%\n", coverage + 0 > "/dev/stderr"
    exit 1
  }
  printf "Go coverage %.1f%% meets the required 90%%\n", coverage + 0
}'
