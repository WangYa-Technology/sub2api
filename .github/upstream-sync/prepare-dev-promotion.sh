#!/usr/bin/env bash
set -euo pipefail

repo="${GITHUB_REPOSITORY:-WangYa-Technology/sub2api}"
[[ "$repo" == WangYa-Technology/sub2api && "${DEV_SHA:-}" =~ ^[0-9a-f]{40}$ ]] || exit 1
git fetch origin dev main
[[ "$(git rev-parse origin/dev)" == "$DEV_SHA" ]] || { echo 'Superseded dev push; newer run will handle it'; exit 0; }
if [[ "$(git rev-parse origin/dev^{tree})" == "$(git rev-parse origin/main^{tree})" ]]; then
  echo 'dev and main have identical content'
  exit 0
fi

# Upstream candidates and release evidence have their own merged-PR progression.
prs="$(gh api "repos/$repo/commits/$DEV_SHA/pulls" --paginate)"
if jq -e --arg sha "$DEV_SHA" '[.[] | select(.merge_commit_sha == $sha and .merged_at != null and (.head.ref | test("^chore/(sync-upstream|release-evidence)-[0-9]+\\.[0-9]+\\.[0-9]+$")))] | length > 0' <<<"$prs" >/dev/null; then
  echo 'Specialized upstream PR progression owns this push'
  exit 0
fi
echo 'run=true' >> "$GITHUB_OUTPUT"
