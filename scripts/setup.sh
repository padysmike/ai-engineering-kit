#!/usr/bin/env bash
# Walks a human through setting up their own copy of the kit.
# Every step checks first and skips what is already done, so it is safe to run again.
#
#   scripts/setup.sh                 set up, or finish a partial setup
#   scripts/setup.sh --check         only report the state of every step
#   scripts/setup.sh --new-project   start a fresh project branch from main
#
# Agents never run this script: it handles tokens and repository settings.
set -euo pipefail

MODE=setup
case "${1:-}" in
  "") ;;
  --check) MODE=check ;;
  --new-project) MODE=new-project ;;
  *) echo "usage: scripts/setup.sh [--check | --new-project]"; exit 2 ;;
esac

cd "$(git rev-parse --show-toplevel)"
KIT_HOME="${AI_KIT_HOME:-$HOME/.ai-engineering-kit}"
IMAGE=ai-engineering-checks
PORT="${GATE_PORT:-8765}"
PENDING=0
ME='' SLUG='' BRANCH='' STATE='' SECRETS='' WHERE='' LISTEN='' URL=''

if [ -t 1 ]; then B=$'\033[1m'; G=$'\033[32m'; Y=$'\033[33m'; R=$'\033[31m'; N=$'\033[0m'; else B=; G=; Y=; R=; N=; fi
step() { printf '\n%s== %s%s\n' "$B" "$1" "$N"; }
ok()   { printf '  %s[ok]%s   %s\n' "$G" "$N" "$1"; }
todo() { printf '  %s[todo]%s %s\n' "$Y" "$N" "$1"; if checking; then PENDING=$((PENDING + 1)); fi; }
open_step() { printf '  %s[todo]%s %s\n' "$Y" "$N" "$1"; PENDING=$((PENDING + 1)); } # stays open even after setup
fail() { printf '  %s[fail]%s %s\n' "$R" "$N" "$1"; exit 1; }
say()  { printf '         %s\n' "$@"; }
checking() { [ "$MODE" = check ]; }
TTY_IN=/dev/tty TTY_OUT=/dev/tty
if ! { : < /dev/tty; } 2> /dev/null; then TTY_IN=/dev/stdin TTY_OUT=/dev/stderr; fi

ask() { # ask "question" "default" -> answer on stdout
  local answer
  printf '  %s [%s]: ' "$1" "$2" > "$TTY_OUT"
  IFS= read -r answer < "$TTY_IN" || true
  printf '%s' "${answer:-$2}"
}
ask_secret() {
  local answer
  printf '  %s: ' "$1" > "$TTY_OUT"
  IFS= read -rs answer < "$TTY_IN" || true
  printf '\n' > "$TTY_OUT"
  printf '%s' "$answer" | tr -d '[:space:]'
}

# owner/repo of a GitHub remote URL (https or ssh form)
slug_of() {
  local url="$1"
  url="${url%.git}"; url="${url%/}"
  case "$url" in
    git@github.com:*) url="${url#git@github.com:}" ;;
    ssh://git@github.com/*) url="${url#ssh://git@github.com/}" ;;
    https://github.com/*) url="${url#https://github.com/}" ;;
    *) return 1 ;;
  esac
  printf '%s' "$url"
}

version_at_least() { # version_at_least 1.26.3 1.25.0
  awk -v have="$1" -v need="$2" 'BEGIN {
    split(have, h, "."); split(need, n, ".")
    for (i = 1; i <= 3; i++) { if (h[i] + 0 > n[i] + 0) exit 0; if (h[i] + 0 < n[i] + 0) exit 1 }
    exit 0 }'
}

prerequisites() {
  step "Tools on this machine"
  local missing=0 cmd
  for cmd in git go docker gh make curl; do
    if command -v "$cmd" > /dev/null; then ok "$cmd"; else todo "$cmd is not installed"; missing=1; fi
  done
  [ "$missing" = 0 ] || fail "install the missing tools and run this again"
  local need have
  need=$(awk '$1 == "go" {print $2}' go.mod)
  have=$(go env GOVERSION | sed 's/^go//')
  if version_at_least "$have" "$need"; then ok "go $have (needs $need or newer)"; else fail "go $have is too old, install $need or newer"; fi
  if docker info > /dev/null 2>&1; then ok "docker daemon is running"; else fail "docker is installed but not running (or you lack permission to use it)"; fi
  if gh auth status > /dev/null 2>&1; then ok "gh is logged in"; else fail "run: gh auth login"; fi
}

