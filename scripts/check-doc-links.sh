#!/bin/sh
# Fails if a Markdown file links to another local file that does not exist.
set -eu

status=0
for file in $(git ls-files '*.md'); do
  dir=$(dirname "$file")
  for target in $(grep -oE '\]\([^)]+\)' "$file" | sed -E 's/^\]\(([^)#]*).*/\1/' | grep -vE '^(https?:|mailto:|$)' || true); do
    if [ ! -e "$dir/$target" ]; then
      echo "broken link in $file: $target"
      status=1
    fi
  done
done
exit $status
