#!/bin/sh
# Usage: scaffold.sh <target-dir> <go-module-name>
set -eu
[ $# -eq 2 ] || { echo "usage: $0 <target-dir> <module-name>"; exit 2; }
target="$1"; module="$2"
root="$(cd "$(dirname "$0")/../../.." && pwd)"
mkdir -p "$target"
if [ -n "$(ls -A "$target" 2>/dev/null | grep -v '^\.git$' || true)" ]; then
  echo "target is not empty: $target"; exit 1
fi
cp -R "$root/app" "$target/app"
mkdir -p "$target/scripts" "$target/scenarios" "$target/docs/adr"
cp "$root/Makefile" "$target/Makefile"
cp "$root/scripts/check-coverage.sh" "$target/scripts/"
cp "$root/AGENTS.md" "$root/CLAUDE.md" "$root/.gitignore" "$root/.env.example" "$target/"
cp "$root/docs/conventions.md" "$root/docs/testing.md" "$root/docs/code-style.md" "$root/docs/scenarios.md" "$target/docs/"
cp "$root/docs/adr/0000-template.md" "$target/docs/adr/"
cp "$root/scenarios/health.feature" "$target/scenarios/"
printf 'module %s\n\ngo 1.22\n' "$module" > "$target/go.mod"
(cd "$target" && go get github.com/cucumber/godog@latest && go mod tidy)
sed -i.bak 's#^\t@sh scripts/validate-skills.sh#\t@echo "no skills to validate"#' "$target/Makefile" && rm -f "$target/Makefile.bak"
sed -i.bak "s/^COVERAGE_MIN ?= .*/COVERAGE_MIN ?= 50/" "$target/Makefile" && rm -f "$target/Makefile.bak"
echo "scaffolded $target (module $module)"