own_repository() {
  step "Your own GitHub repository"
  ME=$(gh api user -q .login)
  SLUG=$(slug_of "$(git remote get-url origin)") || fail "origin is not a GitHub repository: $(git remote get-url origin)"
  if [ "${SLUG%%/*}" = "$ME" ]; then
    ok "origin is $SLUG, which is yours"
    gh repo set-default "$SLUG" > /dev/null 2>&1 || true
    return
  fi
  todo "origin is $SLUG; you need your own copy, so your rules, tokens and gate are yours"
  if checking; then summary; exit 0; fi
  say "This creates a repository under $ME, renames the current origin to 'upstream'" \
      "and pushes the kit's main branch to your repository."
  local name visibility
  name=$(ask "Repository name" "${SLUG#*/}")
  say "Rulesets on a private repository need GitHub Pro; public works on every plan."
  visibility=$(ask "Visibility (public/private)" public)
  case "$visibility" in public|private) ;; *) fail "visibility must be public or private" ;; esac
  git remote rename origin upstream
  gh repo create "$ME/$name" "--$visibility" --source . --remote origin --description "Built with ai-engineering-kit"
  git fetch -q upstream main
  git push -q origin refs/remotes/upstream/main:refs/heads/main
  SLUG="$ME/$name"
  gh repo set-default "$SLUG" > /dev/null
  ok "created $SLUG and pushed main"
}

project_branch() {
  step "Project branch (your product lives here; main stays the kit)"
  BRANCH=$(gh api "repos/$SLUG" -q .default_branch)
  if [ "$BRANCH" != main ] && [ "$MODE" != new-project ]; then
    ok "project branch is $BRANCH (the GitHub default branch)"
  else
    if [ "$BRANCH" = main ]; then todo "the default branch is still main"; fi
    checking && return
    local name
    name=$(ask "Name of the project branch" project)
    [ "$name" != main ] || fail "the project branch must not be main"
    git fetch -q origin main
    if git ls-remote --exit-code --heads origin "$name" > /dev/null; then
      say "branch $name already exists on GitHub, using it"
    else
      git push -q origin "refs/remotes/origin/main:refs/heads/$name"
    fi
    gh api -X PATCH "repos/$SLUG" -f default_branch="$name" > /dev/null
    BRANCH="$name"
    ok "created $BRANCH from main and made it the default branch"
  fi
  if ! checking && [ "$(git branch --show-current)" != "$BRANCH" ] && [ -z "$(git status --porcelain)" ]; then
    git fetch -q origin "$BRANCH"
    git switch -q "$BRANCH" 2> /dev/null || git switch -q -c "$BRANCH" --track "origin/$BRANCH"
    ok "checked out $BRANCH"
  elif ! checking && [ "$(git branch --show-current)" != "$BRANCH" ]; then
    say "You have uncommitted changes, so $BRANCH was not checked out. Later: git switch $BRANCH"
  fi
}

ruleset() {
  step "Ruleset: nothing reaches $BRANCH or main without the gate"
  local names
  if ! names=$(gh api "repos/$SLUG/rulesets" -q '.[].name' 2>&1); then
    case "$names" in *"Upgrade to GitHub Pro"*) fail "rulesets need a public repository or GitHub Pro: gh repo edit $SLUG --visibility public --accept-visibility-change-consequences" ;; esac
    fail "cannot read rulesets: $names"
  fi
  if printf '%s\n' "$names" | grep -qx gate; then ok "ruleset 'gate' exists"; return; fi
  todo "no ruleset 'gate'"
  checking && return
  gh api -X POST "repos/$SLUG/rulesets" --input - > /dev/null << 'EOF'
{
  "name": "gate",
  "target": "branch",
  "enforcement": "active",
  "bypass_actors": [],
  "conditions": {"ref_name": {"include": ["~DEFAULT_BRANCH", "refs/heads/main"], "exclude": []}},
  "rules": [
    {"type": "deletion"},
    {"type": "non_fast_forward"},
    {"type": "pull_request", "parameters": {"required_approving_review_count": 0, "dismiss_stale_reviews_on_push": false, "require_code_owner_review": false, "require_last_push_approval": false, "required_review_thread_resolution": false}},
    {"type": "required_status_checks", "parameters": {"strict_required_status_checks_policy": false, "required_status_checks": [{"context": "gate"}]}}
  ]
}
EOF
  ok "created ruleset 'gate': pull request and status 'gate' required on the default branch and main, no bypass"
}

