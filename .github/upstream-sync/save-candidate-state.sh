#!/usr/bin/env bash
set -euo pipefail
[[ -f "$(git rev-parse --git-path MERGE_HEAD)" ]] || { echo 'Expected pending merge' >&2; exit 1; }
[[ -s /tmp/hcai-candidate-validation.log ]] || { echo 'Missing failed validation log' >&2; exit 1; }
mkdir -p /tmp/hcai-candidate-state
tar -czf /tmp/hcai-candidate-state/worktree.tar.gz \
  --exclude='./.git' --exclude='./frontend/node_modules' --exclude='./frontend/dist' \
  -C "$GITHUB_WORKSPACE" .
cp /tmp/hcai-candidate-validation.log /tmp/hcai-candidate-state/validation.log
