#!/usr/bin/env bash
set -euo pipefail
git diff --check
if [[ -z "$(git status --porcelain)" ]]; then
  echo 'Codex made no repair; manual review required' >&2
  exit 1
fi
git add -A
while IFS= read -r -d '' path; do git add -f -- "$path"; done < <(git ls-files --others --exclude-standard -z docs/hcai-dev)
git diff --cached --check
git config user.name 'hcai-automation[bot]'
git config user.email 'hcai-automation[bot]@users.noreply.github.com'
git commit -m "test(hcai): repair upstream CI run $RUN_ID"
