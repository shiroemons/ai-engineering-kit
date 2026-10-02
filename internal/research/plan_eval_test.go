package research

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shiroemons/ai-engineering-kit/internal/kb"
)

type evalBackendFunc func(context.Context, []kb.Document, string) (map[string]int, error)

func (f evalBackendFunc) Search(ctx context.Context, documents []kb.Document, query string) (map[string]int, error) {
	return f(ctx, documents, query)
}

// These new consistency regressions need two queries and one valid document,
// not a Git repository or every live knowledge item. Existing research
// integration tests continue to use the full corpus through fixture.
func evalFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"config", "sources/catalog", "knowledge/go", "patterns", "modules", "evals/knowledge"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"config/freshness.json", "config/sources.json", "sources/catalog/go-context-docs.json", "knowledge/go/context.md"} {
		data, err := os.ReadFile(filepath.Join(repo, path))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, path), data, 0644)
	}
	writeFile(t, filepath.Join(root, "evals/knowledge/search.json"), []byte(`{"cases":[{"name":"context","query":"context","expected_ids":["go-context"]},{"name":"cancellation","query":"context cancellation","expected_ids":["go-context"]}]}`), 0644)
	return root
}

func evalQueries(t *testing.T, root string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(root, "evals/knowledge/*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var queries []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var suite evaluation
		if err := json.Unmarshal(data, &suite); err != nil {
			t.Fatal(err)
		}
		for _, c := range suite.Cases {
			queries = append(queries, c.Query)
		}
	}
	return queries
}

func TestValidateEvalsExecutesEveryCaseAndRefreshesEachInvocation(t *testing.T) {
	root := evalFixture(t)
	// Repeated queries still represent distinct assertions and must all run.
	writeFile(t, filepath.Join(root, "evals/knowledge/repeated.json"), []byte(`{"cases":[{"name":"first","query":"context cancellation","expected_ids":["go-context"]},{"name":"second","query":"context cancellation","expected_ids":["go-context"]}]}`), 0644)
	r, _, err := indexed(root)
	if err != nil {
		t.Fatal(err)
	}
	want := evalQueries(t, root)
	var got []string
	sawUpdated := false
	r.Backend = evalBackendFunc(func(ctx context.Context, documents []kb.Document, query string) (map[string]int, error) {
		got = append(got, query)
		for _, doc := range documents {
			if doc.Metadata.ID == "go-context" && strings.Contains(doc.Body, "fresh-invocation-marker") {
				sawUpdated = true
			}
		}
		return (kb.FullTextBackend{}).Search(ctx, documents, query)
	})
	if err := validateEvals(t.Context(), root, r); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) || sawUpdated {
		t.Fatalf("first validation queries = %v; want %v; updated = %t", got, want, sawUpdated)
	}
	path := filepath.Join(root, "knowledge/go/context.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, append(data, []byte("\nfresh-invocation-marker\n")...), 0644)
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	got = nil
	if err := validateEvals(t.Context(), root, r); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) || !sawUpdated {
		t.Fatalf("second validation queries = %v; want %v; updated = %t", got, want, sawUpdated)
	}
}

func TestValidateEvalsRejectsChangesDuringFinalQuery(t *testing.T) {
	for _, mutation := range []string{"corrupt index", "reindexed input", "cancel"} {
		t.Run(mutation, func(t *testing.T) {
			root := evalFixture(t)
			r, _, err := indexed(root)
			if err != nil {
				t.Fatal(err)
			}
			remaining := len(evalQueries(t, root))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			r.Backend = evalBackendFunc(func(ctx context.Context, documents []kb.Document, query string) (map[string]int, error) {
				scores, err := (kb.FullTextBackend{}).Search(ctx, documents, query)
				remaining--
				if remaining == 0 {
					switch mutation {
					case "corrupt index":
						writeFile(t, filepath.Join(root, "rag/index/snapshot.json"), []byte("{"), 0644)
					case "reindexed input":
						path := filepath.Join(root, "knowledge/go/context.md")
						data, readErr := os.ReadFile(path)
						if readErr != nil {
							t.Fatal(readErr)
						}
						writeFile(t, path, append(data, []byte("\nchanged-during-evaluation\n")...), 0644)
						if indexErr := r.Index(); indexErr != nil {
							t.Fatal(indexErr)
						}
					case "cancel":
						cancel()
					}
				}
				return scores, err
			})
			err = validateEvals(ctx, root, r)
			if err == nil || remaining != 0 {
				t.Fatalf("validation after %s = %v; remaining cases = %d", mutation, err, remaining)
			}
			if mutation == "cancel" && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation was not preserved: %v", err)
			}
		})
	}
}

func TestValidateEvalsChecksBeforeEveryQuery(t *testing.T) {
	root := evalFixture(t)
	r, _, err := indexed(root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "rag/index/snapshot.json")
	validIndex, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	r.Backend = evalBackendFunc(func(ctx context.Context, documents []kb.Document, query string) (map[string]int, error) {
		calls++
		if calls == 1 {
			writeFile(t, path, []byte("{"), 0644)
		} else {
			// A start/end-only check would allow the second query to restore
			// the index, hiding the corruption between the two searches.
			writeFile(t, path, validIndex, 0644)
		}
		return (kb.FullTextBackend{}).Search(ctx, documents, query)
	})
	if err := validateEvals(t.Context(), root, r); err == nil || calls != 1 {
		t.Fatalf("corrupt index was not checked before the next query: calls=%d error=%v", calls, err)
	}
}

func TestValidateEvalsInvalidSuiteDiagnosticsPrecedePreparation(t *testing.T) {
	for _, scenario := range []struct {
		name string
		data string
	}{
		{"no files", ""},
		{"malformed JSON", "{"},
		{"empty suite", `{"cases":[]}`},
		{"missing name", `{"cases":[{"query":"context","expected_ids":["go-context"]}]}`},
		{"blank query", `{"cases":[{"name":"blank","query":" \t ","expected_ids":["go-context"]}]}`},
		{"missing expected IDs", `{"cases":[{"name":"no IDs","query":"context"}]}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "evals/knowledge")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if scenario.data != "" {
				writeFile(t, filepath.Join(dir, "invalid.json"), []byte(scenario.data), 0644)
			}
			// No repository is needed until a complete, valid case is found.
			if err := validateEvals(t.Context(), root, nil); !errors.Is(err, errInvalidEvalSuite) {
				t.Fatalf("invalid suite error category lost: %v", err)
			}
		})
	}
}

func TestValidateEvalsPreservesSearchFailureDiagnostics(t *testing.T) {
	root := evalFixture(t)
	r, _, err := indexed(root)
	if err != nil {
		t.Fatal(err)
	}
	backendErr := errors.New("test backend failure")
	r.Backend = evalBackendFunc(func(context.Context, []kb.Document, string) (map[string]int, error) {
		return nil, backendErr
	})
	if err := validateEvals(t.Context(), root, r); !errors.Is(err, backendErr) {
		t.Fatalf("backend error was not preserved: %v", err)
	}
	r.Backend = evalBackendFunc(func(context.Context, []kb.Document, string) (map[string]int, error) {
		return map[string]int{}, nil
	})
	if err := validateEvals(t.Context(), root, r); !errors.Is(err, errEvalAssertionFailed) {
		t.Fatalf("assertion error category was not preserved: %v", err)
	}
}
