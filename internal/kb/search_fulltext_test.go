package kb

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
)

// Keep the original implementation independent of the optimized helpers so a
// shared normalization or scoring bug cannot make both sides agree by accident.
func referenceFullTextSearch(ctx context.Context, documents []Document, query string) (map[string]int, error) {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil, errors.New("search query must not be empty")
	}
	scores := make(map[string]int)
	for _, doc := range documents {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		m := doc.Metadata
		title := strings.ToLower(m.Title)
		tags := strings.ToLower(strings.Join(m.Tags, " "))
		all := strings.ToLower(strings.Join([]string{doc.Path, m.ID, m.Title, m.Kind, m.Technology, m.Version, tags, doc.Body}, " "))
		score := 0
		for _, term := range terms {
			if !strings.Contains(all, term) {
				score = 0
				break
			}
			score += strings.Count(all, term)
			if strings.Contains(title, term) {
				score += 10
			}
			if strings.Contains(tags, term) {
				score += 5
			}
		}
		if score > 0 {
			scores[doc.Path] = score
		}
	}
	return scores, nil
}

func preparedFullTextDocuments(documents []Document) []fullTextDocument {
	prepared := make([]fullTextDocument, len(documents))
	for i, doc := range documents {
		prepared[i] = prepareFullTextDocument(doc)
	}
	return prepared
}

func TestPreparedFullTextMatchesReference(t *testing.T) {
	documents := []Document{
		{Path: "knowledge/Go/context.md", Metadata: Metadata{
			ID: "Context", Title: "Context Cancellation RETRY", Kind: "knowledge", Technology: "Go", Version: "1.27",
			Tags: []string{"retry", "cancel", "C++"},
		}, Body: "Retry retrying retries. aaa aaaa. HTTP/2 SQL_tag foo-bar x.y"},
		{Path: "日本語/検索.md", Metadata: Metadata{
			ID: "unicode", Title: "検索 ÄÖÜ İ Σ ς K", Kind: "pattern", Technology: "unicode", Version: "VERSION",
			Tags: []string{"CAFÉ", "e\u0301", "ß", "emoji🙂"},
		}, Body: "日本語の検索\n\tΣσς ééé\u00a0RETRY"},
		{Path: "duplicate", Metadata: Metadata{Title: "retry"}, Body: "retry retry"},
		{Path: "duplicate", Metadata: Metadata{Tags: []string{}}, Body: "retry"},
		{Path: "empty"},
		// Direct backends accept arbitrary strings even though repository loading
		// rejects invalid UTF-8 in metadata; keep their existing behavior too.
		{Path: "invalid-utf8", Metadata: Metadata{Title: "A\xffZ"}, Body: "B\xfeY"},
	}
	queries := []string{
		"", " \t\n\u00a0\u2003", "RETRY", "retry retry", "cancel retry", "context cancellation missing",
		"try", "aaa", "a a", "C++", "HTTP/2", "SQL_tag", "foo-bar", "x.y", "1.27", "knowledge/Go",
		"検索", "äöü", "İ", "Σ", "ς", "K", "CAFÉ", "e\u0301", "ß", "🙂", "éé", "\xff", "�",
		"\u2003ReTrY\u00a0CaNcEl\t", "VERSION", "PATTERN", "Go", "duplicate", "unmatched",
	}
	// Deterministic mixed-field examples exercise substring matches, repeated
	// terms, punctuation, case conversion and Unicode without a large fixture.
	random := rand.New(rand.NewPCG(1, 2))
	parts := []string{"Ab", "ab", "BA", "aa", " ", "\t", "-", "/", "Σ", "ς", "é", "検索", "🙂"}
	text := func() string {
		var builder strings.Builder
		for range 12 {
			builder.WriteString(parts[random.IntN(len(parts))])
		}
		return builder.String()
	}
	for range 20 {
		documents = append(documents, Document{Path: text(), Metadata: Metadata{
			ID: text(), Title: text(), Kind: text(), Technology: text(), Version: text(), Tags: []string{text(), text()},
		}, Body: text()})
		queries = append(queries, parts[random.IntN(len(parts))], parts[random.IntN(len(parts))]+" "+parts[random.IntN(len(parts))])
	}
	for _, corpus := range [][]Document{nil, {}, documents} {
		prepared := preparedFullTextDocuments(corpus)
		for _, query := range queries {
			want, wantErr := referenceFullTextSearch(t.Context(), corpus, query)
			for name, search := range map[string]func() (map[string]int, error){
				"direct":   func() (map[string]int, error) { return (FullTextBackend{}).Search(t.Context(), corpus, query) },
				"prepared": func() (map[string]int, error) { return searchFullText(t.Context(), prepared, query) },
			} {
				got, err := search()
				if !reflect.DeepEqual(got, want) || fmt.Sprint(err) != fmt.Sprint(wantErr) {
					t.Fatalf("%s search %q (%d documents) = %v, %v; want %v, %v", name, query, len(corpus), got, err, want, wantErr)
				}
			}
		}
	}
}

type cancelAfterContextChecks struct {
	context.Context
	remaining int
}

func (c *cancelAfterContextChecks) Err() error {
	if c.remaining > 0 {
		c.remaining--
		return nil
	}
	return context.Canceled
}

