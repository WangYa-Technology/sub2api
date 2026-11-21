#!/usr/bin/env bash
set -euo pipefail
[[ "$(git branch --show-current)" == "$CANDIDATE_BRANCH" ]]
git -c http.extraheader="AUTHORIZATION: basic $(printf 'x-access-token:%s' "$GH_TOKEN" | base64 -w0)" push origin "HEAD:refs/heads/$CANDIDATE_BRANCH"
cat > /tmp/hcai-pr-body.md <<EOF
## 上游版本

- Release: $UPSTREAM_URL
- 上游 SHA: \`$UPSTREAM_SHA\`
- HCAI 版本: \`$HCAI_VERSION\`

## 审查

- [合并记录](https://github.com/$GITHUB_REPOSITORY/blob/$CANDIDATE_BRANCH/docs/hcai-dev/上游合并记录-$HCAI_VERSION.md)
- [部署前审查](https://github.com/$GITHUB_REPOSITORY/blob/$CANDIDATE_BRANCH/docs/hcai-dev/部署前审查-$HCAI_VERSION.md)
- [净差异清单](https://github.com/$GITHUB_REPOSITORY/blob/$CANDIDATE_BRANCH/docs/hcai-dev/上游净差异清单-$HCAI_VERSION.md)

这是自动生成的候选。须核对最新 head SHA 的 Backend checks、Frontend checks、HCAI 定制及文档，并由 CODEOWNER 审查或获得本版本明确例外授权。此 PR 不授权生产部署。
EOF
gh api "repos/$GITHUB_REPOSITORY" --jq .full_name
jq -n --rawfile body /tmp/hcai-pr-body.md --arg head "$CANDIDATE_BRANCH" --arg title "hcai(upstream): merge ${HCAI_VERSION%-hcai}" \
  '{base:"dev",head:$head,title:$title,body:$body}' |
  gh api --method POST "repos/$GITHUB_REPOSITORY/pulls" --input - --jq .html_url
bash .github/upstream-sync/enable-auto-merge.sh "$CANDIDATE_BRANCH" dev
