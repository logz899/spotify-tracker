package database

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// spotifySyncAdvisoryLockKey is reserved for the one-shot Spotify sync job.
const spotifySyncAdvisoryLockKey int64 = 0x53504f54494659

func (db *DB) TrySyncLock(ctx context.Context) (bool, func() error, error) {
	conn, err := db.pool.Acquire(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("acquire connection for sync lock: %w", err)
	}

	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", spotifySyncAdvisoryLockKey).Scan(&acquired); err != nil {
		conn.Release()
		return false, nil, fmt.Errorf("try sync advisory lock: %w", err)
	}
	if !acquired {
		conn.Release()
		return false, func() error { return nil }, nil
	}

	var once sync.Once
	var releaseErr error
	release := func() error {
		once.Do(func() {
			unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var unlocked bool
			if err := conn.QueryRow(unlockCtx, "SELECT pg_advisory_unlock($1)", spotifySyncAdvisoryLockKey).Scan(&unlocked); err != nil {
				rawConn := conn.Hijack()
				releaseErr = errors.Join(fmt.Errorf("unlock Spotify sync: %w", err), rawConn.Close(context.Background()))
				return
			}
			conn.Release()
			if !unlocked {
				releaseErr = errors.New("spotify sync advisory lock was not owned by its connection")
			}
		})
		return releaseErr
	}
	return true, release, nil
}
