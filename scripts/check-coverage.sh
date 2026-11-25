#!/bin/sh
set -eu
min="${COVERAGE_MIN:-70}"
profile="$(mktemp)"
trap 'rm -f "$profile"' EXIT
go test -coverprofile="$profile" ./... >/dev/null
total="$(go tool cover -func="$profile" | awk '/^total:/ {sub("%","",$3); print $3}')"
if [ -z "$total" ]; then echo "could not compute coverage"; exit 1; fi
if awk -v t="$total" -v m="$min" 'BEGIN{exit !(t+0 < m+0)}'; then
  echo "coverage ${total}% < ${min}%"
  exit 1
fi
echo "coverage ${total}% >= ${min}%"
