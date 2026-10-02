package kb

import (
	"encoding/json"
	"reflect"
	"testing"
)

func referenceSearchDocumentsEqual(expected, actual []Document) bool {
	expectedJSON, _ := json.Marshal(expected)
	actualJSON, _ := json.Marshal(actual)
	return string(expectedJSON) == string(actualJSON)
}

func TestSearchDocumentsEqualPreservesJSONComparison(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		expected, actual []Document
		want             bool
	}{
		{"nil", nil, nil, true},
		{"empty", []Document{}, []Document{}, true},
		{"nil versus empty corpus", nil, []Document{}, false},
		{"equal document", []Document{{Body: "検索 retry"}}, []Document{{Body: "検索 retry"}}, true},
		{"changed body", []Document{{Body: "retry"}}, []Document{{Body: "different"}}, false},
		{"omitted evals", []Document{{Metadata: Metadata{Evals: nil}}}, []Document{{Metadata: Metadata{Evals: []string{}}}}, true},
		{"non-omitted tags", []Document{{Metadata: Metadata{Tags: nil}}}, []Document{{Metadata: Metadata{Tags: []string{}}}}, false},
		{"non-omitted sources", []Document{{Metadata: Metadata{Sources: nil}}}, []Document{{Metadata: Metadata{Sources: []SourceRef{}}}}, false},
		{"different invalid bytes encode identically", []Document{{Body: "bad\xff"}}, []Document{{Body: "bad\xfe"}}, true},
		// Keep the current toolchain's invalid-byte normalization equivalent to
		// an actual replacement rune, just as the original JSON comparison does.
		{"invalid UTF-8 versus replacement rune", []Document{{Body: "bad\xff"}}, []Document{{Body: "bad\ufffd"}}, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			want := referenceSearchDocumentsEqual(scenario.expected, scenario.actual)
			if want != scenario.want {
				t.Fatalf("original comparison = %v; expected %v", want, scenario.want)
			}
			if got := searchDocumentsEqual(scenario.expected, scenario.actual); got != want {
				t.Fatalf("comparison = %v; original = %v", got, want)
			}
		})
	}
}

func BenchmarkSearchDocumentsEqual(b *testing.B) {
	documents := benchmarkFullTextDocuments()
	encoded, err := json.Marshal(documents)
	if err != nil {
		b.Fatal(err)
	}
	var indexed []Document
	if err := json.Unmarshal(encoded, &indexed); err != nil {
		b.Fatal(err)
	}
	if !reflect.DeepEqual(documents, indexed) {
		b.Fatal("benchmark must use separately decoded equal documents")
	}
	for name, equal := range map[string]func([]Document, []Document) bool{
		"json":                 referenceSearchDocumentsEqual,
		"structural-fast-path": searchDocumentsEqual,
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if !equal(documents, indexed) {
					b.Fatal("equal documents rejected")
				}
			}
		})
	}
}
