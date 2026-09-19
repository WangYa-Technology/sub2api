# HCAI repository rules

## Documentation synchronization

- Before development, upstream merging, review, release, or deployment, read the relevant documents in `docs/hcai-dev/`, especially `开发规范.md` section 5.
- Keep the corresponding documents synchronized with confirmed changes, decisions, review findings, validation results, and known risks in the same task and PR. Update affected links when moving or renaming documents.
- For review-only tasks, update documentation with findings and evidence, but do not implement business-code fixes or perform remote writes without authorization. If the user explicitly requests no file edits, report the required documentation updates instead.
- Do not rewrite historical baseline snapshots as if they describe a new version. Add a dated, versioned record or regenerate an explicitly identified new baseline, preserving historical evidence.
- Before handoff, list the documents updated. If no update is applicable, explicitly state why. Missing required documentation blocks completion and merge readiness; this is a review rule, not an existing automated CI check.
- New HCAI development records belong in `docs/hcai-dev/` with Chinese filenames. Never include secrets, production credentials, or personal data.
