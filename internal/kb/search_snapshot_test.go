package kb

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type snapshotBackendFunc func(context.Context, []Document, string) (map[string]int, error)

func (f snapshotBackendFunc) Search(ctx context.Context, documents []Document, query string) (map[string]int, error) {
	return f(ctx, documents, query)
}

func prepareSnapshot(t *testing.T, r *Repository) *SearchSnapshot {
	t.Helper()
	prepared, err := r.PrepareSearch(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func TestSearchSnapshotMatchesRepositoryRanking(t *testing.T) {
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
	m = metadata("tie-z")
	m.Title = "Identical"
	writeDoc(t, root, "knowledge/go/tie-a.md", m, "Identical")
	m.ID = "tie-a"
	writeDoc(t, root, "knowledge/go/tie-z.md", m, "Identical")
	r := mustLoad(t, root)
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	prepared := prepareSnapshot(t, r)
	for _, scenario := range []struct {
		query string
		limit int
		ids   []string
	}{
		{"RETRY", 10, []string{"active-official", "context", "community", "stale"}},
		{"cancel retry", 10, []string{"context"}},
		{"identical", 10, []string{"tie-z", "tie-a"}},
		{"retry", 1, []string{"active-official"}},
		{"no-such-term", 10, []string{}},
	} {
		t.Run(scenario.query, func(t *testing.T) {
			want, err := r.Search(t.Context(), scenario.query, scenario.limit)
			if err != nil {
				t.Fatal(err)
			}
			got, err := prepared.Search(t.Context(), scenario.query, scenario.limit)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("prepared results = %+v, %v; repository results = %+v", got, err, want)
			}
			ids := []string{}
			for _, result := range got {
				ids = append(ids, result.ID)
			}
			if !reflect.DeepEqual(ids, scenario.ids) {
				t.Fatalf("result IDs = %v; want %v", ids, scenario.ids)
			}
			if len(got) == 0 && got == nil {
				t.Fatal("empty results must encode as [] rather than null")
			}
		})
	}
}

func TestSearchSnapshotFreezesFreshness(t *testing.T) {
	root := fixture(t)
	m := metadata("context")
	m.ExpiresAt = "2026-09-23"
	writeDoc(t, root, "knowledge/go/context.md", m, "Retry")
	// This local date is September 23, but the UTC date is still September 22.
	before, err := Load(root, time.Date(2026, 9, 23, 1, 0, 0, 0, time.FixedZone("UTC+9", 9*60*60)))
	if err != nil {
		t.Fatal(err)
	}
	if err := before.Index(); err != nil {
		t.Fatal(err)
	}
	active := prepareSnapshot(t, before)
	expired := prepareSnapshot(t, mustLoad(t, root))
	for _, scenario := range []struct {
		name     string
		prepared *SearchSnapshot
		stale    bool
	}{
		{"before UTC expiry", active, false},
		{"on expiry day", expired, true},
		{"earlier snapshot remains active", active, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			got, err := scenario.prepared.Search(t.Context(), "retry", 10)
			if err != nil || len(got) != 1 || got[0].Stale != scenario.stale || got[0].ExpiresAt != "2026-09-23" {
				t.Fatalf("captured freshness = %+v, %v; want stale=%v", got, err, scenario.stale)
			}
		})
	}
}

func TestSearchSnapshotRetainsValidatedState(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		change func(*testing.T, string, *snapshot)
		want   string
	}{
		{"missing index", func(t *testing.T, root string, _ *snapshot) {
			if err := os.Remove(filepath.Join(root, "rag/index/snapshot.json")); err != nil {
				t.Fatal(err)
			}
		}, "index unavailable"},
		{"malformed index", func(t *testing.T, root string, _ *snapshot) {
			write(t, root, "rag/index/snapshot.json", "{")
		}, "index unavailable"},
		{"unknown index field", func(t *testing.T, root string, _ *snapshot) {
			write(t, root, "rag/index/snapshot.json", `{"unknown":true}`)
		}, "index unavailable"},
		{"schema change", func(t *testing.T, root string, index *snapshot) {
			index.SchemaVersion++
			writeJSON(t, root, "rag/index/snapshot.json", index)
		}, "out of date"},
		{"fingerprint change", func(t *testing.T, root string, index *snapshot) {
			index.Fingerprint = "changed"
			writeJSON(t, root, "rag/index/snapshot.json", index)
		}, "out of date"},
		{"tampered content with original fingerprint", func(t *testing.T, root string, index *snapshot) {
			index.Documents[0].Body = "Forged retry guidance"
			writeJSON(t, root, "rag/index/snapshot.json", index)
		}, "inconsistent"},
		{"changed document", func(t *testing.T, root string, _ *snapshot) {
			writeDoc(t, root, "knowledge/go/context.md", metadata("context"), "Changed retry guidance")
		}, "out of date"},
		{"changed source", func(t *testing.T, root string, _ *snapshot) {
			src := source("official", "official")
			src.Purpose = "Changed source purpose"
			writeJSON(t, root, "sources/catalog/official.json", src)
		}, "out of date"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			root := fixture(t)
			r := mustLoad(t, root)
			if err := r.Index(); err != nil {
				t.Fatal(err)
			}
			prepared := prepareSnapshot(t, r)
			want, err := prepared.Search(t.Context(), "retry", 10)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(root, "rag/index/snapshot.json"))
			if err != nil {
				t.Fatal(err)
			}
			var index snapshot
			if err := json.Unmarshal(data, &index); err != nil {
				t.Fatal(err)
			}
			scenario.change(t, root, &index)
			if _, err := r.PrepareSearch(t.Context()); err == nil || !strings.Contains(err.Error(), scenario.want) {
				t.Fatalf("PrepareSearch error = %v; want %q", err, scenario.want)
			}
			if _, err := r.Search(t.Context(), "retry", 10); err == nil || !strings.Contains(err.Error(), scenario.want) {
				t.Fatalf("Repository.Search error = %v; want %q", err, scenario.want)
			}
			got, err := prepared.Search(t.Context(), "retry", 10)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("snapshot changed after filesystem mutation: %+v, %v; want %+v", got, err, want)
			}
		})
	}
}

