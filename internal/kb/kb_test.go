package kb

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func write(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, root, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, path, string(data))
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"knowledge/go", "patterns/retry", "modules/go", "sources/catalog", "evals/modules"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeJSON(t, root, "config/freshness.json", freshnessConfig{map[string]int{"official_docs": 90, "github_repository_analysis": 90, "architecture_pattern": 180}, map[string]int{"go": 90, "ruby": 90}})
	writeJSON(t, root, "config/sources.json", sourceConfig{trustOrder, []string{"https"}})
	writeJSON(t, root, "sources/catalog/official.json", source("official", "official"))
	writeDoc(t, root, "knowledge/go/context.md", metadata("context"), "Cancel retry operations using context.")
	return root
}

func source(id, trust string) Source {
	return Source{ID: id, URL: "https://example.org/" + id, Version: "1", RetrievedAt: "2026-09-01", License: "BSD-3-Clause", Trust: trust, Type: "official_docs", Purpose: "Test source fixture"}
}

func metadata(id string) Metadata {
	return Metadata{ID: id, Title: "Context cancellation", Kind: "knowledge", Technology: "go", Version: "1", Tags: []string{"context"}, Sources: []SourceRef{{"official", "https://example.org/official", "official_docs"}}, RetrievedAt: "2026-09-01", ExpiresAt: "2026-12-01", Trust: "official", Status: "active"}
}

func writeDoc(t *testing.T, root, path string, meta Metadata, body string) {
	t.Helper()
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, path, "---\n"+string(data)+"\n---\n"+body+"\n")
}

