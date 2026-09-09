#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
context="$root/.ai/context"
required=(README.md context-index.md project-state.md decision-log.md feedback-log.md)

for file in "${required[@]}"; do
  test -s "$context/$file" || { echo "Missing or empty context file: $file" >&2; exit 1; }
done

if rg -n 'Исходный портрет не изменён|EST\. 2016' "$context"; then
  echo "Found known stale author-media fact in context." >&2
  exit 1
fi

if ! rg -q '^# ' "$context/project-state.md" || ! rg -q '^# ' "$context/decision-log.md" || ! rg -q '^# ' "$context/feedback-log.md"; then
  echo "Context files must have a top-level heading." >&2
  exit 1
fi

echo "Context structure validation passed. Run semantic review against OpenSpec, Figma, OpenAPI and code separately."
