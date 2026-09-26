package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var testDate = time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
var retrievedAtField = regexp.MustCompile(`"retrieved_at":\s*"\d{4}-\d{2}-\d{2}"`)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		bad  bool
	}{
		{"empty shows help", nil, false},
		{"help", []string{"help"}, false},
		{"command help", []string{"search", "--help"}, false},
		{"flags interspersed", []string{"--root", "/tmp/path with spaces", "search", "context", "--json", "cancellation", "--as-of=2026-09-23", "--limit", "3"}, false},
		{"literal query", []string{"search", "--", "--negative"}, false},
		{"missing command", []string{"--json"}, true},
		{"unknown command", []string{"destroy"}, true},
		{"unknown flag", []string{"search", "context", "--wat"}, true},
		{"missing query", []string{"search", "--json"}, true},
		{"empty query", []string{"search", "  "}, true},
		{"missing root", []string{"search", "context", "--root"}, true},
		{"empty root", []string{"validate", "--root="}, true},
		{"flag as root", []string{"validate", "--root", "--json"}, true},
		{"invalid day", []string{"validate", "--as-of=2026-02-30"}, true},
		{"zero limit", []string{"search", "context", "--limit=0"}, true},
		{"negative limit", []string{"search", "context", "--limit", "-2"}, true},
		{"bad limit", []string{"search", "context", "--limit=many"}, true},
		{"limit on other command", []string{"stats", "--limit=2"}, true},
		{"extra positional", []string{"index", "context"}, true},
		{"boolean value", []string{"validate", "--json=false"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseArgs(tt.args, testDate)
			if (err != nil) != tt.bad {
				t.Fatalf("parseArgs() error = %v; want error %v", err, tt.bad)
			}
			if tt.name == "flags interspersed" && (opts.query != "context cancellation" || opts.root != "/tmp/path with spaces" || !opts.json || opts.limit != 3 || !opts.now.Equal(testDate)) {
				t.Fatalf("unexpected parsed options: %+v", opts)
			}
		})
	}
}

func TestFixCheckDetectsChangesAndToolFailures(t *testing.T) {
	script := filepath.Join(repositoryRoot(t), "scripts", "check-fix.sh")
	for _, scenario := range []struct {
		name   string
		output string
		status string
		want   int
	}{
		{"clean", "", "0", 0},
		{"changes", "diff --git old.go new.go", "0", 1},
		{"tool failure", "package loading failed", "7", 7},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			bin := t.TempDir()
			stub := "#!/bin/sh\n[ \"$*\" = 'fix -diff ./...' ] || exit 98\nprintf '%s' \"$FIX_OUTPUT\"\nexit \"$FIX_EXIT\"\n"
			if err := os.WriteFile(filepath.Join(bin, "go"), []byte(stub), 0o755); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "sh", script)
			cmd.Env = append(os.Environ(), "PATH="+bin, "FIX_OUTPUT="+scenario.output, "FIX_EXIT="+scenario.status)
			output, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				if exit, ok := errors.AsType[*exec.ExitError](err); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != scenario.want || !strings.Contains(string(output), scenario.output) {
				t.Fatalf("fix-check status=%d output=%q; want status %d and output containing %q", code, output, scenario.want, scenario.output)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("output unavailable")
}

func TestOutputFailuresReturnRuntimeError(t *testing.T) {
	root := copyRepository(t)
	for _, scenario := range []struct {
		name       string
		args       []string
		failStdout bool
		failStderr bool
	}{
		{"help stdout", []string{"--help"}, true, false},
		{"json stdout", []string{"validate", "--root", root, "--json"}, true, false},
		{"usage stderr", []string{"unknown-command"}, false, true},
		{"runtime stderr", []string{"validate", "--root", filepath.Join(root, "missing")}, false, true},
		{"both streams", []string{"--help"}, true, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			var stdout, stderr io.Writer = &out, &diagnostic
			if scenario.failStdout {
				stdout = failingWriter{}
			}
			if scenario.failStderr {
				stderr = failingWriter{}
			}
			if code := run(t.Context(), scenario.args, stdout, stderr, testDate); code != 1 {
				t.Fatalf("output failure exit status = %d; want 1", code)
			}
			if scenario.failStdout && !scenario.failStderr && !strings.Contains(diagnostic.String(), "output unavailable") {
				t.Fatalf("stdout failure not reported: %s", diagnostic.String())
			}
			if scenario.failStderr && out.Len() != 0 {
				t.Fatalf("diagnostic leaked into stdout: %s", out.String())
			}
		})
	}
}

