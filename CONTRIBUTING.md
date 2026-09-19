# Contributing

## Branches

- `main` is the production branch. Direct pushes are prohibited.
- `dev` is the integration and development branch.
- Feature and fix branches must start from `dev`, using `feat/...`, `fix/...`, `docs/...`, or `chore/...`.
- Production changes follow `feature/fix -> dev -> main`. A pull request is required at every boundary.

## Pull requests

Every pull request must describe the scope, tests, deployment impact, and rollback plan. Keep unrelated changes out of the same PR. Database migrations must be backward compatible with the previous release and documented in the PR.

CI must be green before review. At least one CODEOWNER review is required. The author must resolve review comments; force-pushing is allowed only on the contributor branch, never on `dev` or `main`.

## Documentation synchronization

Development, fixes, upstream merges, reviews, CI diagnosis, releases, and deployments must keep the relevant documents in `docs/hcai-dev/` current. Follow the mapping and completion gate in [开发规范](docs/hcai-dev/开发规范.md).

Include applicable documentation changes in the same task and PR. Record confirmed behavior, evidence, validation status, and unresolved risks; preserve historical baselines. New HCAI records use Chinese filenames under `docs/hcai-dev/`. Update links when paths change.

Reviewers must verify documentation consistency before approval. If no update is needed, the PR or handoff must explain which documents were checked and why they are unaffected. Missing required updates blocks completion and merge readiness. This is a review requirement, not a currently automated CI check. Documentation updates do not authorize business-code fixes or remote actions during review-only tasks; explicit no-edit instructions still take precedence.

## Commits

Use the repository convention:

```text
hcai(<type>): <imperative summary>
```

Allowed types include `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, and `merge`. Keep each commit focused and do not commit secrets or generated production artifacts.

## Required local checks

```bash
make test
make build
```

For a narrow change, also run the relevant targeted backend or frontend test and include the command in the PR.

## Release flow

Merge approved work into `dev`, deploy `dev` to the staging environment, and verify health checks, migrations, background jobs, and multi-node behavior. Only then open a release PR from `dev` to `main`. Production deployment is performed from `main` after the release PR is merged and tagged.
