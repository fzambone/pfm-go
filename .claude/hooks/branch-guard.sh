#!/bin/bash
# Branch Guard — blocks Edit/Write on the main branch.
# Used as a PreToolUse hook in Claude Code settings.json.

# Consume stdin (hook protocol sends JSON with tool_name and tool_input)
cat > /dev/null

# Not a git repo — allow the edit
if ! git rev-parse --is-inside-work-tree &>/dev/null; then
  exit 0
fi

# Detached HEAD (e.g., during rebase) — allow the edit
BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null)
if [ $? -ne 0 ]; then
  exit 0
fi

# Block on main
if [ "$BRANCH" = "main" ]; then
  printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"Branch Guard: Cannot edit files on main. Create a feature branch first:\\n  git checkout -b feat/<scope>-<description>-<N>"}}'
  exit 0
fi
