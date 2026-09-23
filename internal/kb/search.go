package kb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type snapshot struct {
	SchemaVersion int        `json:"schema_version"`
	Fingerprint   string     `json:"fingerprint"`
	Documents     []Document `json:"documents"`
}

// Search は空白区切りの語を小文字化して AND 検索する。
// タイトルとタグを加点するが、鮮度と参照元の信頼度の優先順位は変えない。
func (FullTextBackend) Search(ctx context.Context, documents []Document, query string) (map[string]int, error) {
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

func (r *Repository) writeGenerated(rel string, value any) (resultErr error) {
	if !strings.HasPrefix(rel, "rag/documents/") && !strings.HasPrefix(rel, "rag/metadata/") && !strings.HasPrefix(rel, "rag/index/") {
		return fmt.Errorf("refusing to write outside generated directories: %s", rel)
	}
	path, err := r.safePath(rel, true)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".kb-*")
	if err != nil {
		return err
	}
	tempPath := tmp.Name()
	closed, published := false, false
	defer func() {
		// 元の書き込みエラーを保ったまま、後処理の失敗も呼び出し元へ返す。
		if !closed {
			if err := tmp.Close(); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("close generated temporary file: %w", err))
			}
		}
		if err := os.Remove(tempPath); err != nil && (!published || !errors.Is(err, os.ErrNotExist)) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove generated temporary file: %w", err))
		}
	}()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	err = tmp.Close()
	closed = true
	if err != nil {
		return err
	}
	// 生成先が途中で置き換えられた場合に備え、公開直前にも確認する。
	if _, err := r.safePath(rel, true); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	published = true
	return nil
}

// Index は検索に必要な内容を含む単一スナップショットを最後に原子的に公開する。
// documents/ と metadata/ は確認用の出力であり、検索時には読み込まない。
func (r *Repository) Index() error {
	current, err := Load(r.root, r.now)
	if err != nil {
		return err
	}
	if err := current.writeGenerated("rag/documents/documents.json", current.documents); err != nil {
		return err
	}
	metadata := make([]Result, 0, len(current.documents))
	for _, doc := range current.documents {
		metadata = append(metadata, current.result(doc, 0))
	}
	if err := current.writeGenerated("rag/metadata/metadata.json", metadata); err != nil {
		return err
	}
	return current.writeGenerated("rag/index/snapshot.json", snapshot{1, current.fingerprint, current.documents})
}

// Search は入力のフィンガープリントを検証してから索引を使う。
// 鮮度は索引作成時の状態を使わず、呼び出し側の UTC 時刻で判定する。
func (r *Repository) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("search query must not be empty")
	}
	if limit < 1 {
		return nil, errors.New("search limit must be positive")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	current, err := Load(r.root, r.now)
	if err != nil {
		return nil, err
	}
	var index snapshot
	if err := current.readJSON("rag/index/snapshot.json", &index); err != nil {
		return nil, fmt.Errorf("index unavailable; run kb index: %w", err)
	}
	if index.SchemaVersion != 1 || index.Fingerprint != current.fingerprint {
		return nil, errors.New("index is out of date; run kb index")
	}
	// 元のフィンガープリントが残っていても、生成データの破損や直接編集を拒否する。
	expected, _ := json.Marshal(current.documents)
	actual, _ := json.Marshal(index.Documents)
	if string(expected) != string(actual) {
		return nil, errors.New("index content is inconsistent; run kb index")
	}
	backend := r.Backend
	if backend == nil {
		return nil, errors.New("search backend is nil")
	}
	scores, err := backend.Search(ctx, index.Documents, query)
	if err != nil {
		return nil, err
	}
	results := []Result{}
	for _, doc := range index.Documents {
		if score, ok := scores[doc.Path]; ok && score > 0 {
			results = append(results, current.result(doc, score))
		}
	}
	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.Stale != b.Stale {
			return !a.Stale
		}
		if trustRank(a.Trust) != trustRank(b.Trust) {
			return trustRank(a.Trust) < trustRank(b.Trust)
		}
		if a.Relevance != b.Relevance {
			return a.Relevance > b.Relevance
		}
		return a.Path < b.Path
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