# Expected answers to a status write with an empty body: no permission -> 403, permission -> 422.
probe_token() { # probe_token TOKEN EXPECTED_STATUS_CODE -> 0 when the token fits its role
  local token="$1" expect="$2" sha code
  code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $token" "https://api.github.com/repos/$SLUG/pulls?per_page=1")
  [ "$code" = 200 ] || { say "the token cannot read pull requests of $SLUG (HTTP $code)"; return 1; }
  sha=$(gh api "repos/$SLUG/commits/$BRANCH" -q .sha)
  code=$(curl -s -o /dev/null -w '%{http_code}' -X POST -d '{}' -H "Authorization: Bearer $token" "https://api.github.com/repos/$SLUG/statuses/$sha")
  [ "$code" = "$expect" ] || { say "commit statuses answered HTTP $code, expected $expect"; return 1; }
}

token() { # token agent|gate
  local kind="$1" file="$SECRETS/$1-github-token" expect statuses value
  if [ "$kind" = agent ]; then expect=403; statuses="No access (this is what stops the agent from approving itself)"
  else expect=422; statuses="Read and write"; fi
  if [ -s "$file" ] && probe_token "$(cat "$file")" "$expect"; then ok "$kind token works and has exactly the right rights"; return; fi
  todo "$kind token missing or wrong ($file)"
  checking && return
  say "Create it at https://github.com/settings/personal-access-tokens/new" \
      "  Token name:          $kind ($SLUG)" \
      "  Expiration:          a date shortly after the course" \
      "  Repository access:   Only select repositories -> $SLUG" \
      "  Permissions:         Contents: Read and write" \
      "                       Pull requests: Read and write" \
      "                       Commit statuses: $statuses" \
      "                       everything else: No access"
  while :; do
    value=$(ask_secret "Paste the $kind token (input is hidden)")
    [ -n "$value" ] || fail "no token given"
    if probe_token "$value" "$expect"; then break; fi
    say "That token does not fit; fix its permissions on GitHub and paste it again."
  done
  (umask 077; printf '%s\n' "$value" > "$file")
  ok "$kind token saved to $file"
}

tokens() {
  step "Two GitHub tokens: one for the agent, one for the gate"
  STATE="$KIT_HOME/${SLUG/\//__}"
  SECRETS="$STATE/secrets"
  checking || (umask 077; mkdir -p "$SECRETS" "$STATE/approvals")
  say "They live in $SECRETS, outside the repository."
  token agent
  token gate
}

check_image() {
  step "Container image the gate runs checks in"
  if docker image inspect "$IMAGE" > /dev/null 2>&1; then ok "image $IMAGE exists"; return; fi
  todo "image $IMAGE is missing"
  checking && return
  docker build -t "$IMAGE" -f gate/checks.Dockerfile gate
  ok "built $IMAGE"
}

agent_location() {
  step "Where your agent runs"
  local config="$STATE/config"
  if [ -s "$config" ]; then
    # shellcheck disable=SC1090
    . "$config"
    ok "agent runs $WHERE; gate listens on $LISTEN, agents reach it at $URL"
    return
  fi
  todo "not chosen yet"
  checking && return
  say "1) on this machine (simplest; the agent could read the gate token, so the gate teaches rather than protects)" \
      "2) in a Docker sandbox (the agent sees only what you mount; keep $KIT_HOME out of it)"
  case "$(ask "Choose 1 or 2" 1)" in
    1) WHERE="on this machine"; LISTEN="127.0.0.1:$PORT"; URL="http://127.0.0.1:$PORT/mcp" ;;
    2) WHERE="in a Docker sandbox"
       if [ "$(uname)" = Darwin ]; then LISTEN="127.0.0.1:$PORT"; URL="http://host.docker.internal:$PORT/mcp"
       else
         local bridge
         bridge=$(ip -4 -o addr show docker0 2> /dev/null | awk '{print $4}' | cut -d/ -f1)
         [ -n "$bridge" ] || fail "no docker0 bridge found; start docker and run this again"
         LISTEN="$bridge:$PORT"; URL="http://$bridge:$PORT/mcp"
         say "If a firewall is active (ufw, firewalld), let the containers reach the gate, for example:" \
             "  sudo ufw allow proto tcp from 172.16.0.0/12 to any port $PORT comment 'ai-engineering-kit gate'"
       fi ;;
    *) fail "choose 1 or 2" ;;
  esac
  printf 'WHERE=%q\nLISTEN=%q\nURL=%q\n' "$WHERE" "$LISTEN" "$URL" > "$config"
  ok "agent runs $WHERE; gate listens on $LISTEN, agents reach it at $URL"
}

