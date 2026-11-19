#!/usr/bin/env bash
set -euo pipefail
repo="$GITHUB_REPOSITORY"
tag="v$HCAI_VERSION"
[[ "$HCAI_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+-hcai(\(fix[1-9][0-9]*\))?$ ]] || exit 1
[[ -z "$(gh pr list --repo "$repo" --base main --head dev --state open --json number --jq '.[0].number // empty')" ]] || { echo 'main PR still open' >&2; exit 1; }
git fetch origin dev main "refs/tags/$tag:refs/tags/$tag"
main_sha="$(git rev-parse origin/main)"
tag_sha="$(git rev-parse "$tag^{commit}")"
[[ "$main_sha" == "$tag_sha" ]] || { echo 'Release tag does not point to main' >&2; exit 1; }
[[ "$(git rev-parse origin/dev^{tree})" == "$(git rev-parse origin/main^{tree})" ]] || { echo 'dev/main contents differ before evidence' >&2; exit 1; }
[[ "$(git show "$main_sha:backend/cmd/server/VERSION")" == "$HCAI_VERSION" ]] || exit 1
expected_go="$(git show "$main_sha:backend/go.mod" | awk '$1 == "go" { print $2; exit }')"
release="$(gh release view "$tag" --repo "$repo" --json isDraft,isPrerelease,assets,url)"
[[ "$(jq -r '.isDraft or .isPrerelease' <<<"$release")" == false ]] || exit 1
mkdir -p /tmp/hcai-release
gh release download "$tag" --repo "$repo" --dir /tmp/hcai-release --clobber
cd /tmp/hcai-release
[[ "$(find . -maxdepth 1 -type f | wc -l | tr -d ' ')" == 4 ]] || exit 1
archive="$(jq -r '.assets[].name | select(endswith("_linux_amd64.tar.gz"))' <<<"$release")"
[[ -n "$archive" && -f "$archive" ]]
sha256sum -c checksums.txt
tar -xOzf "$archive" sub2api > from-archive
gzip -dc sub2api-linux-amd64.gz > from-gzip
cmp from-archive from-gzip
[[ "$(file -b from-archive)" == *'ELF 64-bit'* ]]
module_info="$(go version -m from-archive)"
[[ -n "$expected_go" && "$(awk 'NR == 1 { print $2 }' <<<"$module_info")" == "go$expected_go" ]] || { echo 'Binary Go version mismatch' >&2; exit 1; }
for setting in 'GOOS=linux' 'GOARCH=amd64' "vcs.revision=$main_sha" 'vcs.modified=false'; do
  grep -Eq "^[[:space:]]*build[[:space:]]+$setting$" <<<"$module_info" || { echo "Binary build setting mismatch: $setting" >&2; exit 1; }
done
grep -q "^version=$HCAI_VERSION$" BUILD_INFO.txt
grep -q "^tag=$tag$" BUILD_INFO.txt
grep -q "^commit=${main_sha:0:12}$" BUILD_INFO.txt
grep -q '^target=linux_amd64$' BUILD_INFO.txt
grep -Fxq "archive=$archive" BUILD_INFO.txt
grep -Eq '^built_at=[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' BUILD_INFO.txt
{
  echo "Version: $HCAI_VERSION"
  echo "Release: $(jq -r .url <<<"$release")"
  echo "main/tag SHA: $main_sha"
  echo 'Assets and SHA-256:'
  sha256sum "$archive" sub2api-linux-amd64.gz checksums.txt BUILD_INFO.txt
  echo 'Binary identity:'
  printf '%s\n' "$module_info"
  echo 'BUILD_INFO:'
  cat BUILD_INFO.txt
} > /tmp/hcai-release-evidence.txt
