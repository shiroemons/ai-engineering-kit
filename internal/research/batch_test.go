package research

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shiroemons/ai-engineering-kit/internal/kb"
)

// The test binary doubles as OpenCode. No model requests or remote Git writes.
func TestOpenCodeProcess(t *testing.T) {
	if os.Getenv("RESEARCH_TEST_HELPER") != "1" {
		return
	}
	prompt := os.Args[len(os.Args)-1]
	domainText, _, _ := strings.Cut(strings.Split(prompt, "Assigned domain: ")[1], " ")
	mode := os.Getenv("RESEARCH_TEST_MODE")
	if mode == "rate" {
		fmt.Println(`{"type":"error","error":{"status":429,"message":"quota exceeded"}}`)
		time.Sleep(time.Minute)
		os.Exit(2)
	}
	state := os.Getenv("RESEARCH_TEST_STATE")
	if err := os.WriteFile(filepath.Join(state, domainText+".ready"), []byte("ready"), 0600); err != nil {
		panic(err)
	}
	if mode == "wait" {
		cmd := exec.Command("sleep", "60")
		if err := cmd.Start(); err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(state, "child-pid"), []byte(fmt.Sprint(cmd.Process.Pid)), 0600); err != nil {
			panic(err)
		}
		_ = cmd.Wait()
		os.Exit(0)
	}
	// A sequential implementation cannot pass this barrier.
	if mode == "parallel" || mode == "overlap" {
		count := 2
		if mode == "overlap" {
			count = 3
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			files, err := filepath.Glob(filepath.Join(state, "*.ready"))
			if err != nil {
				panic(err)
			}
			if len(files) == count {
				break
			}
			if time.Now().After(deadline) {
				os.Exit(3)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if mode == "partial" && domainText == "three" {
		os.Exit(1)
	}
	if mode == "early" {
		text, err := json.Marshal(map[string]any{
			"type": "text",
			"part": map[string]string{"text": "TOPIC_SELECTED: " + domainText + " research\nPROGRESS: sources-verified"},
		})
		if err != nil {
			panic(err)
		}
		fmt.Println(string(text))
		os.Exit(0)
	}
	if !strings.HasPrefix(prompt, "DRY RUN:") {
		id := domainText + "-research"
		meta := kb.Metadata{ID: id, Title: id, Kind: "knowledge", Technology: "go", Version: "Go 1.27.1", Tags: []string{"research-domain:" + domainText}, Sources: []kb.SourceRef{{ID: "go-context-docs", URL: "https://pkg.go.dev/context", Type: "official_docs"}}, RetrievedAt: "2026-09-23", ExpiresAt: "2026-12-22", Trust: "official", Status: "active", Evals: []string{"evals/knowledge/" + domainText + ".json"}}
		data, err := json.Marshal(meta)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile("knowledge/go/"+id+".md", []byte("---\n"+string(data)+"\n---\n\n# "+id+"\n\nVerified context behavior.\n"), 0644); err != nil {
			panic(err)
		}
		eval := fmt.Sprintf(`{"cases":[{"name":%q,"query":%q,"expected_ids":[%q]}]}`, id, id, id)
		if mode == "bad-eval" {
			eval = `{"cases":[{"name":"missing","query":"nonexistenttoken","expected_ids":["missing"]}]}`
		}
		if err := os.WriteFile("evals/knowledge/"+domainText+".json", []byte(eval), 0644); err != nil {
			panic(err)
		}
		if mode == "forbidden" {
			if err := os.WriteFile("config/unwanted.json", []byte("{}"), 0644); err != nil {
				panic(err)
			}
		}
		if mode == "conflict" {
			source := "sources/catalog/go-context-docs.json"
			data, err := os.ReadFile(source)
			if err != nil {
				panic(err)
			}
			var record map[string]any
			if err := json.Unmarshal(data, &record); err != nil {
				panic(err)
			}
			record["purpose"] = domainText
			data, err = json.Marshal(record)
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(source, data, 0644); err != nil {
				panic(err)
			}
		}
	}
	text, err := json.Marshal(map[string]any{"type": "text", "part": map[string]string{"text": "TOPIC_SELECTED: " + domainText + " research\nPROGRESS: sources-verified\nTOPIC: " + domainText + " research"}})
	if err != nil {
		panic(err)
	}
	fmt.Println(string(text))
	os.Exit(0)
}

func fixture(t *testing.T, mode string) (string, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"config", "knowledge", "patterns", "modules", "sources", "evals"} {
		if err := os.CopyFS(filepath.Join(root, dir), os.DirFS(filepath.Join(repo, dir))); err != nil {
			t.Fatal(err)
		}
	}
	c, err := readConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	c.Domains = []domain{{ID: "one", Technologies: []string{"go"}}, {ID: "two", Technologies: []string{"go"}}, {ID: "three", Technologies: []string{"go"}}}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "config/research.json"), data, 0644)
	writeFile(t, filepath.Join(root, ".gitignore"), []byte("/.workbench/\n/rag/\n"), 0644)
	for _, args := range [][]string{{"init", "--initial-branch=main"}, {"add", "."}, {"-c", "user.name=Research Test", "-c", "user.email=research@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "test fixture"}} {
		if _, err := git(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	state := t.TempDir()
	bin := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RESEARCH_TEST_BIN", exe)
	t.Setenv("RESEARCH_TEST_HELPER", "1")
	t.Setenv("RESEARCH_TEST_MODE", mode)
	t.Setenv("RESEARCH_TEST_STATE", state)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeFile(t, filepath.Join(bin, "opencode"), []byte("#!/bin/sh\nexec \"$RESEARCH_TEST_BIN\" -test.run='^TestOpenCodeProcess$' -- \"$@\"\n"), 0755)
	return root, state
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func TestParallelResearchAndIsolation(t *testing.T) {
	root, state := fixture(t, "parallel")
	var out bytes.Buffer
	if err := Run(t.Context(), root, state, []string{"opencode/muse", "opencode/mimo"}, false, &out); err != nil {
		t.Fatalf("%v\n%s", err, &out)
	}
	if !strings.Contains(out.String(), "BATCH: complete") {
		t.Fatal(out.String())
	}
	for _, id := range []string{"two", "three"} {
		if _, err := os.Stat(filepath.Join(root, "knowledge/go/"+id+"-research.md")); err != nil {
			t.Fatal(err)
		}
	}
	trees, err := git(t.Context(), root, "worktree", "list", "--porcelain")
	if err != nil || bytes.Count(trees, []byte("worktree ")) != 1 {
		t.Fatalf("worktrees leaked: %s %v", trees, err)
	}
	if _, _, err := indexed(root); err != nil {
		t.Fatal(err)
	}
}

func TestPartialAndConflictingResults(t *testing.T) {
	for _, mode := range []string{"partial", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			root, state := fixture(t, mode)
			var out bytes.Buffer
			if err := Run(t.Context(), root, state, []string{"opencode/muse", "opencode/mimo"}, false, &out); err != nil {
				t.Fatalf("%v\n%s", err, &out)
			}
			if !strings.Contains(out.String(), "BATCH: partial") {
				t.Fatal(out.String())
			}
			if _, err := os.Stat(filepath.Join(root, "knowledge/go/two-research.md")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(root, "knowledge/go/three-research.md")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed worker was integrated")
			}
			if _, _, err := indexed(root); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInvalidResultsAndCooldown(t *testing.T) {
	for _, mode := range []string{"forbidden", "bad-eval", "rate"} {
		t.Run(mode, func(t *testing.T) {
			root, state := fixture(t, mode)
			var out bytes.Buffer
			if err := Run(t.Context(), root, state, []string{"opencode/muse"}, false, &out); err == nil {
				t.Fatal("expected failure")
			}
			if err := requireClean(t.Context(), root); err != nil {
				t.Fatal(err)
			}
			if mode == "rate" {
				if err := Run(t.Context(), root, state, []string{"opencode/muse"}, false, &out); !errors.Is(err, errRateLimit) {
					t.Fatalf("cooldown not respected: %v", err)
				}
			}
		})
	}
}

func TestScheduledAndContinuousBatchesOverlap(t *testing.T) {
	root, state := fixture(t, "overlap")
	// Two integration worktrees model the two independent entry points.
	roots := []string{filepath.Join(t.TempDir(), "scheduled"), filepath.Join(t.TempDir(), "continuous")}
	for _, path := range roots {
		if _, err := git(t.Context(), root, "worktree", "add", "--detach", path, "HEAD"); err != nil {
			t.Fatal(err)
		}
	}
	models := [][]string{{"opencode/muse", "opencode/mimo"}, {"opencode/muse"}}
	errs := make([]error, 2)
	var outputs [2]bytes.Buffer
	var wg sync.WaitGroup
	for i := range roots {
		wg.Go(func() { errs[i] = Run(t.Context(), roots[i], state, models[i], false, &outputs[i]) })
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("batch %d: %v\n%s", i, err, &outputs[i])
		}
	}
	if err := requireClean(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	ready, err := filepath.Glob(filepath.Join(state, "*.ready"))
	if err != nil || len(ready) != 3 {
		t.Fatalf("domains not isolated: %v %v", ready, err)
	}
	for _, path := range roots {
		if _, err := git(t.Context(), root, "worktree", "remove", "--force", path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlanCoverageAndRecency(t *testing.T) {
	domains := []domain{{ID: "one", Technologies: []string{"go"}}, {ID: "two"}, {ID: "three"}}
	docs := []kb.Document{{Path: "knowledge/a.md", Metadata: kb.Metadata{Kind: "knowledge", Technology: "go"}}, {Path: "knowledge/b.md", Metadata: kb.Metadata{Kind: "knowledge", Tags: []string{"research-domain:two"}}}}
	got := plan(domains, docs, []string{"knowledge/b.md"})
	var ids []string
	for _, d := range got {
		ids = append(ids, d.ID)
	}
	if !slices.Equal(ids, []string{"three", "one", "two"}) {
		t.Fatal(ids)
	}
}
