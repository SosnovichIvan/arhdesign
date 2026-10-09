#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
  echo "TEST_DATABASE_URL is required for backend coverage: the repository integration test must run." >&2
  exit 1
fi

# Generated OpenAPI bindings have no authored control flow. The composition root
# is exercised by container health checks; unit coverage applies to the authored
# domain packages below, including the PostgreSQL integration package.
coverage_packages="$(go list ./internal/... | grep -v '/generated$' | paste -sd, -)"
coverage_raw_profile="${TMPDIR:-/tmp}/arhdesign-api-coverage-raw.out"
coverage_profile="${TMPDIR:-/tmp}/arhdesign-api-coverage.out"

go test -tags=integration -covermode=atomic -coverpkg="$coverage_packages" -coverprofile="$coverage_raw_profile" ./internal/...

# `go test ./internal/... -coverpkg=...` emits the same instrumented source block
# once per test binary. `go tool cover` treats those duplicate rows as separate
# statements, which dilutes cross-package coverage (for example handler tests are
# counted once as executed and again as zero for every unrelated package). Merge
# identical blocks before measuring the authored code as one program.
awk '
  NR == 1 { mode = $0; next }
  {
    key = $1 " " $2
    counts[key] += $3
  }
  END {
    print mode
    for (key in counts) print key, counts[key]
  }
' "$coverage_raw_profile" | LC_ALL=C sort -k1,1 > "${coverage_profile}.sorted"
{
  printf 'mode: atomic\n'
  grep -v '^mode:' "${coverage_profile}.sorted"
} > "$coverage_profile"
rm -f "${coverage_profile}.sorted"
coverage="$(go tool cover -func="$coverage_profile" | awk '/^total:/ { sub(/%$/, "", $3); print $3 }')"

awk -v coverage="$coverage" 'BEGIN {
  if (coverage + 0 < 90) {
    printf "Go coverage %.1f%% is below the required 90%%\n", coverage + 0 > "/dev/stderr"
    exit 1
  }
  printf "Go coverage %.1f%% meets the required 90%%\n", coverage + 0
}'
