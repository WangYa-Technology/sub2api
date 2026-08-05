package service

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

type persistentDBAdvisoryLocks struct {
	conn *sql.Conn
	held map[int64]string
}

var persistentAdvisoryLocks = struct {
	sync.Mutex
	byDB map[*sql.DB]*persistentDBAdvisoryLocks
}{byDB: make(map[*sql.DB]*persistentDBAdvisoryLocks)}

func hashAdvisoryLockID(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int64(h.Sum64())
}

func tryAcquireDBAdvisoryLock(ctx context.Context, db *sql.DB, lockID int64) (func(), bool) {
	release, acquired, _ := tryAcquireDBAdvisoryLockWithError(ctx, db, lockID)
	return release, acquired
}

func tryAcquireDBAdvisoryLockWithError(ctx context.Context, db *sql.DB, lockID int64) (func(), bool, error) {
	if db == nil {
		return nil, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("open advisory-lock connection: %w", err)
	}

	acquired := false
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired); err != nil {
		_ = conn.Close()
		return nil, false, fmt.Errorf("query advisory lock: %w", err)
	}
	if !acquired {
		_ = conn.Close()
		return nil, false, nil
	}

	release := func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, "SELECT pg_advisory_unlock($1)", lockID)
		_ = conn.Close()
	}
	return release, true, nil
}

// tryAcquirePersistentDBAdvisoryLock elects a stable leader for the lifetime of
// a service instance. The winning connection remains reserved between task
// cycles; this is required when regional tickers are not aligned.
func tryAcquirePersistentDBAdvisoryLock(ctx context.Context, db *sql.DB, lockID int64, owner string) (func(), bool, error) {
	if db == nil || !backgroundTaskLeaderEligible(owner) {
		return nil, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	persistentAdvisoryLocks.Lock()
	defer persistentAdvisoryLocks.Unlock()
	state := persistentAdvisoryLocks.byDB[db]
	if state != nil {
		if heldOwner, ok := state.held[lockID]; ok && heldOwner != owner {
			return nil, false, nil
		}
		if heldOwner, ok := state.held[lockID]; ok && heldOwner == owner {
			if err := state.conn.PingContext(ctx); err == nil {
				return func() {}, true, nil
			}
			delete(persistentAdvisoryLocks.byDB, db)
			_ = state.conn.Close()
			state = nil
		}
	}

	if state == nil {
		conn, err := db.Conn(ctx)
		if err != nil {
			return nil, false, fmt.Errorf("open persistent advisory-lock connection: %w", err)
		}
		state = &persistentDBAdvisoryLocks{conn: conn, held: make(map[int64]string)}
		persistentAdvisoryLocks.byDB[db] = state
	}
	acquired := false
	if err := state.conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired); err != nil {
		delete(persistentAdvisoryLocks.byDB, db)
		_ = state.conn.Close()
		return nil, false, fmt.Errorf("query persistent advisory lock: %w", err)
	}
	if !acquired {
		if len(state.held) == 0 {
			delete(persistentAdvisoryLocks.byDB, db)
			_ = state.conn.Close()
		}
		return nil, false, nil
	}
	state.held[lockID] = owner
	return func() {}, true, nil
}

// ReleasePersistentLeaderLocks releases all stable task leadership held through
// db. It is called after background services stop and before the DB pool closes.
func ReleasePersistentLeaderLocks(db *sql.DB) {
	if db == nil {
		return
	}
	persistentAdvisoryLocks.Lock()
	state := persistentAdvisoryLocks.byDB[db]
	delete(persistentAdvisoryLocks.byDB, db)
	persistentAdvisoryLocks.Unlock()
	if state == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, _ = state.conn.ExecContext(ctx, "SELECT pg_advisory_unlock_all()")
	cancel()
	_ = state.conn.Close()
}
