#!/bin/sh
# Validates skills/<name>/SKILL.md against the Agent Skills basics.
# Usage: validate-skills.sh [skills-dir]   (default: skills)
set -u
dir="${1:-skills}"
fail=0
err() { echo "$1: $2"; fail=1; }
[ -d "$dir" ] || { echo "no such directory: $dir"; exit 1; }
for d in "$dir"/*/; do
  [ -d "$d" ] || continue
  name="$(basename "$d")"
  f="$d/SKILL.md"
  [ -f "$f" ] || { err "$name" "missing SKILL.md"; continue; }
  [ "$(sed -n 1p "$f")" = "---" ] || { err "$name" "frontmatter must start on line 1 with ---"; continue; }
  end="$(awk 'NR>1 && /^---$/ {print NR; exit}' "$f")"
  [ -n "$end" ] || { err "$name" "frontmatter is not closed"; continue; }
  fm="$(sed -n "2,$((end-1))p" "$f")"
  body="$(sed -n "$((end+1)),\$p" "$f")"
  n="$(printf '%s\n' "$fm" | sed -n 's/^name:[[:space:]]*//p' | head -1)"
  [ "$n" = "$name" ] || err "$name" "name '$n' must equal the directory name"
  printf '%s' "$name" | grep -Eq '^[a-z0-9]+(-[a-z0-9]+)*$' && [ "${#name}" -le 64 ] || err "$name" "name must be lowercase letters, digits, hyphens (max 64)"
  desc="$(printf '%s\n' "$fm" | sed -n 's/^description:[[:space:]]*//p' | head -1)"
  [ -n "$desc" ] || err "$name" "description is empty"
  [ "${#desc}" -le 1024 ] || err "$name" "description is longer than 1024 characters"
  [ -n "$(printf '%s' "$body" | tr -d '[:space:]')" ] || err "$name" "body is empty"
  for key in $(printf '%s\n' "$fm" | grep -E '^[A-Za-z_-]+:' | sed 's/:.*//'); do
    case "$key" in name|description|license|compatibility|metadata) ;; *) err "$name" "unsupported frontmatter key '$key'";; esac
  done
done
[ "$fail" -eq 0 ] && echo "skills ok"
exit "$fail"
