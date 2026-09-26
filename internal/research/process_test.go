package research

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCancellationStopsStandaloneChildren(t *testing.T) {
	root, state := fixture(t, "wait")
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	done := make(chan error, 1)
	go func() {
		_, err := runOpenCode(ctx, root, "opencode/muse", prompt(domain{ID: "two"}, false), cancel)
		done <- err
	}()
	var childPID int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(filepath.Join(state, "child-pid")); err == nil {
			childPID, err = strconv.Atoi(string(data))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID == 0 {
		t.Fatal("child did not start")
	}
	cancel(context.DeadlineExceeded)
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OpenCode did not stop")
	}
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(childPID, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("standalone descendant still running")
}

func TestEventsIgnoreToolTextAndRecognizeProviderErrors(t *testing.T) {
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	e := &events{Cancel: cancel, StopBatch: cancel}
	for _, line := range []string{
		`{"type":"tool_use","part":{"text":"TOPIC: fake quota exceeded 429"}}`,
		`{"type":"text","part":{"text":"TOPIC: actual topic"}}`,
	} {
		if _, err := e.Write([]byte(line + "\n")); err != nil {
			t.Fatal(err)
		}
	}
	if e.Topic != "actual topic" || ctx.Err() != nil {
		t.Fatalf("%+v", e)
	}
	if _, err := e.Write([]byte(`{"type":"error","error":{"status":429}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(ctx), errRateLimit) {
		t.Fatal("rate limit not propagated")
	}
}

func TestDryRunIsolatedAndConfigRejected(t *testing.T) {
	root, state := fixture(t, "normal")
	var out strings.Builder
	if err := Run(t.Context(), root, state, []string{"opencode/muse"}, true, &out); err != nil {
		t.Fatal(err)
	}
	if err := requireClean(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "config/research.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(string(data), `"topics_per_run":2`, `"topics_per_run":0`, 1), strings.Replace(string(data), `"worker_timeout_minutes":45`, `"worker_timeout_minutes":0`, 1)} {
		writeFile(t, filepath.Join(root, "config/research.json"), []byte(bad), 0644)
		if _, err := readConfig(root); err == nil {
			t.Fatal("invalid config accepted")
		}
	}
}

func TestSharedCooldownStopsActiveBatch(t *testing.T) {
	root, state := fixture(t, "wait")
	var out strings.Builder
	done := make(chan error, 1)
	go func() { done <- Run(t.Context(), root, state, []string{"opencode/muse"}, false, &out) }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(state, "child-pid")); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(state, "child-pid")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(state, "cooldown-until"), []byte(strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)+"\n"), 0600)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("active batch did not fail on shared cooldown")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("active batch ignored shared cooldown")
	}
	if err := requireClean(t.Context(), root); err != nil {
		t.Fatal(err)
	}
}
