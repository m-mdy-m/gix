#!/usr/bin/env sh
set -eu

FILE="${1:-}"
if [ -z "$FILE" ] || [ ! -f "$FILE" ]; then
  echo "commit-msg: commit message file is required" >&2
  exit 1
fi

SUBJECT="$(sed -n '/^[^#]/ {p; q;}' "$FILE")"

case "$SUBJECT" in
  Merge\ *|Revert\ "*"|Reapply\ "*")
    exit 0
    ;;
esac

if printf '%s\n' "$SUBJECT" | grep -Eq '^(feat|fix|docs|refactor|test|build|ci|chore|perf|style|revert)(\([a-z0-9./_-]+\))?!?: .+'; then
  exit 0
fi

echo "commit-msg: use Conventional Commits, e.g. 'feat(flow): add release support'" >&2
exit 1
