#!/usr/bin/env bash
set -euo pipefail

repo="${GITHUB_REPOSITORY:-WangYa-Technology/sub2api}"
[[ "$repo" == WangYa-Technology/sub2api ]] || exit 1
[[ "${PR_NUMBER:-}" =~ ^[0-9]+$ ]] || exit 1

pr="$(gh pr view "$PR_NUMBER" --repo "$repo" --json state,mergedAt,headRefName,baseRefName,title)"
[[ "$(jq -r .state <<<"$pr")" == MERGED && "$(jq -r .mergedAt <<<"$pr")" != null ]] || exit 0
head="$(jq -r .headRefName <<<"$pr")"
base="$(jq -r .baseRefName <<<"$pr")"
title="$(jq -r .title <<<"$pr")"

if [[ "$base" == dev && "$head" =~ ^chore/sync-upstream-([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
  version="${BASH_REMATCH[1]}-hcai"
  [[ "$title" == "hcai(upstream): merge ${version%-hcai}" ]] || exit 1
  [[ "$(gh api "repos/$repo/contents/backend/cmd/server/VERSION?ref=dev" --jq .content | base64 -d)" == "$version" ]] || exit 1
  git fetch origin dev main
  [[ -z "$(gh pr list --repo "$repo" --head dev --base main --state open --json number --jq '.[0].number // empty')" ]] || { echo 'dev -> main PR already open' >&2; exit 1; }
  gh pr create --repo "$repo" --base main --head dev --title "hcai(release): $version" --body "Release $version from dev. Review the versioned merge record, predeploy review, migration and rollback notes. Requires final Backend/Frontend checks and CODEOWNER approval. No production deployment."
  bash .github/upstream-sync/enable-auto-merge.sh dev main
elif [[ "$base" == dev && "$head" =~ ^chore/release-evidence-([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
  version="${BASH_REMATCH[1]}-hcai"
  [[ "$title" == "hcai(docs): $version release evidence" ]] || exit 1
  [[ -z "$(gh pr diff "$PR_NUMBER" --repo "$repo" --name-only | grep -v '^docs/hcai-dev/.*\.md$')" ]] || exit 1
  [[ -z "$(gh pr list --repo "$repo" --head dev --base main --state open --json number --jq '.[0].number // empty')" ]] || { echo 'dev -> main PR already open' >&2; exit 1; }
  gh pr create --repo "$repo" --base main --head dev --title "hcai(docs): $version release evidence" --body "Pure Markdown release evidence for $version. Final CI and CODEOWNER approval required. Production deployment remains separate."
  bash .github/upstream-sync/enable-auto-merge.sh dev main
elif [[ "$base" == main && "$head" == dev && "$title" =~ ^hcai\(docs\):\ ([0-9]+\.[0-9]+\.[0-9]+-hcai)\ release\ evidence$ ]]; then
  version="${BASH_REMATCH[1]}"
  tag="v$version"
  [[ -z "$(gh pr diff "$PR_NUMBER" --repo "$repo" --name-only | grep -v '\.md$')" ]] || exit 1
  git fetch origin dev main
  [[ "$(git rev-parse origin/dev^{tree})" == "$(git rev-parse origin/main^{tree})" ]] || { echo 'dev/main trees differ; retain temporary branches' >&2; exit 1; }
  gh release view "$tag" --repo "$repo" >/dev/null
  for branch in "chore/sync-upstream-${version%-hcai}" "chore/release-evidence-${version%-hcai}"; do
    [[ -z "$(gh pr list --repo "$repo" --head "$branch" --state open --json number --jq '.[0].number // empty')" ]] || exit 1
    if gh api "repos/$repo/git/ref/heads/$branch" >/dev/null 2>&1; then
      gh api --method DELETE "repos/$repo/git/refs/heads/$branch"
    fi
  done
  echo "Complete: dev/main trees match, release $tag verified, temporary branches removed. Production not deployed."
fi