func TestCommandsAndRetrievalEvals(t *testing.T) {
	root := copyRepository(t)
	for _, command := range []string{"validate", "freshness", "stats", "index"} {
		t.Run(command, func(t *testing.T) {
			stdout, stderr, code := invoke("--root", root, command, "--json", "--as-of=2026-09-23")
			if code != 0 || stderr != "" || !json.Valid([]byte(stdout)) {
				t.Fatalf("%s: code=%d stdout=%s stderr=%s", command, code, stdout, stderr)
			}
			if command == "stats" {
				var stats map[string]int
				if err := json.Unmarshal([]byte(stdout), &stats); err != nil {
					t.Fatal(err)
				}
				for _, kind := range []string{"knowledge", "patterns", "modules", "sources"} {
					if stats[kind] < 1 {
						t.Errorf("expected at least one %s: %s", kind, stdout)
					}
				}
			}
		})
	}
	evalPaths, err := filepath.Glob(filepath.Join(repositoryRoot(t), "evals/knowledge/*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(evalPaths) == 0 {
		t.Fatal("no knowledge evals")
	}
	evalPaths = append(evalPaths, filepath.Join(repositoryRoot(t), "evals/patterns/search.json"))
	for _, evalPath := range evalPaths {
		data, err := os.ReadFile(evalPath)
		if err != nil {
			t.Fatal(err)
		}
		var evaluation struct {
			Cases []struct {
				Name        string   `json:"name"`
				Query       string   `json:"query"`
				ExpectedIDs []string `json:"expected_ids"`
			} `json:"cases"`
		}
		if err := json.Unmarshal(data, &evaluation); err != nil {
			t.Fatal(err)
		}
		if len(evaluation.Cases) == 0 {
			t.Fatalf("%s contains no retrieval evals", evalPath)
		}
		for _, scenario := range evaluation.Cases {
			t.Run(scenario.Name, func(t *testing.T) {
				stdout, stderr, code := invoke("search", scenario.Query, "--json", "--root", root)
				if code != 0 || stderr != "" {
					t.Fatalf("search failed: %d %s", code, stderr)
				}
				results := decodeResults(t, stdout)
				ids := map[string]bool{}
				for _, result := range results {
					id, _ := result["id"].(string)
					ids[id] = true
					for _, field := range []string{"path", "title", "type", "technology", "version", "relevance", "trust", "retrieved_at", "expires_at", "stale"} {
						if _, ok := result[field]; !ok {
							t.Errorf("result %q lacks %s", id, field)
						}
					}
				}
				for _, id := range scenario.ExpectedIDs {
					if !ids[id] {
						t.Errorf("expected retrieval %q absent: %s", id, stdout)
					}
				}
			})
		}
	}
	stdout, stderr, code := invoke("search", "context", "--root", root, "--limit", "1", "--json")
	if code != 0 || stderr != "" || len(decodeResults(t, stdout)) != 1 {
		t.Fatalf("limit not applied: %d %s %s", code, stdout, stderr)
	}
	stdout, stderr, code = invoke("search", "qzxvnonexistenttokenqzxv", "--root", root, "--json")
	if code != 0 || stderr != "" || strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("empty results: %d %s %s", code, stdout, stderr)
	}
	stdout, stderr, code = invoke("search", "context", "cancellation", "--root", root)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "context-cancellation") {
		t.Fatalf("multiword text search: %d %s %s", code, stdout, stderr)
	}
	for _, field := range []string{"path", "title", "type", "technology", "version", "relevance", "trust", "retrieved_at", "expires_at", "stale"} {
		if !strings.Contains(stdout, `"`+field+`"`) {
			t.Errorf("text output lacks %s", field)
		}
	}
}

