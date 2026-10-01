package research

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/shiroemons/ai-engineering-kit/internal/kb"
)

func gptFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root, state := fixture(t, "normal")
	writeFile(t, filepath.Join(root, "evals/knowledge/one.json"), []byte(`{"cases":[{"name":"baseline-context","query":"context cancellation","expected_ids":["go-context"]}]}`), 0644)
	gptTestCommit(t, root)
	return root, gptTestHEAD(t, root), state
}

func gptTestHEAD(t *testing.T, root string) string {
	t.Helper()
	data, err := git(t.Context(), root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func gptTestCommit(t *testing.T, root string) {
	t.Helper()
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=GPT Test", "-c", "user.email=gpt@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "test artifacts"}} {
		if _, err := git(t.Context(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
}

func gptTestDocument(t *testing.T, root, domain, id string) string {
	t.Helper()
	path := "knowledge/go/" + id + ".md"
	meta := kb.Metadata{ID: id, Title: id, Kind: "knowledge", Technology: "go", Version: "Go 1.27.1", Tags: []string{"research-domain:" + domain}, Sources: []kb.SourceRef{{ID: "go-context-docs", URL: "https://pkg.go.dev/context", Type: "official_docs"}}, RetrievedAt: "2026-09-23", ExpiresAt: "2026-12-22", Trust: "official", Status: "active", Evals: []string{"evals/knowledge/" + domain + ".json"}}
	gptWriteMetadata(t, root, path, meta)
	evalPath := filepath.Join(root, "evals/knowledge/"+domain+".json")
	var suite evaluation
	if data, err := os.ReadFile(evalPath); err == nil {
		if err := json.Unmarshal(data, &suite); err != nil {
			t.Fatal(err)
		}
	}
	suite.Cases = append(suite.Cases, evalCase{Name: id, Query: id, ExpectedIDs: []string{id}})
	data, err := json.Marshal(suite)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, evalPath, data, 0644)
	return path
}

func gptWriteMetadata(t *testing.T, root, path string, meta kb.Metadata) {
	t.Helper()
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, path), []byte("---\n"+string(data)+"\n---\n\n# "+meta.ID+"\n\nVerified context behavior.\n"), 0644)
}

func gptChangeMetadata(t *testing.T, root, path string, change func(*kb.Metadata)) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	front, _, ok := bytes.Cut(data[4:], []byte("\n---\n"))
	if !ok {
		t.Fatal("invalid test document")
	}
	var meta kb.Metadata
	if err := json.Unmarshal(front, &meta); err != nil {
		t.Fatal(err)
	}
	change(&meta)
	gptWriteMetadata(t, root, path, meta)
}

