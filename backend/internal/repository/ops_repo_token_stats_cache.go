package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Two-key advisory lock namespace: OPS / token stats. All report filters share
// one gate so different filters cannot create concurrent detail scans either.
const tokenStatsComputeLockSQL = `SELECT pg_try_advisory_xact_lock(1330664276, 1)`

type tokenStatsRowQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func readTokenStatsCache(ctx context.Context, q tokenStatsRowQueryer, key string) (*service.OpsOpenAITokenStatsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var raw []byte
	err := q.QueryRowContext(ctx, `SELECT payload FROM ops_token_stats_cache WHERE cache_key = $1 AND expires_at > statement_timestamp()`, key).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out service.OpsOpenAITokenStatsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *opsRepository) GetOpenAITokenStats(ctx context.Context, filter *service.OpsOpenAITokenStatsFilter) (*service.OpsOpenAITokenStatsResponse, error) {
	if r == nil || r.db == nil || filter == nil {
		return nil, infraerrors.ServiceUnavailable("OPS_STATS_CACHE_UNAVAILABLE", "Token statistics unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 58*time.Second)
	defer cancel()
	key := service.OpsTokenStatsCacheKey(filter)
	out, err := readTokenStatsCache(ctx, r.db, key)
	if err != nil || out != nil {
		return out, err
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		out, err := r.computeTokenStats(ctx, key, filter)
		if !errors.Is(err, errTokenStatsBusy) {
			return out, err
		}
		// No connection or transaction is held while waiting for another instance.
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, errTokenStatsBusy
			}
			return nil, ctx.Err()
		case <-ticker.C:
		}
		out, err = readTokenStatsCache(ctx, r.db, key)
		if err != nil || out != nil {
			return out, err
		}
	}
}

var errTokenStatsBusy = infraerrors.ServiceUnavailable("OPS_STATS_BUSY", "Token statistics are being calculated; retry shortly")

func (r *opsRepository) computeTokenStats(ctx context.Context, key string, filter *service.OpsOpenAITokenStatsFilter) (*service.OpsOpenAITokenStatsResponse, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Server-side limits also release an abandoned transaction/lock if its client
	// disappears. Transaction pooling is safe: no session-level lock/state is used.
	if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = '55s'; SET LOCAL idle_in_transaction_session_timeout = '60s'; SET LOCAL lock_timeout = '1s'`); err != nil {
		return nil, err
	}
	var acquired bool
	if err := tx.QueryRowContext(ctx, tokenStatsComputeLockSQL).Scan(&acquired); err != nil {
		return nil, err
	}
	if !acquired {
		return nil, errTokenStatsBusy
	}
	// READ COMMITTED gives this statement a fresh snapshot after another owner
	// committed, closing the cache-miss/acquire race.
	out, err := readTokenStatsCache(ctx, tx, key)
	if err != nil {
		return nil, err
	}
	if out != nil {
		return out, nil
	}
	out, err = queryOpenAITokenStats(ctx, tx, filter)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO ops_token_stats_cache(cache_key,payload,expires_at)
 VALUES($1,$2::jsonb,clock_timestamp()+interval '120 seconds')
 ON CONFLICT(cache_key) DO UPDATE SET payload=EXCLUDED.payload,expires_at=EXCLUDED.expires_at`, key, string(raw)); err != nil {
		return nil, err
	}
	// Global compute lock serializes cleanup too; delete a bounded indexed batch.
	if _, err := tx.ExecContext(ctx, `DELETE FROM ops_token_stats_cache WHERE cache_key IN
 (SELECT cache_key FROM ops_token_stats_cache WHERE expires_at <= statement_timestamp() ORDER BY expires_at LIMIT 1000)`); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}
