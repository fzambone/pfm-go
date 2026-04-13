#!/bin/bash
# Import Rule Enforcer — checks domain files for forbidden imports after Edit/Write.
# Used as a PostToolUse hook in Claude Code settings.json.
#
# Allowed internal imports for domain packages:
#   internal/message/, internal/types/, internal/platform/validate/
# Forbidden:
#   internal/adapter/*, internal/middleware/*, internal/platform/* (except validate)

# Extract file_path from stdin JSON, then check imports
RESULT=$(python3 -c "
import sys, json, re

raw = sys.stdin.read()
try:
    data = json.loads(raw)
except Exception:
    sys.exit(0)

tool_input = data.get('tool_input', {})
file_path = tool_input.get('file_path', '')

# Only check files under internal/domain/
if '/internal/domain/' not in file_path:
    sys.exit(0)

# Exempt test files
if file_path.endswith('_test.go'):
    sys.exit(0)

# Read the file
try:
    with open(file_path) as f:
        content = f.read()
except Exception:
    sys.exit(0)

# Extract all import paths from the file
imports = re.findall(r'\"([^\"]+)\"', content.split('func ')[0] if 'func ' in content else content)

# Only check internal imports (skip stdlib and third-party)
forbidden = []
for imp in imports:
    if '/internal/' not in imp:
        continue
    # Extract the internal path portion
    idx = imp.index('/internal/')
    internal_path = imp[idx:]

    # Allowed internal imports
    if internal_path.startswith('/internal/message'):
        continue
    if internal_path.startswith('/internal/types'):
        continue
    if internal_path.startswith('/internal/platform/validate'):
        continue
    if internal_path.startswith('/internal/domain/'):
        continue
    # Port interfaces are consumed by domain
    if internal_path.startswith('/internal/port/'):
        continue

    # Everything else is forbidden
    forbidden.append(imp)

if forbidden:
    lines = []
    for f in forbidden:
        lines.append(f'  Forbidden: \"{f}\"')
    detail = chr(10).join(lines)

    # Extract relative path for readability
    rel = file_path
    if '/internal/domain/' in file_path:
        idx = file_path.index('internal/domain/')
        rel = file_path[idx:]

    msg = (
        f'IMPORT RULE VIOLATION in {rel}:\\n'
        f'{detail}\\n\\n'
        f'Domain packages must not import adapter/, platform/ (except validate), or middleware/.\\n'
        f'Allowed internal imports: message/, types/, platform/validate/, port/, other domain packages.\\n'
        f'Fix the import before continuing.'
    )
    result = {'hookEventName': 'PostToolUse', 'additionalContext': msg}
    print(json.dumps(result))
")

# Output result if any (empty = silent pass)
if [ -n "$RESULT" ]; then
  echo "$RESULT"
fi
