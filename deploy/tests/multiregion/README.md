# Multi-region Docker integration test

This fixture models the production topology with one shared PostgreSQL server:

- `jp-01` and `jp-02` share `redis-japan` and are load-balanced by `jp.sub2api.test`.
- `us-01` uses `redis-us` and is exposed as `us.sub2api.test`.
- `tw-01` uses `redis-taiwan` and is exposed as `tw.sub2api.test`.
- US and Taiwan global background tasks are disabled; Japan uses PostgreSQL advisory locks.
- Auth-cache invalidations are consumed once by each Redis scope, so disabling a key
  converges across the three independent Redis data sets.

The runner creates an isolated Compose project with temporary volumes and removes it on exit.
The real upstream credential is required at runtime and is never stored in this directory:

```bash
HCTOPUP_API_KEY='...' ./deploy/tests/multiregion/run.sh
```

Set `KEEP_ENV=1` to keep a failed environment for inspection. The script prints the Compose
project name and the dynamically allocated ingress port.
