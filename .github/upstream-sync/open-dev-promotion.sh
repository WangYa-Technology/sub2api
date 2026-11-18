#!/usr/bin/env bash
set -euo pipefail

repo="${GITHUB_REPOSITORY:-WangYa-Technology/sub2api}"
[[ "$repo" == WangYa-Technology/sub2api && "${DEV_SHA:-}" =~ ^[0-9a-f]{40}$ ]] || exit 1
[[ -s .hcai-dev-main-pr.md ]] || { echo 'AI review did not produce a PR body' >&2; exit 1; }
git diff --quiet
git diff --cached --quiet
[[ "$(git ls-files --others --exclude-standard)" == .hcai-dev-main-pr.md ]] || { echo 'AI changed unexpected files' >&2; exit 1; }
for heading in '## 更新范围' '## 功能影响' '## HCAI 定制影响' '## 验证与风险'; do
  rg -Fxq "$heading" .hcai-dev-main-pr.md || { echo "Missing PR section: $heading" >&2; exit 1; }
done
git fetch origin dev main
[[ "$(git rev-parse origin/dev)" == "$DEV_SHA" ]] || { echo 'dev moved during AI review; wait for the next push run' >&2; exit 1; }
[[ "$(git rev-parse origin/dev^{tree})" != "$(git rev-parse origin/main^{tree})" ]] || exit 0

existing="$(gh pr list --repo "$repo" --head dev --base main --state open --json number,title --jq '.[0] // empty')"
if [[ -n "$existing" ]]; then
  number="$(jq -r .number <<<"$existing")"
  [[ "$(jq -r .title <<<"$existing")" == 'hcai(dev): promote local updates' ]] || {
    echo "Another dev -> main PR is open: $number" >&2
    exit 1
  }
  gh pr edit "$number" --repo "$repo" --body-file .hcai-dev-main-pr.md
else
  gh pr create --repo "$repo" --base main --head dev --title 'hcai(dev): promote local updates' --body-file .hcai-dev-main-pr.md
fi
bash .github/upstream-sync/enable-auto-merge.sh dev main
