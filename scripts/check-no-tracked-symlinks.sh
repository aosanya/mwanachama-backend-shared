#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

links="$(git ls-files -s | awk '$1 == "120000" { sub(/^[^\t]*\t/, ""); print }' | grep -v '/testdata/' || true)"

if [ -n "$links" ]; then
  echo "tracked symlinks outside testdata:" >&2
  echo "$links" >&2
  exit 1
fi
