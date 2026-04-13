#!/bin/bash
# TDD Sentinel — warns when a production .go file is written without a corresponding test.
# Used as a PostToolUse hook in Claude Code settings.json.
# WARNING only, not blocking — a nudge to write tests first.

# Extract file_path from stdin JSON and check for test file
RESULT=$(python3 -c "
import sys, json, os

raw = sys.stdin.read()
try:
    data = json.loads(raw)
except Exception:
    sys.exit(0)

tool_input = data.get('tool_input', {})
file_path = tool_input.get('file_path', '')

# Only check .go files
if not file_path.endswith('.go'):
    sys.exit(0)

# Skip test files — they ARE the test
if file_path.endswith('_test.go'):
    sys.exit(0)

# Only check files under internal/
if '/internal/' not in file_path:
    sys.exit(0)

# Exempt cmd/ (composition root)
if '/cmd/' in file_path:
    sys.exit(0)

# Exempt generated files (sqlc, etc.)
try:
    with open(file_path) as f:
        first_line = f.readline()
    if 'Code generated' in first_line:
        sys.exit(0)
except Exception:
    sys.exit(0)

# Check if corresponding test file exists
test_file = file_path[:-3] + '_test.go'
if os.path.exists(test_file):
    sys.exit(0)

# Extract relative path for readability
rel = file_path
if '/internal/' in file_path:
    idx = file_path.index('internal/')
    rel = file_path[idx:]

test_rel = test_file
if '/internal/' in test_file:
    idx = test_file.index('internal/')
    test_rel = test_file[idx:]

msg = (
    f'TDD Reminder: {rel} was written but no corresponding test file exists ({test_rel}). '
    f'Consider writing the test first.'
)
result = {'hookEventName': 'PostToolUse', 'additionalContext': msg}
print(json.dumps(result))
")

if [ -n "$RESULT" ]; then
  echo "$RESULT"
fi
