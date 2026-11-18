#!/usr/bin/env bash
set -euo pipefail
[[ "$HEAD_REPO" == "$GITHUB_REPOSITORY" ]] || exit 0
[[ "$HEAD_BRANCH" =~ ^chore/sync-upstream-[0-9]+\.[0-9]+\.[0-9]+$ ]] || exit 0
pr="$(gh pr list --head "$HEAD_BRANCH" --base dev --state open --json number,headRefOid --jq '.[0] // empty')"
[[ -n "$pr" ]] || exit 0
[[ "$(jq -r .headRefOid <<<"$pr")" == "$HEAD_SHA" ]] || exit 0
version="${HEAD_BRANCH#chore/sync-upstream-}-hcai"
git fetch origin dev "refs/heads/$HEAD_BRANCH:refs/remotes/origin/$HEAD_BRANCH"
attempts="$(git log --first-parent --format=%s origin/dev.."origin/$HEAD_BRANCH" | grep -c '^test(hcai): repair upstream CI run ' || true)"
attempts="${attempts:-0}"
if (( attempts >= 3 )); then
  echo "Three auto-repair attempts reached for $HEAD_BRANCH" >&2
  exit 1
fi
git switch -c "$HEAD_BRANCH" "origin/$HEAD_BRANCH"
gh run view "$RUN_ID" --log-failed > /tmp/hcai-ci-failure.log
[[ -s /tmp/hcai-ci-failure.log ]] || { echo 'Failed logs unavailable' >&2; exit 1; }
printf 'CI run %s, candidate %s, SHA %s\n' "$RUN_ID" "$HEAD_BRANCH" "$HEAD_SHA" | tee /tmp/hcai-ci-context.txt
echo 'run=true' >> "$GITHUB_OUTPUT"
echo "version=$version" >> "$GITHUB_OUTPUT"