func TestGPTPlanCleanCoverageWithoutOpenCode(t *testing.T) {
	root, base, state := gptFixture(t)
	before, err := os.ReadFile(filepath.Join(root, ".git/index"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := GPTPlan(t.Context(), root, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Base != base || result.Topics != 3 || result.SelectionStrategy != "coverage_first" || len(result.Domains) != 3 {
		t.Fatalf("unexpected plan: %+v", result)
	}
	if result.Domains[0].ID != "two" || result.Domains[1].ID != "three" || result.Domains[2].ID != "one" {
		t.Fatalf("not coverage-first: %+v", result.Domains)
	}
	if result.Domains[0].KnowledgeCount != 0 || result.Domains[2].KnowledgeCount == 0 || result.Domains[0].EvalPath != "evals/knowledge/two.json" {
		t.Fatalf("missing coverage/assignment details: %+v", result.Domains)
	}
	after, err := os.ReadFile(filepath.Join(root, ".git/index"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("plan changed Git index: %v", err)
	}
	entries, err := os.ReadDir(state)
	if err != nil || len(entries) != 0 {
		t.Fatalf("plan invoked OpenCode: %v %v", entries, err)
	}
	result, err = GPTPlan(t.Context(), root, 1)
	if err != nil || len(result.Domains) != 1 || result.Domains[0].ID != "two" {
		t.Fatalf("bounded plan: %+v %v", result, err)
	}
	writeFile(t, filepath.Join(root, "untracked.txt"), []byte("dirty"), 0644)
	if _, err := GPTPlan(t.Context(), root, 3); err == nil || !strings.Contains(err.Error(), "clean working tree") {
		t.Fatalf("dirty plan accepted: %v", err)
	}
}

func TestGPTPlanRejectsInvalidBaselineEval(t *testing.T) {
	root, _, _ := gptFixture(t)
	writeFile(t, filepath.Join(root, "evals/knowledge/one.json"), []byte(`{"cases":[{"name":"missing","query":"impossibletoken","expected_ids":["go-context"]}]}`), 0644)
	gptTestCommit(t, root)
	if _, err := GPTPlan(t.Context(), root, 3); err == nil || !strings.Contains(err.Error(), "baseline knowledge eval") {
		t.Fatalf("broken baseline accepted: %v", err)
	}
}

func TestGPTValidateFinalCommittedStagedAndUntracked(t *testing.T) {
	root, base, state := gptFixture(t)
	gptTestDocument(t, root, "one", "one-research")
	gptTestCommit(t, root)
	gptTestDocument(t, root, "two", "two-research")
	if _, err := git(t.Context(), root, "add", "knowledge/go/two-research.md", "evals/knowledge/two.json"); err != nil {
		t.Fatal(err)
	}
	gptTestDocument(t, root, "three", "three-research")
	before, err := os.ReadFile(filepath.Join(root, ".git/index"))
	if err != nil {
		t.Fatal(err)
	}
	head := gptTestHEAD(t, root)
	result, err := ValidateGPT(t.Context(), root, base, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Documents) != 3 || len(result.Files) != 6 || result.Base != base || result.Topics != 3 {
		t.Fatalf("unexpected validation: %+v", result)
	}
	after, err := os.ReadFile(filepath.Join(root, ".git/index"))
	if err != nil || !bytes.Equal(before, after) || gptTestHEAD(t, root) != head {
		t.Fatalf("validation mutated Git state: %v", err)
	}
	entries, err := os.ReadDir(state)
	if err != nil || len(entries) != 0 {
		t.Fatalf("validation invoked OpenCode: %v %v", entries, err)
	}
	if _, err := ValidateGPT(t.Context(), root, base, 2); err == nil || !strings.Contains(err.Error(), "1..2") {
		t.Fatalf("topic limit bypassed: %v", err)
	}
}

func TestGPTValidateOneTopicAndNewReferencedSource(t *testing.T) {
	root, base, _ := gptFixture(t)
	path := gptTestDocument(t, root, "one", "one-research")
	data, err := os.ReadFile(filepath.Join(root, "sources/catalog/go-context-docs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source kb.Source
	if err := json.Unmarshal(data, &source); err != nil {
		t.Fatal(err)
	}
	source.ID = "new-context-docs"
	data, err = json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "sources/catalog/new-context-docs.json"), data, 0644)
	gptChangeMetadata(t, root, path, func(m *kb.Metadata) { m.Sources[0].ID = source.ID })
	result, err := ValidateGPT(t.Context(), root, base, 3)
	if err != nil || len(result.Documents) != 1 || !slices.Contains(result.Files, "sources/catalog/new-context-docs.json") {
		t.Fatalf("valid bounded new-source batch: %+v %v", result, err)
	}
}

func TestGPTValidateRejectsUnsafeArtifacts(t *testing.T) {
	cases := []struct {
		name string
		want string
		edit func(*testing.T, string, string)
	}{
		{"untracked outside scope", "outside allowed", func(t *testing.T, root, _ string) {
			writeFile(t, filepath.Join(root, "unexpected.txt"), []byte("bad"), 0644)
		}},
		{"working config", "outside allowed", func(t *testing.T, root, _ string) {
			writeFile(t, filepath.Join(root, "config/research.json"), []byte("{}"), 0644)
		}},
		{"staged config concealed by working tree", "outside allowed", func(t *testing.T, root, _ string) {
			path := filepath.Join(root, "config/research.json")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, []byte("{}"), 0644)
			if _, err := git(t.Context(), root, "add", "config/research.json"); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, original, 0644)
		}},
		{"staged source concealed by working tree", "mixed staged", func(t *testing.T, root, _ string) {
			path := filepath.Join(root, "sources/catalog/go-context-docs.json")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, bytes.ReplaceAll(original, []byte("2026-09-23"), []byte("2026-09-24")), 0644)
			if _, err := git(t.Context(), root, "add", "sources/catalog/go-context-docs.json"); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, original, 0644)
		}},
		{"staged eval concealed by working tree", "mixed staged", func(t *testing.T, root, _ string) {
			path := filepath.Join(root, "evals/knowledge/one.json")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, []byte(`{"cases":[]}`), 0644)
			if _, err := git(t.Context(), root, "add", "evals/knowledge/one.json"); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, original, 0644)
		}},

		{"committed code", "outside allowed", func(t *testing.T, root, _ string) {
			writeFile(t, filepath.Join(root, "unexpected.go"), []byte("package bad\n"), 0644)
			gptTestCommit(t, root)
		}},
		{"committed config concealed by working tree", "outside allowed", func(t *testing.T, root, _ string) {
			path := filepath.Join(root, "config/research.json")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, []byte("{}"), 0644)
			gptTestCommit(t, root)
			writeFile(t, path, original, 0644)
		}},
		{"committed source concealed by working tree", "masked by the final", func(t *testing.T, root, _ string) {
			path := filepath.Join(root, "sources/catalog/go-context-docs.json")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, bytes.ReplaceAll(original, []byte("2026-09-23"), []byte("2026-09-24")), 0644)
			gptTestCommit(t, root)
			writeFile(t, path, original, 0644)
		}},
		{"committed eval concealed by working tree", "masked by the final", func(t *testing.T, root, _ string) {
			path := filepath.Join(root, "evals/knowledge/backend.json")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, []byte(`{"cases":[]}`), 0644)
			gptTestCommit(t, root)
			writeFile(t, path, original, 0644)
		}},

		{"delete source", "deletion", func(t *testing.T, root, _ string) {
			if err := os.Remove(filepath.Join(root, "sources/catalog/go-context-docs.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"committed delete", "deletion", func(t *testing.T, root, _ string) {
			if err := os.Remove(filepath.Join(root, "sources/catalog/go-context-docs.json")); err != nil {
				t.Fatal(err)
			}
			gptTestCommit(t, root)
		}},
		{"rename", "rename", func(t *testing.T, root, _ string) {
			if _, err := git(t.Context(), root, "mv", "sources/catalog/go-context-docs.json", "sources/catalog/renamed.json"); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink", "symlink", func(t *testing.T, root, path string) {
			if err := os.Remove(filepath.Join(root, path)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("go-context.md", filepath.Join(root, path)); err != nil {
				t.Fatal(err)
			}
		}},
		{"symlink parent", "forbidden", func(t *testing.T, root, _ string) {
			if err := os.Rename(filepath.Join(root, "knowledge/go"), filepath.Join(root, "knowledge-real")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("../knowledge-real", filepath.Join(root, "knowledge/go")); err != nil {
				t.Fatal(err)
			}
		}},
		{"executable working file", "non-executable", func(t *testing.T, root, path string) {
			if err := os.Chmod(filepath.Join(root, path), 0755); err != nil {
				t.Fatal(err)
			}
		}},
		{"executable staged concealed by working tree", "non-executable", func(t *testing.T, root, path string) {
			if err := os.Chmod(filepath.Join(root, path), 0755); err != nil {
				t.Fatal(err)
			}
			if _, err := git(t.Context(), root, "add", path); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(filepath.Join(root, path), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := git(t.Context(), root, "config", "core.filemode", "false"); err != nil {
				t.Fatal(err)
			}
		}},
		{"executable committed concealed by working tree", "non-executable", func(t *testing.T, root, path string) {
			if err := os.Chmod(filepath.Join(root, path), 0755); err != nil {
				t.Fatal(err)
			}
			gptTestCommit(t, root)
			if err := os.Chmod(filepath.Join(root, path), 0644); err != nil {
				t.Fatal(err)
			}
		}},

		{"oversized", "2 MiB", func(t *testing.T, root, path string) {
			writeFile(t, filepath.Join(root, path), bytes.Repeat([]byte{'x'}, gptMaxArtifactBytes+1), 0644)
		}},
		{"wrong extension", "outside allowed", func(t *testing.T, root, _ string) {
			writeFile(t, filepath.Join(root, "sources/catalog/extra.txt"), []byte("bad"), 0644)
		}},
		{"nested source", "outside allowed", func(t *testing.T, root, _ string) {
			dir := filepath.Join(root, "sources/catalog/nested")
			if err := os.Mkdir(dir, 0755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(dir, "extra.json"), []byte("{}"), 0644)
		}},
		{"same domain twice", "exactly one knowledge", func(t *testing.T, root, _ string) { gptTestDocument(t, root, "one", "another-research") }},
		{"missing domain tag", "exactly one configured", func(t *testing.T, root, path string) {
			gptChangeMetadata(t, root, path, func(m *kb.Metadata) { m.Tags = []string{"context"} })
		}},
		{"duplicate domain tag", "exactly one configured", func(t *testing.T, root, path string) {
			gptChangeMetadata(t, root, path, func(m *kb.Metadata) { m.Tags = append(m.Tags, "research-domain:one") })
		}},
		{"conflicting domain tag", "exactly one configured", func(t *testing.T, root, path string) {
			gptChangeMetadata(t, root, path, func(m *kb.Metadata) { m.Tags = append(m.Tags, "research-domain:two") })
		}},
		{"unconfigured domain", "exactly one configured", func(t *testing.T, root, path string) {
			gptChangeMetadata(t, root, path, func(m *kb.Metadata) { m.Tags = []string{"research-domain:unknown"} })
		}},
		{"unconfigured technology", "matching technology", func(t *testing.T, root, path string) {
			gptChangeMetadata(t, root, path, func(m *kb.Metadata) { m.Technology = "python" })
		}},
		{"directory technology mismatch", "directory must match", func(t *testing.T, root, path string) {
			if err := os.Rename(filepath.Join(root, path), filepath.Join(root, "knowledge/python/one-research.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing metadata eval", "metadata must reference", func(t *testing.T, root, path string) {
			gptChangeMetadata(t, root, path, func(m *kb.Metadata) { m.Evals = nil })
		}},
		{"eval missing changed ID", "must cover", func(t *testing.T, root, _ string) {
			writeFile(t, filepath.Join(root, "evals/knowledge/one.json"), []byte(`{"cases":[{"name":"context","query":"context","expected_ids":["go-context"]}]}`), 0644)
		}},
		{"unrelated domain eval", "no changed knowledge", func(t *testing.T, root, _ string) {
			writeFile(t, filepath.Join(root, "evals/knowledge/two.json"), []byte(`{"cases":[{"name":"context","query":"context","expected_ids":["go-context"]}]}`), 0644)
		}},
		{"removed existing eval", "preserved unchanged", func(t *testing.T, root, _ string) {
			writeFile(t, filepath.Join(root, "evals/knowledge/one.json"), []byte(`{"cases":[{"name":"one-research","query":"one-research","expected_ids":["one-research"]}]}`), 0644)
		}},
		{"modified existing eval", "preserved unchanged", func(t *testing.T, root, _ string) {
			path := filepath.Join(root, "evals/knowledge/one.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, bytes.ReplaceAll(data, []byte("context cancellation"), []byte("context")), 0644)
		}},
		{"duplicate JSON eval key", "duplicate", func(t *testing.T, root, _ string) {
			writeFile(t, filepath.Join(root, "evals/knowledge/one.json"), []byte(`{"cases":[],"cases":[{"name":"one-research","query":"one-research","expected_ids":["one-research"]}]}`), 0644)
		}},
		{"modified existing source", "source records must be preserved", func(t *testing.T, root, _ string) {
			path := filepath.Join(root, "sources/catalog/go-context-docs.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, bytes.ReplaceAll(data, []byte("2026-09-23"), []byte("2026-09-24")), 0644)
		}},
		{"unreferenced source", "referenced by a changed", func(t *testing.T, root, _ string) {
			data, err := os.ReadFile(filepath.Join(root, "sources/catalog/go-context-docs.json"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "sources/catalog/unrelated.json"), bytes.ReplaceAll(data, []byte("go-context-docs"), []byte("unrelated-docs")), 0644)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, base, _ := gptFixture(t)
			path := gptTestDocument(t, root, "one", "one-research")
			tc.edit(t, root, path)
			if _, err := ValidateGPT(t.Context(), root, base, 3); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestGPTValidateRunsEverySuite(t *testing.T) {
	root, _, _ := gptFixture(t)
	writeFile(t, filepath.Join(root, "evals/knowledge/backend.json"), []byte(`{"cases":[{"name":"unrelated-broken-suite","query":"impossibletoken","expected_ids":["go-context"]}]}`), 0644)
	gptTestCommit(t, root)
	base := gptTestHEAD(t, root)
	gptTestDocument(t, root, "one", "one-research")
	if _, err := ValidateGPT(t.Context(), root, base, 3); err == nil || !strings.Contains(err.Error(), "unrelated-broken-suite") {
		t.Fatalf("unmodified suite not executed: %v", err)
	}
}

func TestGPTValidateBaselineAndBounds(t *testing.T) {
	root, base, _ := gptFixture(t)
	for _, n := range []int{-1, 0, 4} {
		if _, err := GPTPlan(t.Context(), root, n); err == nil {
			t.Fatalf("plan accepted topics=%d", n)
		}
		if _, err := ValidateGPT(t.Context(), root, base, n); err == nil {
			t.Fatalf("validate accepted topics=%d", n)
		}
	}
	for _, value := range []string{"", "HEAD", base[:12], strings.Repeat("f", 40), "--help"} {
		if _, err := ValidateGPT(t.Context(), root, value, 3); err == nil {
			t.Fatalf("invalid base accepted: %q", value)
		}
	}
	blob, err := git(t.Context(), root, "rev-parse", "HEAD:config/research.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateGPT(t.Context(), root, strings.TrimSpace(string(blob)), 3); err == nil || !strings.Contains(err.Error(), "identify a commit") {
		t.Fatalf("blob base accepted: %v", err)
	}
	if _, err := ValidateGPT(t.Context(), root, base, 3); err == nil || !strings.Contains(err.Error(), "found 0") {
		t.Fatalf("empty batch accepted: %v", err)
	}
	if _, err := GPTPlan(t.Context(), filepath.Join(root, "knowledge"), 1); err == nil || !strings.Contains(err.Error(), "top-level") {
		t.Fatalf("nonroot accepted: %v", err)
	}
	// Create a sibling commit without changing the worktree or index.
	tree, err := git(t.Context(), root, "rev-parse", "HEAD^{tree}")
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := git(t.Context(), root, "-c", "user.name=GPT Test", "-c", "user.email=gpt@example.invalid", "commit-tree", strings.TrimSpace(string(tree)), "-m", "unrelated root")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateGPT(t.Context(), root, strings.TrimSpace(string(sibling)), 3); err == nil || !strings.Contains(err.Error(), "ancestor") {
		t.Fatalf("nonancestor accepted: %v", err)
	}
}

func TestGPTArtifactPathAndDiffParsing(t *testing.T) {
	for _, path := range []string{"../knowledge/go/a.md", "/knowledge/go/a.md", "knowledge/go/../a.md", "knowledge\\go\\a.md", "knowledge/go/a\nb.md", "knowledge/go/a.md/extra", "evals/knowledge/unknown.txt", "sources/catalog/x.md", "knowledge/go/.md"} {
		if err := gptArtifactPath(path); err == nil {
			t.Fatalf("unsafe path accepted: %q", path)
		}
	}
	for _, data := range []string{"M\x00knowledge/go/a.md", "R100\x00knowledge/go/a.md\x00knowledge/go/b.md\x00", "D\x00knowledge/go/a.md\x00", "T\x00knowledge/go/a.md\x00", "M\x00"} {
		if _, err := gptDiffPaths([]byte(data)); err == nil {
			t.Fatalf("unsafe diff accepted: %q", data)
		}
	}
}

func TestGPTValidateRejectsHiddenCommittedDocument(t *testing.T) {
	root, base, _ := gptFixture(t)
	path := filepath.Join(root, "knowledge/go/context.md")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, append(slices.Clone(original), []byte("\nExtra committed research.\n")...), 0644)
	gptTestCommit(t, root)
	writeFile(t, path, original, 0644)
	gptTestDocument(t, root, "one", "one-research")
	if _, err := ValidateGPT(t.Context(), root, base, 1); err == nil || !strings.Contains(err.Error(), "masked by the final") {
		t.Fatalf("hidden extra committed document accepted: %v", err)
	}
}

func TestGPTValidateExistingDocumentWithUnchangedEval(t *testing.T) {
	root, base, _ := gptFixture(t)
	path := "knowledge/go/context.md"
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	front, body, ok := bytes.Cut(data[4:], []byte("\n---\n"))
	if !ok {
		t.Fatal("invalid fixture front matter")
	}
	var meta kb.Metadata
	if err := json.Unmarshal(front, &meta); err != nil {
		t.Fatal(err)
	}
	meta.Tags = append(meta.Tags, "research-domain:one")
	meta.Evals = []string{"evals/knowledge/one.json"}
	front, err = json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, path), append([]byte("---\n"+string(front)+"\n---\n"), body...), 0644)
	result, err := ValidateGPT(t.Context(), root, base, 1)
	if err != nil || len(result.Documents) != 1 || len(result.Files) != 1 {
		t.Fatalf("existing domain eval covering the updated document should suffice: %+v %v", result, err)
	}
}

func TestGPTRegularFileRejectsNonregularAndSymlinkParents(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "knowledge/go"), 0755); err != nil {
		t.Fatal(err)
	}
	fifo := "knowledge/go/fifo.md"
	if err := syscall.Mkfifo(filepath.Join(root, fifo), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := gptRegularFile(root, fifo); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("FIFO accepted: %v", err)
	}
	if err := os.Symlink("go", filepath.Join(root, "knowledge/alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := gptRegularFile(root, "knowledge/alias/fifo.md"); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink parent accepted: %v", err)
	}
}
