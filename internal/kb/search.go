package kb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

// SearchSnapshot は検証済みの入力と時刻を固定した検索用スナップショット。
// 作成後のファイル更新は反映しない。更新検知が必要な通常の検索には Repository.Search を使う。
// 既定の FullTextBackend では複数 goroutine から検索できる。
// 独自 Backend を使う場合は、その Backend 自体も並行呼び出しに対応する必要がある。
type SearchSnapshot struct {
	documents []Document
	results   []Result
	backend   Backend
}

// PrepareSearch は入力と索引を一度検証し、同じ corpus を繰り返し評価するために固定する。
// 鮮度の基準時刻は呼び出し元 Repository を Load したときの UTC 時刻となる。
func (r *Repository) PrepareSearch(ctx context.Context) (*SearchSnapshot, error) {
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
	if r.Backend == nil {
		return nil, errors.New("search backend is nil")
	}
	results := make([]Result, len(index.Documents))
	for i, doc := range index.Documents {
		results[i] = current.result(doc, 0)
	}
	return &SearchSnapshot{documents: index.Documents, results: results, backend: r.Backend}, nil
}

// Search は呼び出しごとに入力・索引の整合性を検証する。
// 鮮度は索引作成時の状態を使わず、呼び出し側の UTC 時刻で判定する。
func (r *Repository) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if err := validateSearch(ctx, query, limit); err != nil {
		return nil, err
	}
	prepared, err := r.PrepareSearch(ctx)
	if err != nil {
		return nil, err
	}
	return prepared.Search(ctx, query, limit)
}

func validateSearch(ctx context.Context, query string, limit int) error {
	if strings.TrimSpace(query) == "" {
		return errors.New("search query must not be empty")
	}
	if limit < 1 {
		return errors.New("search limit must be positive")
	}
	return ctx.Err()
}

// Search は準備時点の入力を検索する。返した結果を変更しても後続の検索に影響しない。
func (s *SearchSnapshot) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if err := validateSearch(ctx, query, limit); err != nil {
		return nil, err
	}
	documents := s.documents
	switch s.backend.(type) {
	case FullTextBackend, *FullTextBackend:
		// 標準 backend は入力を変更しないので、検証済みの内容を共有できる。
	default:
		// 独自 backend に可変 slice を渡しても snapshot 自体は変更させない。
		documents = cloneDocuments(documents)
	}
	scores, err := s.backend.Search(ctx, documents, query)
	if err != nil {
		return nil, err
	}
	results := []Result{}
	for i, doc := range s.documents {
		if score, ok := scores[doc.Path]; ok && score > 0 {
			result := s.results[i]
			result.Relevance = score
			results = append(results, result)
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

func cloneDocuments(documents []Document) []Document {
	cloned := slices.Clone(documents)
	for i := range cloned {
		m := &cloned[i].Metadata
		m.Tags = slices.Clone(m.Tags)
		m.Sources = slices.Clone(m.Sources)
		m.Evals = slices.Clone(m.Evals)
	}
	return cloned
}