func mustLoad(t *testing.T, root string) *Repository {
	t.Helper()
	r, err := Load(root, testNow)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestValidateAndFreshness(t *testing.T) {
	root := fixture(t)
	r := mustLoad(t, root)
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := r.Stats(); got.Documents != 1 || got.Active != 1 || got.Sources != 1 || got.Knowledge != 1 {
		t.Fatalf("unexpected stats: %+v", got)
	}
	if expiry := r.documents[0].EffectiveExpiry; expiry != "2026-11-30" {
		t.Fatalf("TTL should cap explicit expiry: %s", expiry)
	}
	m := metadata("context")
	m.ExpiresAt = "2026-09-23"
	writeDoc(t, root, "knowledge/go/context.md", m, "Retry")
	r = mustLoad(t, root)
	if got := r.Freshness(); len(got) != 1 || !got[0].Stale || got[0].ExpiresAt != "2026-09-23" {
		t.Fatalf("expiry day must be stale: %+v", got)
	}
	previous, err := Load(root, time.Date(2026, 9, 22, 23, 59, 59, 0, time.UTC))
	if err != nil || len(previous.Freshness()) != 0 {
		t.Fatalf("previous day should be active: %v", err)
	}
	m.Status = "stale"
	m.ExpiresAt = "2026-12-01"
	writeDoc(t, root, "knowledge/go/context.md", m, "Retry")
	if got := mustLoad(t, root).Stats(); got.Stale != 1 {
		t.Fatal("explicit stale status must be respected")
	}
}

func TestInvalidMetadata(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Metadata)
		want string
	}{
		{"bad date", func(m *Metadata) { m.RetrievedAt = "2026-02-30" }, "invalid YYYY-MM-DD"},
		{"future date", func(m *Metadata) { m.RetrievedAt = "2026-09-24" }, "future"},
		{"expiry before retrieval", func(m *Metadata) { m.ExpiresAt = "2026-08-01" }, "must follow"},
		{"missing title", func(m *Metadata) { m.Title = " " }, "nonempty title"},
		{"unsafe id", func(m *Metadata) { m.ID = "../oops" }, "valid id"},
		{"wrong kind", func(m *Metadata) { m.Kind = "pattern" }, "kind must be knowledge"},
		{"bad status", func(m *Metadata) { m.Status = "draft" }, "status must be"},
		{"bad trust", func(m *Metadata) { m.Trust = "trusted" }, "unknown trust"},
		{"empty sources", func(m *Metadata) { m.Sources = nil }, "at least one source"},
		{"unknown source", func(m *Metadata) { m.Sources[0].ID = "missing" }, "unknown source id"},
		{"URL mismatch", func(m *Metadata) { m.Sources[0].URL += "/changed" }, "does not match"},
		{"type mismatch", func(m *Metadata) { m.Sources[0].Type = "architecture_pattern" }, "does not match"},
		{"duplicate source", func(m *Metadata) { m.Sources = append(m.Sources, m.Sources[0]) }, "duplicate source"},
		{"eval traversal", func(m *Metadata) { m.Evals = []string{"evals/../outside.json"} }, "unsafe repository path"},
		{"eval absolute", func(m *Metadata) { m.Evals = []string{"/tmp/test.json"} }, "repository-relative"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			m := metadata("context")
			tc.edit(&m)
			writeDoc(t, root, "knowledge/go/context.md", m, "Body")
			_, err := Load(root, testNow)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestInvalidCorpusAndConfig(t *testing.T) {
	tests := []struct{ name, path, content, want string }{
		{"null config", "config/freshness.json", "null", "nonempty"},
		{"unknown key", "config/freshness.json", `{"ttl":90}`, "semantic"},
		{"zero TTL", "config/freshness.json", `{"source_types":{"official_docs":0},"technologies":{"go":90}}`, "invalid TTL"},
		{"negative TTL", "config/freshness.json", `{"source_types":{"official_docs":-1},"technologies":{"go":90}}`, "invalid TTL"},
		{"fractional TTL", "config/freshness.json", `{"source_types":{"official_docs":1.5},"technologies":{"go":90}}`, "semantic"},
		{"wrong trust order", "config/sources.json", `{"trust_order":["unknown"],"allowed_url_schemes":["https"]}`, "trust_order"},
		{"unsafe scheme", "config/sources.json", `{"trust_order":["official","maintainer","primary-source","community","unknown"],"allowed_url_schemes":["file"]}`, "unsupported URL scheme"},
		{"missing frontmatter", "knowledge/go/context.md", "# Context", "expected JSON front matter"},
		{"unknown frontmatter key", "knowledge/go/context.md", "---\n{\"unexpected\":true}\n---\nBody", "semantic"},
		{"YAML rejected", "knowledge/go/context.md", "---\nid: context\n---\nBody", "invalid JSON"},
		{"trailing JSON", "config/sources.json", "{} {}", "syntax"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			write(t, root, tc.path, tc.content)
			_, err := Load(root, testNow)
			if tc.want == "semantic" || tc.want == "syntax" {
				assertJSONError(t, err, tc.want)
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q got %v", tc.want, err)
			}
		})
	}
	for _, path := range []string{"config/freshness.json", "sources/catalog", "knowledge", "patterns", "modules"} {
		t.Run("missing "+path, func(t *testing.T) {
			root := fixture(t)
			if err := os.RemoveAll(filepath.Join(root, path)); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(root, testNow); err == nil {
				t.Fatal("missing required input should fail")
			}
		})
	}
}

func assertJSONError(t *testing.T, err error, kind string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s JSON error", kind)
	}
	// ラップされた型を確認し、標準ライブラリのメッセージ表現に依存しない。
	if kind == "semantic" {
		if _, ok := errors.AsType[*jsonv2.SemanticError](err); !ok {
			t.Fatalf("expected SemanticError, got %T: %v", err, err)
		}
	} else if _, ok := errors.AsType[*jsontext.SyntacticError](err); !ok {
		t.Fatalf("expected SyntacticError, got %T: %v", err, err)
	}
}