func TestPreparedFullTextCancellation(t *testing.T) {
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	documents := []Document{{Path: "first", Body: "retry"}, {Path: "second", Body: "retry"}}
	for _, scenario := range []struct {
		name      string
		documents []Document
		query     string
		context   func() context.Context
	}{
		{"canceled", documents, "retry", func() context.Context { return canceled }},
		{"canceled empty corpus", nil, "retry", func() context.Context { return canceled }},
		{"empty query wins over cancellation", documents, "\t", func() context.Context { return canceled }},
		{"cancel after partial matches", documents, "retry", func() context.Context { return &cancelAfterContextChecks{Context: t.Context(), remaining: 1} }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			want, wantErr := referenceFullTextSearch(scenario.context(), scenario.documents, scenario.query)
			prepared := preparedFullTextDocuments(scenario.documents)
			for name, search := range map[string]func() (map[string]int, error){
				"direct": func() (map[string]int, error) {
					return (FullTextBackend{}).Search(scenario.context(), scenario.documents, scenario.query)
				},
				"prepared": func() (map[string]int, error) {
					return searchFullText(scenario.context(), prepared, scenario.query)
				},
			} {
				got, err := search()
				if !reflect.DeepEqual(got, want) || fmt.Sprint(err) != fmt.Sprint(wantErr) {
					t.Fatalf("%s = %v, %v; want %v, %v", name, got, err, want, wantErr)
				}
			}
		})
	}
}

// An embedded built-in backend is still a custom backend: its override must run.
type overridingFullTextBackend struct {
	FullTextBackend
	query string
}

func (b *overridingFullTextBackend) Search(_ context.Context, documents []Document, query string) (map[string]int, error) {
	b.query = query
	return map[string]int{documents[0].Path: 123}, nil
}

func TestSearchSnapshotPreparesOnlyBuiltinFullText(t *testing.T) {
	r := mustLoad(t, fixture(t))
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	query := "\tReTrY \n"
	for _, backend := range []Backend{FullTextBackend{}, &FullTextBackend{}} {
		r.Backend = backend
		prepared := prepareSnapshot(t, r)
		if prepared.fullText == nil {
			t.Fatalf("%T did not prepare searchable text", backend)
		}
		want, err := referenceFullTextSearch(t.Context(), r.documents, query)
		if err != nil {
			t.Fatal(err)
		}
		got, err := prepared.Search(t.Context(), query, 10)
		if err != nil || len(got) != 1 || got[0].Relevance != want[got[0].Path] {
			t.Fatalf("%T result = %+v, %v; want scores %v", backend, got, err, want)
		}
	}
	backend := &overridingFullTextBackend{}
	r.Backend = backend
	prepared := prepareSnapshot(t, r)
	if prepared.fullText != nil {
		t.Fatal("custom backend was replaced with prepared full-text search")
	}
	got, err := prepared.Search(t.Context(), query, 10)
	if err != nil || len(got) != 1 || got[0].Relevance != 123 || backend.query != query {
		t.Fatalf("custom backend result = %+v, %v; received query %q", got, err, backend.query)
	}
}

func TestSearchSnapshotPreservesNilFullTextBackend(t *testing.T) {
	r := mustLoad(t, fixture(t))
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	var backend *FullTextBackend
	r.Backend = backend
	prepared := prepareSnapshot(t, r)
	if prepared.fullText != nil {
		t.Fatal("nil backend must not be replaced with a working full-text backend")
	}
	defer func() {
		if recover() == nil {
			t.Error("nil backend no longer preserves its original Search panic")
		}
	}()
	_, _ = prepared.Search(t.Context(), "retry", 10)
}

func TestSearchSnapshotCancelsFullTextPreparation(t *testing.T) {
	r := mustLoad(t, fixture(t))
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	// loadSearchIndex checks three times; the next check is the first document
	// normalization, before any result or prepared snapshot is exposed.
	ctx := &cancelAfterContextChecks{Context: t.Context(), remaining: 3}
	if prepared, err := r.PrepareSearch(ctx); prepared != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled preparation = %+v, %v; want nil, context.Canceled", prepared, err)
	}
}

func benchmarkFullTextDocuments() []Document {
	// Deliberately synthetic: use stable document count/body size for repeatable
	// comparisons without loading or modifying the changing repository corpus.
	documents := make([]Document, 256)
	for i := range documents {
		documents[i] = Document{Path: fmt.Sprintf("knowledge/go/document-%d.md", i), Metadata: Metadata{
			ID: fmt.Sprintf("document-%d", i), Title: "Context cancellation and RETRY", Kind: "knowledge", Technology: "go", Version: "1.27",
			Tags: []string{"context", "cancel", "retry"},
		}, Body: strings.Repeat("Context cancellation, bounded RETRY and 日本語の検索. ", 64)}
	}
	return documents
}

func BenchmarkFullTextSearch(b *testing.B) {
	documents := benchmarkFullTextDocuments()
	prepared := preparedFullTextDocuments(documents)
	for _, query := range []string{"context", "cancel retry", "absent-term"} {
		b.Run(query, func(b *testing.B) {
			b.Run("reference", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := referenceFullTextSearch(b.Context(), documents, query); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("direct", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := (FullTextBackend{}).Search(b.Context(), documents, query); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("prepare-and-search", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := searchFullText(b.Context(), preparedFullTextDocuments(documents), query); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("prepared", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := searchFullText(b.Context(), prepared, query); err != nil {
						b.Fatal(err)
					}
				}
				bytes := 0
				for _, doc := range prepared {
					bytes += len(doc.title) + len(doc.tags) + len(doc.all)
				}
				b.ReportMetric(float64(bytes), "text-B")
			})
		})
	}
	b.Run("prepare", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = preparedFullTextDocuments(documents)
		}
	})
}
