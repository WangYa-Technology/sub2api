#!/usr/bin/env bash
set -euo pipefail
[[ "${HCAI_VERSION:-}" =~ ^[0-9]+\.[0-9]+\.[0-9]+-hcai$ ]]
merge_head="$(git rev-parse --git-path MERGE_HEAD)"
[[ -f "$merge_head" ]] || { echo 'Expected pending two-parent merge' >&2; exit 1; }
[[ "$(cat "$merge_head")" == "$UPSTREAM_SHA" ]] || { echo 'Merge SHA changed' >&2; exit 1; }
[[ -z "$(git ls-files -u)" ]] || { echo 'Unresolved conflicts' >&2; exit 1; }
[[ "$(cat backend/cmd/server/VERSION)" == "$HCAI_VERSION" ]] || { echo 'VERSION mismatch' >&2; exit 1; }
for name in "上游合并记录-$HCAI_VERSION.md" "部署前审查-$HCAI_VERSION.md" "上游净差异清单-$HCAI_VERSION.md" "版本发布记录-$HCAI_VERSION.md"; do
  [[ -s "docs/hcai-dev/$name" ]] || { echo "Missing $name" >&2; exit 1; }
  git add -f "docs/hcai-dev/$name"
done
git diff --check
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
pnpm --dir frontend run build
(
  cd backend
  go test -tags=unit ./...
  go test ./...
  CGO_ENABLED=0 go build -o /tmp/hcai-candidate-server ./cmd/server
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed -o /tmp/hcai-candidate-embed ./cmd/server
)
git add -A
while IFS= read -r -d '' path; do git add -f -- "$path"; done < <(git ls-files --others --exclude-standard -z docs/hcai-dev)
git diff --cached --check
git config user.name 'hcai-automation[bot]'
git config user.email 'hcai-automation[bot]@users.noreply.github.com'
git commit -m "hcai(upstream): merge ${HCAI_VERSION%-hcai}"
[[ "$(git rev-list --parents -n 1 HEAD | wc -w | tr -d ' ')" == 3 ]] || { echo 'Merge commit must have two parents' >&2; exit 1; }
