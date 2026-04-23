#!/usr/bin/env bash
# ADR requirement check - blocks PR creation when architectural changes lack an ADR.
# Runs as:
#   1. Claude Code PreToolUse hook on Bash commands (filters to gh pr create only)
#   2. Manual call from /pr command (step 5) with empty JSON "{}" on stdin
# See docs/adr/README.md for ADR process, AGENTS.md for trigger criteria.

# Read stdin (hook JSON or empty object from /pr)
INPUT=$(cat)

# Extract the command from hook JSON (empty if manual /pr call)
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null)

# Hook mode: skip non-PR commands
if [ -n "$COMMAND" ] && [[ "$COMMAND" != gh\ pr\ create* ]]; then
  exit 0
fi

BASE="main"

# If ADR already added in this branch, proceed
if git diff --name-only "$BASE"...HEAD -- 'docs/adr/' 2>/dev/null | grep -qE '/[0-9]{4}-'; then
  exit 0
fi

signals=()

# 1. Database migration/schema changes
if git diff -w --name-only "$BASE"...HEAD 2>/dev/null | grep -qiE '(db/migration|\.sql$)'; then
  signals+=("Database migration/schema changes")
fi

# 2. Security/auth configuration
if git diff -w --name-only "$BASE"...HEAD 2>/dev/null | grep -qiE '/(security|auth|jwt|oauth)/'; then
  signals+=("Security/auth configuration changes")
fi

# 3. Error handling/resilience patterns
if git diff -w "$BASE"...HEAD -- '*.java' '*.go' '*.ts' '*.js' '*.py' 2>/dev/null | grep -qE '(failOpen|circuitBreaker|retryPolicy|fallbackStrategy)'; then
  signals+=("Error handling/resilience pattern changes")
fi

# 4. New source package/module (directory that doesn't exist in base branch)
for src_dir in 'cmd/' 'pkg/' 'web/src/' 'api/'; do
  if git diff -w --diff-filter=A --name-only "$BASE"...HEAD -- "$src_dir" 2>/dev/null | grep -qE '\.(java|go|ts|js|py|vue)$'; then
    has_new_pkg=false
    while IFS= read -r f; do
      d=$(dirname "$f")
      git rev-parse --verify --quiet "$BASE:$d" >/dev/null 2>&1 || { has_new_pkg=true; break; }
    done < <(git diff -w --diff-filter=A --name-only "$BASE"...HEAD -- "$src_dir" 2>/dev/null | grep -qE '\.(java|go|ts|js|py|vue)$')
    $has_new_pkg && signals+=("New package/module added in $src_dir")
  fi
done

# No signals - proceed normally
if [ ${#signals[@]} -eq 0 ]; then
  exit 0
fi

# Build bullet list
list=""
for s in "${signals[@]}"; do
  list="${list}  - ${s}"$'\n'
done

MSG="ADR CHECK: Architectural changes detected but no ADR found in docs/adr/.

Signals:
${list}
Per AGENTS.md, evaluate if an ADR is needed (see docs/adr/template.md).
To proceed: create an ADR, or note in the PR description why none is needed."

# stdout: JSON for /pr backward compatibility
jq -n --arg msg "$MSG" '{ continue: false, stopReason: $msg }'

# stderr: plain message for hook system feedback
echo "$MSG" >&2

exit 2