func TestSearchSnapshotIsolatesCustomBackendAndResults(t *testing.T) {
	root := fixture(t)
	addModule(t, root)
	r := mustLoad(t, root)
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	want, err := r.Search(t.Context(), "context", 10)
	if err != nil || len(want) != 2 {
		t.Fatalf("baseline results = %+v, %v", want, err)
	}
	calls := 0
	r.Backend = snapshotBackendFunc(func(ctx context.Context, documents []Document, query string) (map[string]int, error) {
		calls++
		if !reflect.DeepEqual(documents, r.documents) {
			t.Error("backend received documents changed by an earlier search")
		}
		scores, err := (FullTextBackend{}).Search(ctx, documents, query)
		for i := range documents {
			documents[i].Path = "changed"
			documents[i].Body = "changed"
			documents[i].Metadata.Title = "changed"
			documents[i].Metadata.Tags[0] = "changed"
			documents[i].Metadata.Sources[0].ID = "changed"
			if len(documents[i].Metadata.Evals) > 0 {
				documents[i].Metadata.Evals[0] = "changed"
			}
		}
		return scores, err
	})
	prepared := prepareSnapshot(t, r)
	for range 3 {
		got, err := prepared.Search(t.Context(), "context", 10)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("backend mutation affected snapshot results: %+v, %v; want %+v", got, err, want)
		}
		got[0].Title = "caller changed result"
		got[0].Relevance = -1
	}
	if calls != 3 {
		t.Fatalf("backend calls = %d; want 3", calls)
	}
}

func TestSearchSnapshotCapturesBackend(t *testing.T) {
	r := mustLoad(t, fixture(t))
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	prepared := prepareSnapshot(t, r)
	backendErr := errors.New("backend unavailable")
	r.Backend = snapshotBackendFunc(func(context.Context, []Document, string) (map[string]int, error) {
		return nil, backendErr
	})
	if got, err := prepared.Search(t.Context(), "retry", 10); err != nil || len(got) != 1 {
		t.Fatalf("existing snapshot did not retain backend: %+v, %v", got, err)
	}
	if _, err := r.Search(t.Context(), "retry", 10); !errors.Is(err, backendErr) {
		t.Fatalf("Repository.Search did not use replacement backend: %v", err)
	}
	if _, err := prepareSnapshot(t, r).Search(t.Context(), "retry", 10); !errors.Is(err, backendErr) {
		t.Fatalf("new snapshot did not propagate backend error: %v", err)
	}
}

func TestSearchSnapshotErrors(t *testing.T) {
	r := mustLoad(t, fixture(t))
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	prepared := prepareSnapshot(t, r)
	r.Backend = nil
	if _, err := r.PrepareSearch(t.Context()); err == nil || !strings.Contains(err.Error(), "backend is nil") {
		t.Fatalf("nil backend accepted: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := r.PrepareSearch(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled preparation error = %v", err)
	}
	// Invalid requests must be rejected before filesystem/backend validation.
	if err := os.Remove(filepath.Join(r.root, "rag/index/snapshot.json")); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name  string
		ctx   context.Context
		query string
		limit int
		want  string
	}{
		{"empty query", t.Context(), "", 10, "query must not be empty"},
		{"whitespace query", t.Context(), " \t ", 10, "query must not be empty"},
		{"zero limit", t.Context(), "retry", 0, "limit must be positive"},
		{"negative limit", t.Context(), "retry", -1, "limit must be positive"},
		{"canceled context", ctx, "retry", 10, "context canceled"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			for _, search := range []func(context.Context, string, int) ([]Result, error){r.Search, prepared.Search} {
				if _, err := search(scenario.ctx, scenario.query, scenario.limit); err == nil || !strings.Contains(err.Error(), scenario.want) {
					t.Errorf("search error = %v; want %q", err, scenario.want)
				}
			}
		})
	}
}

func TestSearchSnapshotConcurrentSearches(t *testing.T) {
	r := mustLoad(t, fixture(t))
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	prepared := prepareSnapshot(t, r)
	want, err := prepared.Search(t.Context(), "retry", 10)
	if err != nil || len(want) != 1 {
		t.Fatalf("baseline results = %+v, %v", want, err)
	}
	for range 8 {
		t.Run("reader", func(t *testing.T) {
			t.Parallel()
			for range 10 {
				got, err := prepared.Search(t.Context(), "retry", 10)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("concurrent results = %+v, %v; want %+v", got, err, want)
				}
				got[0].Title = "caller changed result"
			}
		})
	}
}