gate() {
  step "The gate (runs on this machine, from its own checkout)"
  local checkout="$STATE/checkout"
  if checking; then
    if [ -x "$STATE/start-gate.sh" ]; then ok "gate installed in $STATE"; else todo "gate not installed"; fi
    return
  fi
  [ -d "$checkout/.git" ] || gh repo clone "$SLUG" "$checkout" -- -q
  git -C "$checkout" fetch -q origin
  git -C "$checkout" checkout -q --detach "origin/$BRANCH"
  (cd "$checkout" && go build -o "$STATE/gate" ./gate)
  [ -s "$SECRETS/gate-mcp-token" ] || (umask 077; od -An -tx1 -N32 /dev/urandom | tr -d ' \n' > "$SECRETS/gate-mcp-token")
  cat > "$STATE/start-gate.sh" << EOF
#!/bin/sh
# Starts the merge gate for $SLUG. Leave it running; stop it with Ctrl-C.
set -eu
base=\$(gh api repos/$SLUG -q .default_branch)
GATE_GITHUB_TOKEN=\$(cat "$SECRETS/gate-github-token")
GATE_MCP_TOKEN=\$(cat "$SECRETS/gate-mcp-token")
export GATE_GITHUB_TOKEN GATE_MCP_TOKEN
echo "gate for $SLUG guards \$base"
exec "$STATE/gate" mcp --repo "$checkout" --github "$SLUG" --base "\$base" --allow-merge \\
  --listen "$LISTEN" --state "$STATE/approvals" --sandbox-image $IMAGE
EOF
  cat > "$STATE/approve.sh" << EOF
#!/bin/sh
# Your decisions on pull requests the gate holds: approvals | approve <PR> | deny <PR> --reason "..."
# Turning single checks off and on: checks | disable <check> | enable <check>
GATE_STATE_DIR="$STATE/approvals" exec "$STATE/gate" "\$@"
EOF
  cat > "$STATE/agent-env.sh" << EOF
#!/bin/sh
# Runs a command with only the agent's GitHub token, for example: $STATE/agent-env.sh claude
GITHUB_TOKEN=\$(cat "$SECRETS/agent-github-token")
GH_TOKEN=\$GITHUB_TOKEN
export GITHUB_TOKEN GH_TOKEN
exec "\$@"
EOF
  chmod 700 "$STATE"/*.sh
  ok "gate built from $BRANCH into $STATE"
}

add_tracker() { # add_tracker NAME URL
  claude mcp add --scope local --transport http "$1" "$2" > /dev/null
  ok "registered the tracker ($1); log in once: start claude here, type /mcp, choose $1"
}

tracker() {
  local name
  for name in linear atlassian; do
    if claude mcp get "$name" > /dev/null 2>&1; then ok "Claude Code knows a tracker ($name)"; return; fi
  done
  say "A tracker is optional: without one, plan-roadmap writes docs/roadmap.md instead."
  if checking; then return; fi
  case "$(ask "Connect a tracker now? linear, jira or none" linear)" in
    linear) add_tracker linear https://mcp.linear.app/mcp ;;
    jira) add_tracker atlassian https://mcp.atlassian.com/v2/mcp ;;
    *) say "no tracker connected; run scripts/setup.sh again to add one" ;;
  esac
}

mcp_clients() {
  step "Connect your agent to the gate (and optionally a tracker)"
  if ! command -v claude > /dev/null; then
    open_step "Claude Code not found; register this MCP server in your agent yourself:"
    say "  URL $URL, transport HTTP, header 'Authorization: Bearer <contents of $SECRETS/gate-mcp-token>'"
    return
  fi
  if claude mcp get gate > /dev/null 2>&1; then ok "Claude Code knows the gate (this project)"
  else
    todo "the gate is not registered in Claude Code"
    if ! checking; then
      say "Claude config: ${CLAUDE_CONFIG_DIR:-$HOME/.claude} (for a sandbox, use the config it mounts)"
      claude mcp add --scope local --transport http gate "$URL" --header "Authorization: Bearer $(cat "$SECRETS/gate-mcp-token")" > /dev/null
      ok "registered the gate in Claude Code for this project"
    fi
  fi
  tracker
}

summary() {
  step "Summary"
  if [ "$PENDING" -gt 0 ]; then
    say "$PENDING step(s) still to do; run scripts/setup.sh to finish them."
    return
  fi
  say "Everything is set up. Every working session:" \
      "" \
      "  terminal 1:  $STATE/start-gate.sh" \
      "  terminal 2:  $STATE/agent-env.sh claude      (or start your sandbox with the agent token)" \
      "" \
      "When the gate holds a pull request for you:  $STATE/approve.sh approvals" \
      "To turn a check off for a while:             $STATE/approve.sh disable lint" \
      "Next: docs/getting-started.md, section 'The workflow'."
}

prerequisites
own_repository
project_branch
ruleset
tokens
check_image
agent_location
gate
mcp_clients
summary
