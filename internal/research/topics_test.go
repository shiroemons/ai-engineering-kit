package research

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestTopicLeaseNormalizesCaseAndPunctuation(t *testing.T) {
	state := t.TempDir()
	d := domain{ID: "frontend"}
	lease, err := reserveTopic(state, d, "React form-errors")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := releaseTopic(lease); err != nil {
			t.Errorf("releaseTopic() error = %v", err)
		}
	})
	if _, err := reserveTopic(state, d, "react form errors!"); !errors.Is(err, errTopicAlreadyActive) {
		t.Fatalf("normalized duplicate topic was accepted: %v", err)
	}
}

func TestTopicSelectionLockWaitsCancelsAndReleases(t *testing.T) {
	state := t.TempDir()
	releaseFirst, err := acquireTopicSelection(t.Context(), state)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := acquireTopicSelection(ctx, state); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting selection did not honor cancellation: %v", err)
	}
	if err := releaseFirst(); err != nil {
		t.Fatal(err)
	}
	releaseSecond, err := acquireTopicSelection(t.Context(), state)
	if err != nil {
		t.Fatalf("selection lock remained held after release: %v", err)
	}
	if err := releaseSecond(); err != nil {
		t.Fatal(err)
	}
}

func TestReadActiveTopicsPrunesDeadAndExpiredLeases(t *testing.T) {
	state := t.TempDir()
	directory := filepath.Join(state, "topics")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	deadPID := os.Getpid() + 1000
	if err := syscall.Kill(deadPID, 0); !errors.Is(err, syscall.ESRCH) {
		t.Skipf("cannot find a dead test PID: %v", err)
	}
	leases := map[string]activeTopic{
		"dead.json":    {Topic: "dead topic", Domain: "one", PID: deadPID, StartedAt: now},
		"expired.json": {Topic: "expired topic", Domain: "two", PID: os.Getpid(), StartedAt: now.Add(-maxTopicLeaseAge - time.Second)},
		"active.json":  {Topic: "active topic", Domain: "three", PID: os.Getpid(), StartedAt: now},
	}
	for name, lease := range leases {
		data, err := json.Marshal(lease)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	release, err := acquireTopicSelection(t.Context(), state)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	}()
	active, err := readActiveTopics(state)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].Topic != "active topic" {
		t.Fatalf("unexpected active leases after pruning: %+v", active)
	}
	for _, name := range []string{"dead.json", "expired.json"} {
		if _, err := os.Stat(filepath.Join(directory, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stale lease %s was retained: %v", name, err)
		}
	}
}

func TestReadTopicLeaseSkipsConcurrentRelease(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "active.json")
	data, err := json.Marshal(activeTopic{Topic: "finished topic", Domain: "one", PID: os.Getpid(), StartedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, found, err := readTopicLease(path, entries[0].Name()); err != nil || found {
		t.Fatalf("lease released after listing should be skipped: found=%t err=%v", found, err)
	}
}
