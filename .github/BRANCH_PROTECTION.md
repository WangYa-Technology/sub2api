# GitHub branch protection

Configure these repository rules for both `dev` and `main` (Settings -> Rules -> Rulesets, or equivalent branch protection rules):

- Require a pull request before merging; require one approval from a CODEOWNER.
- Require branches to be up to date before merging.
- Require these status checks: `Backend checks`, `Frontend checks`.
- Require conversation resolution and dismiss stale approvals after new commits.
- Do not allow force pushes or branch deletion.
- Restrict direct pushes to `main`; allow only the release PR from `dev`.
- Require signed commits if repository policy supports it.

`main` should use the stricter production rule. `dev` may allow maintainers to push emergency CI fixes, but normal changes still go through pull requests.