func TestStrictJSONInput(t *testing.T) {
	meta, err := json.Marshal(metadata("context"))
	if err != nil {
		t.Fatal(err)
	}
	valid := string(meta)
	for _, tc := range []struct{ name, json, kind string }{
		{"duplicate metadata key", strings.Replace(valid, `"title":`, `"title":"earlier title","title":`, 1), "syntax"},
		{"invalid UTF-8", strings.Replace(valid, "Context cancellation", "Context \xff", 1), "syntax"},
		{"case-mismatched key", strings.Replace(valid, `"title":`, `"Title":`, 1), "semantic"},
		{"trailing JSON object", valid + " {}", "syntax"},
		{"trailing non-JSON content", valid + " trailing", "syntax"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			write(t, root, "knowledge/go/context.md", "---\n"+tc.json+"\n---\nBody")
			_, err := Load(root, testNow)
			assertJSONError(t, err, tc.kind)
		})
	}
	t.Run("duplicate TTL key", func(t *testing.T) {
		root := fixture(t)
		write(t, root, "config/freshness.json", `{"source_types":{"official_docs":90,"official_docs":180},"technologies":{"go":90}}`)
		_, err := Load(root, testNow)
		assertJSONError(t, err, "syntax")
	})
}

func TestSourceValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Source)
		want string
	}{
		{"trust overclaim", func(s *Source) { s.Trust = "community" }, "weakest source"},
		{"unsafe URL", func(s *Source) { s.URL = "file:///etc/passwd" }, "invalid source URL"},
		{"credentials in URL", func(s *Source) { s.URL = "https://secret@example.org/" }, "invalid source URL"},
		{"unknown source type", func(s *Source) { s.Type = "unknown" }, "unknown source type"},
		{"future source", func(s *Source) { s.RetrievedAt = "2026-09-24" }, "future"},
		{"unpinned repository", func(s *Source) {
			s.Type = "github_repository_analysis"
			s.RepositoryURL = "https://github.com/example/repo"
		}, "pinned"},
		{"short SHA", func(s *Source) { s.CommitSHA = "abc" }, "40 hexadecimal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			src := source("official", "official")
			tc.edit(&src)
			writeJSON(t, root, "sources/catalog/official.json", src)
			_, err := Load(root, testNow)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q got %v", tc.want, err)
			}
		})
	}
}

