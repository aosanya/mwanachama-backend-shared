#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
hub="${1:-$repo/../projects/hub-steward/code/wakala-api/modules/shared}"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

copy_packages() {
  local from="$1" to="$2"
  mkdir -p "$to"
  (cd "$from" && find . -name '*.go' -not -path './.git/*' -not -path './scripts/*' -print0 | tar -cf - --null -T -) | tar -xf - -C "$to"
}

copy_packages "$hub" "$work/hub"

mkdir -p "$work/backend"
for package in $(cd "$work/hub" && find . -mindepth 1 -maxdepth 1 -type d | sed 's#^\./##'); do
  if [ -d "$repo/$package" ]; then
    copy_packages "$repo/$package" "$work/backend/$package"
  fi
done

find "$work/hub" -name '*.go' -print0 | xargs -0 perl -pi -e '
  s{"wakalaserver/modules/shared/}{"github.com/aosanya/mwanachama-backend-shared/}g;
'

(cd "$repo" && go run ./scripts/hubdiff "$work/backend")
(cd "$repo" && go run ./scripts/hubdiff "$work/hub")

cd "$work"
diff -r backend hub | sed "s#^diff -r backend/#diff #"
