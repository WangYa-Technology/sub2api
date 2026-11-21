#!/usr/bin/env bash
set -euo pipefail

repo="${GITHUB_REPOSITORY:-WangYa-Technology/sub2api}"
[[ "$repo" == WangYa-Technology/sub2api ]] || { echo 'Unexpected origin repository' >&2; exit 1; }
git remote set-url origin "https://github.com/$repo.git"
git remote add upstream https://github.com/Wei-Shaw/sub2api.git
git fetch origin dev main --no-tags
active="$(gh pr list --repo "$repo" --base dev --state open --json headRefName --jq '[.[].headRefName | select(startswith("chore/sync-upstream-"))] | length')"
if [[ "$active" != 0 ]]; then
  echo 'An upstream candidate PR is still open; finish it before starting another version' >&2
  exit 1
fi

if [[ -n "${REQUESTED_TAG:-}" ]]; then
  tag="$REQUESTED_TAG"
else
  tag="$(gh release view --repo Wei-Shaw/sub2api --json tagName --jq .tagName)"
fi
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "Expected stable upstream tag: $tag" >&2; exit 1; }
release="$(gh release view "$tag" --repo Wei-Shaw/sub2api --json url,isDraft,isPrerelease,tagName)"
[[ "$(jq -r '.isDraft or .isPrerelease' <<<"$release")" == false ]] || { echo 'Release is draft or prerelease' >&2; exit 1; }
git fetch upstream "refs/tags/$tag:refs/tags/hcai-upstream-$tag"
upstream_sha="$(git rev-parse "refs/tags/hcai-upstream-$tag^{commit}")"
version="${tag#v}-hcai"
branch="chore/sync-upstream-${tag#v}"

if [[ -n "${RESUME_SHA:-}" ]]; then
  [[ -n "${REQUESTED_TAG:-}" && "$RESUME_SHA" =~ ^[0-9a-f]{40}$ ]] || { echo 'Resume requires an explicit tag and full candidate SHA' >&2; exit 1; }
  git fetch origin "refs/heads/$branch:refs/remotes/origin/$branch"
  [[ "$(git rev-parse "origin/$branch")" == "$RESUME_SHA" ]] || { echo 'Remote candidate SHA changed' >&2; exit 1; }
  read -r commit first_parent second_parent extra <<<"$(git rev-list --parents -n 1 "$RESUME_SHA")"
  [[ "$commit" == "$RESUME_SHA" && -n "$first_parent" && "$second_parent" == "$upstream_sha" && -z "${extra:-}" ]] || { echo 'Candidate merge parents changed' >&2; exit 1; }
  git merge-base --is-ancestor "$first_parent" origin/dev || { echo 'Candidate base is not in dev' >&2; exit 1; }
  git switch -c "$branch" "origin/$branch"
  [[ "$(git show "$RESUME_SHA:backend/cmd/server/VERSION")" == "$version" ]] || { echo 'Candidate VERSION mismatch' >&2; exit 1; }
  for name in "上游合并记录-$version.md" "部署前审查-$version.md" "上游净差异清单-$version.md" "版本发布记录-$version.md"; do
    [[ -s "docs/hcai-dev/$name" ]] || { echo "Missing candidate record: $name" >&2; exit 1; }
  done
  {
    echo 'run=resume'
    echo "version=$version"
    echo "branch=$branch"
    echo "upstream_sha=$upstream_sha"
    echo "release_url=$(jq -r .url <<<"$release")"
  } >> "$GITHUB_OUTPUT"
  exit 0
fi

if git merge-base --is-ancestor "$upstream_sha" origin/dev; then
  echo "Already integrated: $tag"
  echo 'run=false' >> "$GITHUB_OUTPUT"
  exit 0
fi
if gh pr list --repo "$repo" --state all --head "$branch" --json number --jq 'length' | grep -qv '^0$'; then
  echo "Candidate already exists: $branch"
  echo 'run=false' >> "$GITHUB_OUTPUT"
  exit 0
fi
if git ls-remote --exit-code --heads origin "$branch" >/dev/null; then
  echo "Remote candidate exists: $branch" >&2
  exit 1
fi
git switch -c "$branch" origin/dev
git config user.name 'hcai-automation[bot]'
git config user.email 'hcai-automation[bot]@users.noreply.github.com'
printf 'HCAI dev=%s main=%s upstream=%s (%s)\n' "$(git rev-parse origin/dev)" "$(git rev-parse origin/main)" "$tag" "$upstream_sha"
if git merge --no-commit --no-ff "$upstream_sha"; then
  echo 'Merge has no textual conflicts'
else
  [[ -n "$(git ls-files -u)" ]] || { echo 'Merge failed without conflicts' >&2; exit 1; }
  git diff --name-only --diff-filter=U
fi
{
  echo 'run=true'
  echo "version=$version"
  echo "branch=$branch"
  echo "upstream_sha=$upstream_sha"
  echo "release_url=$(jq -r .url <<<"$release")"
} >> "$GITHUB_OUTPUT"