func TestDuplicateIDsAndExclusions(t *testing.T) {
	root := fixture(t)
	write(t, root, "knowledge/go/draft.template.md", "Unfilled template")
	write(t, root, "patterns/retry/README.md", "Organizational guidance")
	if got := mustLoad(t, root).Stats().Documents; got != 1 {
		t.Fatalf("templates should not be indexed: %d", got)
	}
	writeDoc(t, root, "knowledge/go/duplicate.md", metadata("context"), "duplicate")
	if _, err := Load(root, testNow); err == nil || !strings.Contains(err.Error(), "duplicate document id") {
		t.Fatalf("duplicate accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "knowledge/go/duplicate.md")); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, root, "sources/catalog/duplicate.json", source("official", "official"))
	if _, err := Load(root, testNow); err == nil || !strings.Contains(err.Error(), "duplicate source id") {
		t.Fatalf("duplicate source accepted: %v", err)
	}
}

func TestSearchRankingAndANDMatching(t *testing.T) {
	root := fixture(t)
	writeJSON(t, root, "sources/catalog/community.json", source("community", "community"))
	m := metadata("community")
	m.Trust = "community"
	m.Title = "Retry retry retry"
	m.Sources = []SourceRef{{"community", "https://example.org/community", "official_docs"}}
	writeDoc(t, root, "knowledge/go/community.md", m, "retry retry retry retry")
	m = metadata("stale")
	m.ExpiresAt = "2026-09-23"
	m.Title = "Retry retry retry"
	writeDoc(t, root, "knowledge/go/stale.md", m, "retry retry retry retry retry")
	m = metadata("active-official")
	m.Title = "Retry"
	writeDoc(t, root, "knowledge/go/active.md", m, "Retry")
	r := mustLoad(t, root)
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	results, err := r.Search(context.Background(), "RETRY", 10)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"active-official", "context", "community", "stale"}
	for i, id := range want {
		if len(results) != len(want) || results[i].ID != id {
			t.Fatalf("ranking: %+v", results)
		}
	}
	results, err = r.Search(context.Background(), "cancel retry", 10)
	if err != nil || len(results) != 1 || results[0].ID != "context" {
		t.Fatalf("AND matching: %+v %v", results, err)
	}
	results, err = r.Search(context.Background(), "no-such-term", 10)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(results)
	if string(b) != "[]" {
		t.Fatalf("empty JSON result: %s", b)
	}
	results, err = r.Search(context.Background(), "retry", 1)
	if err != nil || len(results) != 1 {
		t.Fatalf("limit: %+v %v", results, err)
	}
	for _, query := range []string{"", "  \t"} {
		if _, err := r.Search(context.Background(), query, 10); err == nil {
			t.Fatal("empty query accepted")
		}
	}
	if _, err := r.Search(context.Background(), "retry", 0); err == nil {
		t.Fatal("zero limit accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Search(ctx, "retry", 10); err == nil {
		t.Fatal("canceled search succeeded")
	}
}

func TestIndexInvalidationAndQueryTimeFreshness(t *testing.T) {
	root := fixture(t)
	r := mustLoad(t, root)
	if _, err := r.Search(context.Background(), "retry", 10); err == nil || !strings.Contains(err.Error(), "kb index") {
		t.Fatalf("missing index: %v", err)
	}
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	later, err := Load(root, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	results, err := later.Search(context.Background(), "retry", 10)
	if err != nil || len(results) != 1 || !results[0].Stale {
		t.Fatalf("query-time expiry: %+v %v", results, err)
	}
	writeDoc(t, root, "knowledge/go/context.md", metadata("context"), "Changed retry guidance")
	if _, err := r.Search(context.Background(), "retry", 10); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Fatalf("changed content: %v", err)
	}
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	src := source("official", "official")
	src.Purpose = "Changed source purpose"
	writeJSON(t, root, "sources/catalog/official.json", src)
	if _, err := r.Search(context.Background(), "retry", 10); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Fatalf("changed catalog: %v", err)
	}
}

func TestIndexDoesNotWriteSourcesAndRejectsCorruption(t *testing.T) {
	root := fixture(t)
	r := mustLoad(t, root)
	before := r.inputFingerprint()
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	if after := mustLoad(t, root).inputFingerprint(); before != after {
		t.Fatal("index modified source inputs")
	}
	var index snapshot
	data, err := os.ReadFile(filepath.Join(root, "rag/index/snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatal(err)
	}
	index.Documents[0].Body = "Forged guidance"
	writeJSON(t, root, "rag/index/snapshot.json", index)
	if _, err := r.Search(context.Background(), "retry", 10); err == nil || !strings.Contains(err.Error(), "inconsistent") {
		t.Fatalf("corrupt snapshot: %v", err)
	}
}

func TestSymlinksRejected(t *testing.T) {
	t.Run("input", func(t *testing.T) {
		root := fixture(t)
		target := filepath.Join(t.TempDir(), "external.md")
		if err := os.WriteFile(target, []byte("external"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(root, "knowledge/go/link.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(root, testNow); err == nil || !strings.Contains(err.Error(), "symlink") {
			t.Fatalf("symlink accepted: %v", err)
		}
	})
	for _, path := range []string{"rag", "rag/index", "rag/documents/documents.json"} {
		t.Run(path, func(t *testing.T) {
			root := fixture(t)
			r := mustLoad(t, root)
			full := filepath.Join(root, path)
			if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), full); err != nil {
				t.Fatal(err)
			}
			if err := r.Index(); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("output symlink accepted: %v", err)
			}
		})
	}
}

func TestFailedIndexPublicationCleansTemporaryFile(t *testing.T) {
	root := fixture(t)
	r := mustLoad(t, root)
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	snapshotPath := filepath.Join(root, "rag/index/snapshot.json")
	before, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(root, "rag/documents/documents.json")
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(blocked, 0755); err != nil {
		t.Fatal(err)
	}
	if err := r.Index(); err == nil {
		t.Fatal("publication onto a directory should fail")
	}
	after, err := os.ReadFile(snapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed publication changed authoritative snapshot")
	}
	entries, err := os.ReadDir(filepath.Join(root, "rag/documents"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".kb-") {
			t.Fatalf("temporary file was not removed: %s", entry.Name())
		}
	}
	if _, err := r.Search(context.Background(), "retry", 10); err != nil {
		t.Fatalf("prior snapshot should remain usable: %v", err)
	}
}

func addModule(t *testing.T, root string) {
	t.Helper()
	m := metadata("contextwait")
	m.Kind = "module"
	m.Evals = []string{"evals/modules/contextwait.json"}
	writeJSON(t, root, m.Evals[0], map[string]any{"cases": []string{"cancel", "timeout"}})
	writeDoc(t, root, "modules/go/contextwait/README.md", m, "Wait with cancellation")
	writeJSON(t, root, "modules/go/contextwait/module.json", moduleManifest{"Wait(ctx context.Context)", []string{"wait_test.go"}, m.Evals, "PROVENANCE.md", "test-reviewer", "2026-09-23", "No shared state"})
	write(t, root, "modules/go/contextwait/PROVENANCE.md", "Original implementation; ideas from context documentation.")
	write(t, root, "modules/go/contextwait/wait.go", "package contextwait\nfunc Wait() {}\n")
	write(t, root, "modules/go/contextwait/wait_test.go", "package contextwait\nimport \"testing\"\nfunc TestWait(t *testing.T) { Wait() }\n")
}

func TestModulePromotion(t *testing.T) {
	root := fixture(t)
	addModule(t, root)
	r := mustLoad(t, root)
	if r.Stats().Modules != 1 {
		t.Fatal("module absent")
	}
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	write(t, root, "modules/go/contextwait/wait.go", "package contextwait\nfunc Wait() { /* 変更後 */ }\n")
	if _, err := r.Search(context.Background(), "wait", 10); err == nil || !strings.Contains(err.Error(), "out of date") {
		t.Fatalf("module edit did not invalidate index: %v", err)
	}
	for _, path := range []string{"README.md", "module.json", "PROVENANCE.md", "wait.go", "wait_test.go"} {
		t.Run("missing "+path, func(t *testing.T) {
			root := fixture(t)
			addModule(t, root)
			if err := os.Remove(filepath.Join(root, "modules/go/contextwait", path)); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(root, testNow); err == nil {
				t.Fatalf("missing %s accepted", path)
			}
		})
	}
	t.Run("unknown license", func(t *testing.T) {
		root := fixture(t)
		addModule(t, root)
		src := source("official", "official")
		src.License = "unknown"
		writeJSON(t, root, "sources/catalog/official.json", src)
		if _, err := Load(root, testNow); err == nil || !strings.Contains(err.Error(), "unknown license") {
			t.Fatalf("unknown license accepted: %v", err)
		}
	})
	t.Run("loose code", func(t *testing.T) {
		root := fixture(t)
		write(t, root, "modules/go/wait.go", "package wait")
		if _, err := Load(root, testNow); err == nil {
			t.Fatal("loose module code accepted")
		}
	})
}

func TestEvalRequiresStructuredCases(t *testing.T) {
	for _, content := range []string{"null", "{}", "[]", `"only a label"`, "true", "invalid JSON", `{"cases":[],"cases":["cancel"]}`, "{\"case\":\"\xff\"}"} {
		t.Run(content, func(t *testing.T) {
			root := fixture(t)
			addModule(t, root)
			write(t, root, "evals/modules/contextwait.json", content)
			if _, err := Load(root, testNow); err == nil {
				t.Fatalf("invalid eval accepted: %s", content)
			}
		})
	}
}
