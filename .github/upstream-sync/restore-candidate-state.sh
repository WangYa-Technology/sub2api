#!/usr/bin/env bash
set -euo pipefail
[[ "$BASE_SHA" =~ ^[0-9a-f]{40}$ && "$UPSTREAM_SHA" =~ ^[0-9a-f]{40}$ ]] || exit 1
[[ "$HCAI_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+-hcai$ ]] || exit 1
[[ "$CANDIDATE_BRANCH" == "chore/sync-upstream-${HCAI_VERSION%-hcai}" ]] || exit 1
[[ "$(git rev-parse HEAD)" == "$BASE_SHA" ]] || exit 1
[[ -s /tmp/hcai-candidate-state/worktree.tar.gz && -s /tmp/hcai-candidate-state/validation.log ]] || exit 1

tag="v${HCAI_VERSION%-hcai}"
git remote add upstream https://github.com/Wei-Shaw/sub2api.git
git fetch upstream "refs/tags/$tag:refs/tags/hcai-upstream-$tag"
[[ "$(git rev-parse "refs/tags/hcai-upstream-$tag^{commit}")" == "$UPSTREAM_SHA" ]] || exit 1
git switch -c "$CANDIDATE_BRANCH"
git config user.name 'hcai-automation[bot]'
git config user.email 'hcai-automation[bot]@users.noreply.github.com'
if ! git merge --no-commit --no-ff "$UPSTREAM_SHA"; then
  [[ -n "$(git ls-files -u)" ]] || { echo 'Merge failed without conflicts' >&2; exit 1; }
fi

mkdir -p /tmp/hcai-candidate-restored
tar -xzf /tmp/hcai-candidate-state/worktree.tar.gz -C /tmp/hcai-candidate-restored
rsync -a --delete --exclude='.git' --exclude='frontend/node_modules' --exclude='frontend/dist' \
  /tmp/hcai-candidate-restored/ "$GITHUB_WORKSPACE/"
cp /tmp/hcai-candidate-state/validation.log /tmp/hcai-candidate-validation.log
[[ "$(git rev-parse HEAD)" == "$BASE_SHA" && "$(git rev-parse MERGE_HEAD)" == "$UPSTREAM_SHA" ]] || exit 1
[[ "$(cat backend/cmd/server/VERSION)" == "$HCAI_VERSION" ]] || exit 1
