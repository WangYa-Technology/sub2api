#!/usr/bin/env bash
set -euo pipefail
branch="chore/release-evidence-${HCAI_VERSION%-hcai}"
if [[ "${COMMIT:-0}" != 1 ]]; then
  git fetch origin dev
  git switch -c "$branch" origin/dev
else
  [[ "$(git branch --show-current)" == "$branch" ]] || exit 1
fi
for name in "版本发布记录-$HCAI_VERSION.md" "上游合并记录-$HCAI_VERSION.md"; do
  [[ -s "docs/hcai-dev/$name" ]] || { echo "Missing $name" >&2; exit 1; }
done
# Codex edits the docs after this preparation step. This script is called again with COMMIT=1.
if [[ "${COMMIT:-0}" != 1 ]]; then exit 0; fi
[[ -z "$(git -c core.quotePath=false diff --name-only | grep -v '^docs/hcai-dev/.*\.md$')" ]] || { echo 'Evidence step changed non-document files' >&2; exit 1; }
[[ -z "$(git -c core.quotePath=false diff --cached --name-only | grep -v '^docs/hcai-dev/.*\.md$')" ]] || { echo 'Evidence step staged non-document files' >&2; exit 1; }
[[ -z "$(git -c core.quotePath=false ls-files --others --exclude-standard | grep -v '^docs/hcai-dev/.*\.md$')" ]] || { echo 'Evidence step added non-document files' >&2; exit 1; }
git add -f "docs/hcai-dev/版本发布记录-$HCAI_VERSION.md" "docs/hcai-dev/上游合并记录-$HCAI_VERSION.md"
[[ -z "$(git -c core.quotePath=false diff --cached --name-only | grep -Ev "^docs/hcai-dev/(版本发布记录|上游合并记录)-$HCAI_VERSION\\.md$")" ]] || { echo 'Only the two versioned records may be committed' >&2; exit 1; }
git diff --cached --check
git config user.name 'hcai-automation[bot]'
git config user.email 'hcai-automation[bot]@users.noreply.github.com'
git commit -m "hcai(docs): record $HCAI_VERSION release evidence"
git -c http.extraheader="AUTHORIZATION: basic $(printf 'x-access-token:%s' "$GH_TOKEN" | base64 -w0)" push origin "HEAD:refs/heads/$branch"
gh pr create --repo "$GITHUB_REPOSITORY" --base dev --head "$branch" --title "hcai(docs): $HCAI_VERSION release evidence" --body "Verified Release evidence for $HCAI_VERSION. Only versioned Markdown records. Requires final CI and review; production deployment remains separate."
bash .github/upstream-sync/enable-auto-merge.sh "$branch" dev
