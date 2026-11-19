#!/bin/sh
# Prints "services= a b " with the folders under services/ (and bots/) that changed since the given base commit.
# Without a base (push to develop/main) or when the CI itself changed, every one is listed.
set -eu

base="${1:-}"
all=$(find services -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort | tr '\n' ' ')
[ -d bots ] && all="${all}bots "

if [ -z "$base" ]; then
  echo "services= $all"
  exit 0
fi

changed=$(git diff --name-only "$base"...HEAD)
if echo "$changed" | grep -qE '^(\.github/workflows/|scripts/ci-changed-services\.sh$)'; then
  echo "services= $all"
  exit 0
fi

touched=$(echo "$changed" | sed -nE -e 's#^services/([^/]+)/.*#\1#p' -e 's#^bots/.*#bots#p' | sort -u | tr '\n' ' ')
echo "services= $touched"
