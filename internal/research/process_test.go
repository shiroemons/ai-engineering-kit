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
		d := domain{ID: "two"}
		_, err := runOpenCodeTopicSelection(ctx, root, "opencode/muse", topicSelectionPrompt(d, false), cancel, d, func(Progress) error { return nil })
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
	if e.Topic != "actual topic" || !e.TopicFinal || ctx.Err() != nil {
		t.Fatalf("%+v", e)
	}
	if _, err := e.Write([]byte(`{"type":"error","error":{"status":429}}` + "\n")); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(context.Cause(ctx), errRateLimit) {
		t.Fatal("rate limit not propagated")
	}
}

func TestTopicSelectionDoesNotCompleteResearch(t *testing.T) {
	_, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	e := &events{Cancel: cancel, StopBatch: cancel}

	e.text("TOPIC_SELECTED: react form errors\nPROGRESS: sources-verified\n")
	if e.Topic != "react form errors" || !e.TopicSelected || e.TopicFinal || !e.SourcesVerified {
		t.Fatalf("topic selection incorrectly completed research: %+v", e)
	}
	e.text("PROGRESS: knowledge-written\n")
	if !e.KnowledgeWritten {
		t.Fatalf("knowledge completion milestone was not recorded: %+v", e)
	}

	e.text("TOPIC: react form errors\n")
	if e.Topic != "react form errors" || !e.TopicFinal {
		t.Fatalf("final topic marker was not accepted: %+v", e)
	}
}

func TestPromptsSeparateResearchPhases(t *testing.T) {
	d := domain{ID: "frontend", Name: "Frontend", Technologies: []string{"react"}}
	selection := topicSelectionPrompt(d, false)
	if !strings.Contains(selection, "PHASE: TOPIC_SELECTION") || !strings.Contains(selection, "Do not emit TOPIC or any PROGRESS marker") || !strings.Contains(selection, "Stop after the selected topic") {
		t.Fatalf("selection phase is not bounded: %s", selection)
	}

	topic := "react controlled form errors"
	sources := sourceVerificationPrompt(d, topic)
	if !strings.Contains(sources, "PHASE: SOURCE_VERIFICATION") || !strings.Contains(sources, "Selected topic: "+topic) || !strings.Contains(sources, "Do not edit files") || !strings.Contains(sources, "PROGRESS: sources-verified") {
		t.Fatalf("source phase is not bounded to verification: %s", sources)
	}

	knowledge := knowledgeWritingPrompt(d, topic, "SOURCE: https://example.test/spec")
	if !strings.Contains(knowledge, "PHASE: KNOWLEDGE_WRITING") || !strings.Contains(knowledge, "Selected topic: "+topic) || !strings.Contains(knowledge, "https://example.test/spec") || !strings.Contains(knowledge, "PROGRESS: knowledge-written") {
		t.Fatalf("knowledge phase does not receive the verified evidence: %s", knowledge)
	}

	evalWriting := evalWritingPrompt(d, topic)
	if !strings.Contains(evalWriting, "PHASE: EVAL_WRITING") || !strings.Contains(evalWriting, "Selected topic: "+topic) || !strings.Contains(evalWriting, "evals/knowledge/frontend.json") || !strings.Contains(evalWriting, "runner performs deterministic artifact and eval validation") || strings.Contains(evalWriting, "emit TOPIC:") {
		t.Fatalf("eval phase does not preserve the topic or delegate completion checks to the runner: %s", evalWriting)
	}
}

func TestRunOpenCodeEvalWritingDoesNotRequireTopicMarker(t *testing.T) {
	root, _ := fixture(t, "no-final-topic")
	d := domain{ID: "two", Name: "two", Technologies: []string{"go"}}
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)

	if err := runOpenCodeEvalWriting(ctx, root, "opencode/muse", evalWritingPrompt(d, "two research"), cancel, d, nil); err != nil {
		t.Fatalf("eval writing was rejected without a model-authored topic marker: %v", err)
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