func TestFreshnessIsEvaluatedAtQueryTime(t *testing.T) {
	root := copyRepository(t)
	_, stderr, code := invoke("index", "--root", root)
	if code != 0 {
		t.Fatal(stderr)
	}
	stdout, stderr, code := invoke("search", "context", "--root", root, "--as-of=2099-01-01", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("query after expiration failed: %d %s", code, stderr)
	}
	results := decodeResults(t, stdout)
	if len(results) < 3 {
		t.Fatalf("expected stale documents from all three kinds: %s", stdout)
	}
	for _, result := range results {
		if result["stale"] != true {
			t.Errorf("expired result not marked stale: %+v", result)
		}
	}
	stdout, stderr, code = invoke("freshness", "--root", root, "--as-of=2099-01-01", "--json")
	if code != 0 || stderr != "" || len(decodeResults(t, stdout)) < 3 {
		t.Fatalf("freshness omits expired documents: %d %s %s", code, stdout, stderr)
	}
}

func TestSearchRejectsMissingAndOutdatedIndex(t *testing.T) {
	root := copyRepository(t)
	stdout, stderr, code := invoke("search", "context", "--root", root, "--json")
	if code != 1 || stdout != "" || !strings.Contains(strings.ToLower(stderr), "index") {
		t.Fatalf("missing index must be actionable: %d %s %s", code, stdout, stderr)
	}
	if _, stderr, code := invoke("index", "--root", root); code != 0 {
		t.Fatal(stderr)
	}
	var document string
	if err := filepath.WalkDir(filepath.Join(root, "knowledge"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".md") {
			document = path
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if document == "" {
		t.Fatal("no knowledge fixture available")
	}
	f, err := os.OpenFile(document, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := io.WriteString(f, "\nCorpus changed after indexing.\n")
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code = invoke("search", "context", "--root", root, "--json")
	if code != 1 || stdout != "" || !strings.Contains(strings.ToLower(stderr), "index") {
		t.Fatalf("outdated index must be actionable: %d %s %s", code, stdout, stderr)
	}
}

func TestRootDiscoveryAndErrors(t *testing.T) {
	root := copyRepository(t)
	nested := filepath.Join(root, "knowledge", "go")
	t.Chdir(nested)
	if got, err := discoverRoot(); err != nil || got != root {
		t.Fatalf("discover nested root = %q, %v; want %q", got, err, root)
	}
	stdout, stderr, code := invoke("validate", "--json")
	if code != 0 || stderr != "" || !json.Valid([]byte(stdout)) {
		t.Fatalf("nested validation: %d %s %s", code, stdout, stderr)
	}
	t.Chdir(t.TempDir())
	stdout, stderr, code = invoke("validate", "--json")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "--root") {
		t.Fatalf("missing root error: %d %s %s", code, stdout, stderr)
	}
	stdout, stderr, code = invoke("validate", "--root", filepath.Join(root, "missing"), "--json")
	if code != 1 || stdout != "" || stderr == "" {
		t.Fatalf("bad explicit root error: %d %s %s", code, stdout, stderr)
	}
	stdout, stderr, code = invoke("search", "context", "--wat", "--json")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "unknown option") {
		t.Fatalf("usage error: %d %s %s", code, stdout, stderr)
	}
	stdout, stderr, code = invoke("--help")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Usage:") {
		t.Fatalf("help outside repository: %d %s %s", code, stdout, stderr)
	}
}

func invoke(args ...string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &stdout, &stderr, testDate)
	return stdout.String(), stderr.String(), code
}

func decodeResults(t *testing.T, output string) []map[string]any {
	t.Helper()
	var results []map[string]any
	if err := json.Unmarshal([]byte(output), &results); err != nil {
		t.Fatalf("invalid result JSON: %v: %s", err, output)
	}
	return results
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func copyRepository(t *testing.T) string {
	t.Helper()
	source := repositoryRoot(t)
	destination := filepath.Join(t.TempDir(), "repository with spaces")
	for _, name := range []string{"config", "knowledge", "patterns", "modules", "sources", "evals"} {
		from := filepath.Join(source, name)
		err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			target := filepath.Join(destination, relative)
			if entry.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relativePath := filepath.ToSlash(relative)
			if strings.HasPrefix(relativePath, "knowledge/") || strings.HasPrefix(relativePath, "sources/catalog/") {
				fixtureDate := testDate.Format("2006-01-02")
				data = retrievedAtField.ReplaceAll(data, []byte(`"retrieved_at": "`+fixtureDate+`"`))
			}
			return os.WriteFile(target, data, 0o644)
		})
		if err != nil {
			t.Fatalf("copy %s: %v", name, err)
		}
	}
	module, err := os.ReadFile(filepath.Join(source, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "go.mod"), module, 0o644); err != nil {
		t.Fatal(err)
	}
	return destination
}
