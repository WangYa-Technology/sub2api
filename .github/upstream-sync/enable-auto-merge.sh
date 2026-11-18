#!/usr/bin/env bash
set -euo pipefail

head="$1"
base="$2"
repo="${GITHUB_REPOSITORY:-WangYa-Technology/sub2api}"
[[ "$repo" == WangYa-Technology/sub2api ]] || exit 1
[[ "$base" == dev || "$base" == main ]] || exit 1

[[ "$(gh api "repos/$repo" --jq .allow_auto_merge)" == true ]] || {
  echo 'Repository auto-merge is disabled' >&2
  exit 1
}
protection="$(gh api "repos/$repo/branches/$base/protection")" || {
  echo "Cannot verify $base branch protection" >&2
  exit 1
}
for name in 'Backend checks' 'Frontend checks'; do
  jq -e --arg name "$name" '((.required_status_checks.contexts // []) + ((.required_status_checks.checks // []) | map(.context))) | index($name)' <<<"$protection" >/dev/null || {
    echo "Missing required $name on $base" >&2
    exit 1
  }
done
jq -e '.required_pull_request_reviews.require_code_owner_reviews == true and .required_pull_request_reviews.required_approving_review_count >= 1' <<<"$protection" >/dev/null || {
  echo "CODEOWNER approval must be required on $base" >&2
  exit 1
}

number="$(gh pr list --repo "$repo" --head "$head" --base "$base" --state open --json number --jq '.[0].number // empty')"
[[ -n "$number" ]] || { echo "Missing $head -> $base PR" >&2; exit 1; }
[[ "$(gh pr view "$number" --repo "$repo" --json autoMergeRequest --jq '.autoMergeRequest != null')" == true ]] && exit 0
gh pr merge "$number" --repo "$repo" --auto --merge
