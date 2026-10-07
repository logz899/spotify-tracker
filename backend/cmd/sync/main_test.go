package main

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"spotify-backend/internal/services"
)

func TestRunPrintsCountsAndReturnsSuccessForPartialResults(t *testing.T) {
	var output bytes.Buffer
	job := &fakeSyncJob{summary: services.SyncSummary{Attempted: 3, Succeeded: 2, Failed: 1}}

	err := run(context.Background(), dependencies{job: job, output: &output})

	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if output.String() != "attempted=3 succeeded=2 failed=1\n" {
		t.Fatalf("output = %q", output.String())
	}
	if commandExitCode(err) != 0 {
		t.Fatalf("commandExitCode(nil) = %d, want 0", commandExitCode(err))
	}
}

func TestRunReturnsNonZeroForUnavailableLock(t *testing.T) {
	job := &fakeSyncJob{err: services.ErrSyncAlreadyRunning}

	err := run(context.Background(), dependencies{job: job, output: &bytes.Buffer{}})

	if !errors.Is(err, services.ErrSyncAlreadyRunning) {
		t.Fatalf("run() error = %v, want ErrSyncAlreadyRunning", err)
	}
	if commandExitCode(err) != 1 {
		t.Fatalf("commandExitCode(error) = %d, want 1", commandExitCode(err))
	}
}

func TestRunReturnsNonZeroForTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	job := &fakeSyncJob{waitForContext: true}

	err := run(ctx, dependencies{job: job, output: &bytes.Buffer{}})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run() error = %v, want context.Canceled", err)
	}
	if commandExitCode(err) != 1 {
		t.Fatalf("commandExitCode(error) = %d, want 1", commandExitCode(err))
	}
}

func TestRunReturnsNonZeroForFatalJobError(t *testing.T) {
	fatalErr := errors.New("database unavailable")
	job := &fakeSyncJob{err: fatalErr}

	err := run(context.Background(), dependencies{job: job, output: &bytes.Buffer{}})

	if !errors.Is(err, fatalErr) {
		t.Fatalf("run() error = %v, want fatal error", err)
	}
	if commandExitCode(err) != 1 {
		t.Fatalf("commandExitCode(error) = %d, want 1", commandExitCode(err))
	}
}

type fakeSyncJob struct {
	summary        services.SyncSummary
	err            error
	waitForContext bool
}

func (f *fakeSyncJob) SyncAllUsers(ctx context.Context) (services.SyncSummary, error) {
	if f.waitForContext {
		<-ctx.Done()
		return services.SyncSummary{}, ctx.Err()
	}
	return f.summary, f.err
}
