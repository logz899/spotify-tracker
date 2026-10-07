package database

import (
	"context"
	"testing"
	"time"
)

func TestSyncLockAllowsOnlyOneOwnerAndCanBeReacquired(t *testing.T) {
	firstDB := newIntegrationDB(t)
	t.Cleanup(firstDB.Close)
	secondDB := newIntegrationDB(t)
	t.Cleanup(secondDB.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	acquired, releaseFirst, err := firstDB.TrySyncLock(ctx)
	if err != nil {
		t.Fatalf("first TrySyncLock() error = %v", err)
	}
	if !acquired {
		t.Fatal("first TrySyncLock() acquired = false, want true")
	}

	started := time.Now()
	secondAcquired, releaseSecond, err := secondDB.TrySyncLock(ctx)
	if err != nil {
		t.Fatalf("concurrent TrySyncLock() error = %v", err)
	}
	if secondAcquired {
		_ = releaseSecond()
		t.Fatal("concurrent TrySyncLock() acquired = true, want false")
	}
	if time.Since(started) > time.Second {
		t.Fatal("concurrent TrySyncLock() blocked instead of returning immediately")
	}

	if err := releaseFirst(); err != nil {
		t.Fatalf("release first lock: %v", err)
	}
	acquired, releaseSecond, err = secondDB.TrySyncLock(ctx)
	if err != nil {
		t.Fatalf("TrySyncLock() after release error = %v", err)
	}
	if !acquired {
		t.Fatal("TrySyncLock() after release acquired = false, want true")
	}
	if err := releaseSecond(); err != nil {
		t.Fatalf("release second lock: %v", err)
	}
}

func TestSyncLockReleaseAfterCancellationDoesNotLeakConnection(t *testing.T) {
	db := newIntegrationDB(t)
	t.Cleanup(db.Close)
	ctx, cancel := context.WithCancel(context.Background())

	acquired, release, err := db.TrySyncLock(ctx)
	if err != nil {
		t.Fatalf("TrySyncLock() error = %v", err)
	}
	if !acquired {
		t.Fatal("TrySyncLock() acquired = false, want true")
	}
	cancel()
	if err := release(); err != nil {
		t.Fatalf("release after cancellation: %v", err)
	}

	retryCtx, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer retryCancel()
	acquired, release, err = db.TrySyncLock(retryCtx)
	if err != nil {
		t.Fatalf("TrySyncLock() after canceled owner error = %v", err)
	}
	if !acquired {
		t.Fatal("TrySyncLock() after canceled owner acquired = false, want true")
	}
	if err := release(); err != nil {
		t.Fatalf("release reacquired lock: %v", err)
	}
}
