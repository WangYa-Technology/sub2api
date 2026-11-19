#!/usr/bin/env bash
set -euo pipefail
branch="$(git branch --show-current)"
[[ "$branch" == "$HEAD_BRANCH" ]]
number="$(gh pr list --head "$branch" --base dev --state open --json number --jq '.[0].number')"
[[ -n "$number" ]]
attempts="$(git log --first-parent --format=%s origin/dev..HEAD | grep -c '^test(hcai): repair upstream CI run ' || true)"
[[ "${attempts:-0}" -ge 1 && "$attempts" -le 3 ]] || exit 1
git -c http.extraheader="AUTHORIZATION: basic $(printf 'x-access-token:%s' "$GH_TOKEN" | base64 -w0)" push origin "HEAD:refs/heads/$branch"
gh pr comment "$number" --body "HCAI CI auto-repair attempt $attempts/3 for failed run $RUN_ID. New head: $(git rev-parse HEAD). Full CI must pass again."
