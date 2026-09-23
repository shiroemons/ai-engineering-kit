// Package kb は参照資料を検証し、再生成可能なローカル検索索引を構築する。
package kb

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

var trustOrder = []string{"official", "maintainer", "primary-source", "community", "unknown"}
var identifier = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
var commitSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

type SourceRef struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	Type string `json:"type"`
}
type Metadata struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Kind        string      `json:"kind"`
	Technology  string      `json:"technology"`
	Version     string      `json:"version"`
	Tags        []string    `json:"tags"`
	Sources     []SourceRef `json:"sources"`
	RetrievedAt string      `json:"retrieved_at"`
	ExpiresAt   string      `json:"expires_at"`
	Trust       string      `json:"trust"`
	Status      string      `json:"status"`
	Evals       []string    `json:"evals,omitempty"`
}
type Source struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	RepositoryURL string `json:"repository_url"`
	CommitSHA     string `json:"commit_sha"`
	Version       string `json:"version"`
	RetrievedAt   string `json:"retrieved_at"`
	License       string `json:"license"`
	Trust         string `json:"trust"`
	Type          string `json:"type"`
	Purpose       string `json:"purpose"`
}
type freshnessConfig struct {
	SourceTypes  map[string]int `json:"source_types"`
	Technologies map[string]int `json:"technologies"`
}
type sourceConfig struct {
	TrustOrder        []string `json:"trust_order"`
	AllowedURLSchemes []string `json:"allowed_url_schemes"`
}
type moduleManifest struct {
	API         string   `json:"api"`
	Tests       []string `json:"tests"`
	Evals       []string `json:"evals"`
	Provenance  string   `json:"provenance"`
	ReviewedBy  string   `json:"reviewed_by"`
	ReviewedAt  string   `json:"reviewed_at"`
	Concurrency string   `json:"concurrency"`
}

// Document は将来の embedding や hybrid 検索でも共有する入力形式。
type Document struct {
	Path            string   `json:"path"`
	Metadata        Metadata `json:"metadata"`
	Body            string   `json:"body"`
	EffectiveExpiry string   `json:"effective_expiry"`
}
type Result struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	Technology  string `json:"technology"`
	Version     string `json:"version"`
	Relevance   int    `json:"relevance"`
	Trust       string `json:"trust"`
	RetrievedAt string `json:"retrieved_at"`
	ExpiresAt   string `json:"expires_at"`
	Stale       bool   `json:"stale"`
}
type Stats struct {
	Documents int `json:"documents"`
	Knowledge int `json:"knowledge"`
	Patterns  int `json:"patterns"`
	Modules   int `json:"modules"`
	Sources   int `json:"sources"`
	Active    int `json:"active"`
	Stale     int `json:"stale"`
}

// Backend は関連度を返す。鮮度と信頼度の優先順位は Repository が維持する。
type Backend interface {
	Search(context.Context, []Document, string) (map[string]int, error)
}
type FullTextBackend struct{}
type Repository struct {
	root         string
	now          time.Time
	freshness    freshnessConfig
	sourcePolicy sourceConfig
	sources      map[string]Source
	documents    []Document
	inputs       map[string][]byte
	fingerprint  string
	Backend      Backend
}

func decode(data []byte, target any) error {
	// v2 の既定動作で重複キー・不正 UTF-8・後続データも拒否する。
	return jsonv2.Unmarshal(data, target, jsonv2.RejectUnknownMembers(true))
}
func nonempty(s string) bool { return strings.TrimSpace(s) != "" }
func date(value string) (time.Time, error) {
	t, err := time.Parse(dateLayout, value)
	if err != nil || t.Format(dateLayout) != value {
		return time.Time{}, fmt.Errorf("invalid YYYY-MM-DD date %q", value)
	}
	return t, nil
}
func trustRank(s string) int {
	for i, item := range trustOrder {
		if s == item {
			return i
		}
	}
	return -1
}

// safePath は中間ディレクトリを含めてパストラバーサルとシンボリックリンクを拒否する。
func (r *Repository) safePath(rel string, missing bool) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "\\") || filepath.Clean(rel) != rel || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("unsafe repository path %q", rel)
	}
	current := r.root
	for part := range strings.SplitSeq(filepath.ToSlash(rel), "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && missing {
			continue
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("symlink is not allowed: %s", rel)
		}
	}
	return current, nil
}
func (r *Repository) read(rel string) ([]byte, error) {
	path, err := r.safePath(rel, false)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", rel)
	}
	data, err := os.ReadFile(path)
	if err == nil {
		r.inputs[rel] = data
	}
	return data, err
}
func (r *Repository) readJSON(rel string, target any) error {
	b, err := r.read(rel)
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	if err = decode(b, target); err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	return nil
}
func (r *Repository) walk(rel string, visit func(string) error) error {
	path, err := r.safePath(rel, false)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("expected directory: %s", rel)
	}
	return filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(r.root, path)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed: %s", relative)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("not a regular file: %s", relative)
		}
		return visit(filepath.ToSlash(relative))
	})
}

// Load は読み込み時に入力を検証する。now を明示して UTC の鮮度判定を再現可能にする。
func Load(root string, now time.Time) (*Repository, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	r := &Repository{root: resolved, now: now.UTC(), sources: map[string]Source{}, inputs: map[string][]byte{}, documents: []Document{}, Backend: FullTextBackend{}}
	if err := r.readJSON("config/freshness.json", &r.freshness); err != nil {
		return nil, err
	}
	if err := r.readJSON("config/sources.json", &r.sourcePolicy); err != nil {
		return nil, err
	}
	if len(r.freshness.SourceTypes) == 0 || len(r.freshness.Technologies) == 0 {
		return nil, errors.New("freshness requires nonempty source_types and technologies")
	}
	for _, mapping := range []map[string]int{r.freshness.SourceTypes, r.freshness.Technologies} {
		for key, ttl := range mapping {
			if !nonempty(key) || ttl <= 0 || ttl > 36500 {
				return nil, fmt.Errorf("invalid TTL for %q: require 1..36500 days", key)
			}
		}
	}
	if strings.Join(r.sourcePolicy.TrustOrder, ",") != strings.Join(trustOrder, ",") {
		return nil, errors.New("trust_order must be official, maintainer, primary-source, community, unknown")
	}
	if len(r.sourcePolicy.AllowedURLSchemes) == 0 {
		return nil, errors.New("allowed_url_schemes must not be empty")
	}
	for _, scheme := range r.sourcePolicy.AllowedURLSchemes {
		if scheme != "https" && scheme != "http" {
			return nil, fmt.Errorf("unsupported URL scheme %q", scheme)
		}
	}
	if err := r.walk("sources/catalog", func(path string) error {
		if filepath.Ext(path) != ".json" {
			return nil
		}
		var src Source
		if err := r.readJSON(path, &src); err != nil {
			return err
		}
		if err := r.validateSource(src); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if _, exists := r.sources[src.ID]; exists {
			return fmt.Errorf("duplicate source id %q", src.ID)
		}
		r.sources[src.ID] = src
		return nil
	}); err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	if err := r.validateModuleLayout(); err != nil {
		return nil, err
	}
	for _, dir := range []string{"knowledge", "patterns", "modules"} {
		err := r.walk(dir, func(path string) error {
			if filepath.Ext(path) != ".md" || strings.HasSuffix(path, ".template.md") {
				return nil
			}
			parts := strings.Split(path, "/")
			if dir == "modules" && (len(parts) != 4 || parts[3] != "README.md") {
				return nil
			}
			data, err := r.read(path)
			if err != nil {
				return err
			}
			if filepath.Base(path) == "README.md" && len(parts) <= 3 && !bytes.HasPrefix(data, []byte("---\n")) {
				delete(r.inputs, path)
				return nil
			}
			meta, body, err := parseMarkdown(data)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			expected := map[string]string{"knowledge": "knowledge", "patterns": "pattern", "modules": "module"}[dir]
			if meta.Kind != expected {
				return fmt.Errorf("%s: kind must be %s", path, expected)
			}
			expiry, err := r.validateMetadata(meta)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if ids[meta.ID] {
				return fmt.Errorf("duplicate document id %q", meta.ID)
			}
			ids[meta.ID] = true
			if !nonempty(body) {
				return fmt.Errorf("%s: document body is empty", path)
			}
			if dir == "modules" {
				if err := r.validateModule(filepath.Dir(path), meta); err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
			}
			r.documents = append(r.documents, Document{Path: path, Metadata: meta, Body: body, EffectiveExpiry: expiry.Format(dateLayout)})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(r.documents, func(i, j int) bool { return r.documents[i].Path < r.documents[j].Path })
	r.fingerprint = r.inputFingerprint()
	return r, nil
}
func (r *Repository) validateModuleLayout() error {
	path, err := r.safePath("modules", false)
	if err != nil {
		return err
	}
	return filepath.WalkDir(path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(r.root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed: %s", relative)
		}
		if entry.IsDir() && len(parts) == 3 {
			if _, err := r.read(filepath.ToSlash(filepath.Join(relative, "README.md"))); err != nil {
				return fmt.Errorf("module directory %s requires README.md: %w", relative, err)
			}
		}
		if !entry.IsDir() && len(parts) < 4 && (filepath.Ext(path) == ".go" || filepath.Ext(path) == ".rb") {
			return fmt.Errorf("module code must be under modules/<language>/<module>: %s", relative)
		}
		return nil
	})
}
func parseMarkdown(data []byte) (Metadata, string, error) {
	var m Metadata
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return m, "", errors.New("expected JSON front matter between --- lines")
	}
	rest := data[4:]
	before, after, ok := bytes.Cut(rest, []byte("\n---\n"))
	if !ok {
		return m, "", errors.New("missing front matter closing --- line")
	}
	if err := decode(before, &m); err != nil {
		return m, "", fmt.Errorf("invalid JSON front matter: %w", err)
	}
	return m, string(after), nil
}
func (r *Repository) validURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("invalid source URL %q", value)
	}
	if slices.Contains(r.sourcePolicy.AllowedURLSchemes, u.Scheme) {
		return nil
	}
	return fmt.Errorf("URL scheme is not allowed: %q", value)
}
func (r *Repository) validateSource(src Source) error {
	if !identifier.MatchString(src.ID) || !nonempty(src.Version) || !nonempty(src.License) || !nonempty(src.Purpose) || trustRank(src.Trust) < 0 {
		return errors.New("source requires valid id, version, license, purpose and trust")
	}
	if err := r.validURL(src.URL); err != nil {
		return err
	}
	if src.RepositoryURL != "" {
		if err := r.validURL(src.RepositoryURL); err != nil {
			return err
		}
	}
	if _, ok := r.freshness.SourceTypes[src.Type]; !ok {
		return fmt.Errorf("unknown source type %q", src.Type)
	}
	retrieved, err := date(src.RetrievedAt)
	if err != nil {
		return err
	}
	if retrieved.After(r.now) {
		return errors.New("source retrieved_at is in the future")
	}
	if src.CommitSHA != "" && !commitSHA.MatchString(src.CommitSHA) {
		return errors.New("commit_sha must be 40 hexadecimal characters")
	}
	if src.Type == "github_repository_analysis" && (src.RepositoryURL == "" || !commitSHA.MatchString(src.CommitSHA)) {
		return errors.New("repository analysis requires repository_url and pinned 40-character commit_sha")
	}
	return nil
}
func (r *Repository) validateMetadata(m Metadata) (time.Time, error) {
	if !identifier.MatchString(m.ID) || !nonempty(m.Title) || !nonempty(m.Technology) || !nonempty(m.Version) {
		return time.Time{}, errors.New("metadata requires valid id and nonempty title, technology, version")
	}
	if m.Status != "active" && m.Status != "stale" {
		return time.Time{}, errors.New("status must be active or stale")
	}
	rank := trustRank(m.Trust)
	if rank < 0 {
		return time.Time{}, fmt.Errorf("unknown trust %q", m.Trust)
	}
	retrieved, err := date(m.RetrievedAt)
	if err != nil {
		return time.Time{}, err
	}
	expires, err := date(m.ExpiresAt)
	if err != nil {
		return time.Time{}, err
	}
	if retrieved.After(r.now) {
		return time.Time{}, errors.New("retrieved_at is in the future")
	}
	if !expires.After(retrieved) {
		return time.Time{}, errors.New("expires_at must follow retrieved_at")
	}
	if len(m.Sources) == 0 {
		return time.Time{}, errors.New("at least one source is required")
	}
	seen := map[string]bool{}
	for _, ref := range m.Sources {
		src, ok := r.sources[ref.ID]
		if !ok {
			return time.Time{}, fmt.Errorf("unknown source id %q", ref.ID)
		}
		if seen[ref.ID] {
			return time.Time{}, fmt.Errorf("duplicate source reference %q", ref.ID)
		}
		seen[ref.ID] = true
		if ref.URL != src.URL || ref.Type != src.Type {
			return time.Time{}, fmt.Errorf("source %q URL/type does not match catalog", ref.ID)
		}
		if rank < trustRank(src.Trust) {
			return time.Time{}, errors.New("document trust cannot exceed weakest source trust")
		}
		srcRetrieved, _ := date(src.RetrievedAt)
		// 文書の日付だけを更新して、古い参照元の確認日を延長させない。
		base := retrieved
		if srcRetrieved.Before(base) {
			base = srcRetrieved
		}
		deadline := base.AddDate(0, 0, r.freshness.SourceTypes[src.Type])
		if deadline.Before(expires) {
			expires = deadline
		}
		if m.Kind == "module" && strings.EqualFold(strings.TrimSpace(src.License), "unknown") {
			return time.Time{}, errors.New("module cannot depend on a source with unknown license")
		}
	}
	if ttl, ok := r.freshness.Technologies[m.Technology]; ok {
		deadline := retrieved.AddDate(0, 0, ttl)
		if deadline.Before(expires) {
			expires = deadline
		}
	}
	for _, tag := range m.Tags {
		if !nonempty(tag) {
			return time.Time{}, errors.New("tags must not contain empty values")
		}
	}
	for _, eval := range m.Evals {
		if err := r.validateEval(eval); err != nil {
			return time.Time{}, err
		}
	}
	return expires, nil
}
func (r *Repository) validateEval(path string) error {
	if !strings.HasPrefix(path, "evals/") || filepath.Ext(path) != ".json" {
		return fmt.Errorf("eval must be a repository-relative evals/*.json path: %s", path)
	}
	data, err := r.read(path)
	if err != nil {
		return err
	}
	var v any
	if err := decode(data, &v); err != nil {
		return fmt.Errorf("invalid eval %s: %w", path, err)
	}
	if v == nil {
		return fmt.Errorf("empty eval %s", path)
	}
	switch value := v.(type) {
	case map[string]any:
		if len(value) == 0 {
			return fmt.Errorf("empty eval %s", path)
		}
	case []any:
		if len(value) == 0 {
			return fmt.Errorf("empty eval %s", path)
		}
	default:
		return fmt.Errorf("eval must contain a JSON object or array: %s", path)
	}
	return nil
}
func (r *Repository) validateModule(dir string, m Metadata) error {
	var manifest moduleManifest
	if err := r.readJSON(filepath.ToSlash(filepath.Join(dir, "module.json")), &manifest); err != nil {
		return err
	}
	if !nonempty(manifest.API) || !nonempty(manifest.Concurrency) || !nonempty(manifest.ReviewedBy) {
		return errors.New("module manifest requires api, concurrency and reviewed_by")
	}
	reviewed, err := date(manifest.ReviewedAt)
	if err != nil {
		return err
	}
	if reviewed.After(r.now) {
		return errors.New("reviewed_at is in the future")
	}
	if manifest.Provenance != "PROVENANCE.md" {
		return errors.New("module provenance must be PROVENANCE.md")
	}
	data, err := r.read(filepath.ToSlash(filepath.Join(dir, manifest.Provenance)))
	if err != nil {
		return err
	}
	if !nonempty(string(data)) {
		return errors.New("module provenance must not be empty")
	}
	if len(manifest.Tests) == 0 || len(manifest.Evals) == 0 || len(m.Evals) == 0 {
		return errors.New("module requires tests and evals in manifest and metadata")
	}
	for _, test := range manifest.Tests {
		if filepath.IsAbs(test) || filepath.Clean(test) != test || strings.HasPrefix(test, "..") {
			return fmt.Errorf("unsafe module test path %q", test)
		}
		if !strings.HasSuffix(test, "_test.go") && !strings.HasSuffix(test, "_test.rb") && !strings.HasPrefix(filepath.Base(test), "test_") {
			return fmt.Errorf("unrecognized test file %q", test)
		}
		data, err := r.read(filepath.ToSlash(filepath.Join(dir, test)))
		if err != nil {
			return err
		}
		if !nonempty(string(data)) {
			return fmt.Errorf("empty test file %s", test)
		}
	}
	a := append([]string(nil), m.Evals...)
	b := append([]string(nil), manifest.Evals...)
	sort.Strings(a)
	sort.Strings(b)
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		return errors.New("module manifest evals must match metadata evals")
	}
	for _, path := range manifest.Evals {
		if err := r.validateEval(path); err != nil {
			return err
		}
	}
	code := false
	err = r.walk(dir, func(path string) error {
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".rb" {
			return nil
		}
		data, err := r.read(path)
		if err != nil {
			return err
		}
		base := filepath.Base(path)
		if !strings.Contains(base, "_test.") && !strings.HasPrefix(base, "test_") && nonempty(string(data)) {
			code = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !code {
		return errors.New("module must contain implementation code")
	}
	return nil
}
func (r *Repository) inputFingerprint() string {
	keys := make([]string, 0, len(r.inputs))
	for key := range r.inputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	manifest := []byte{}
	for _, key := range keys {
		// 本文全体を連結せず、ファイルごとの固定長ダイジェストを集約する。
		digest := sha256.Sum256(r.inputs[key])
		manifest = fmt.Appendf(manifest, "%d:%s:%x\n", len(key), key, digest)
	}
	digest := sha256.Sum256(manifest)
	return hex.EncodeToString(digest[:])
}

// Validate は Load での検証完了を確認する。Repository の内容は読み込み時点のもの。
func (r *Repository) Validate() error {
	if r == nil || r.fingerprint == "" {
		return errors.New("repository was not loaded")
	}
	return nil
}
func (r *Repository) result(doc Document, relevance int) Result {
	return Result{ID: doc.Metadata.ID, Path: doc.Path, Title: doc.Metadata.Title, Type: doc.Metadata.Kind, Technology: doc.Metadata.Technology, Version: doc.Metadata.Version, Relevance: relevance, Trust: doc.Metadata.Trust, RetrievedAt: doc.Metadata.RetrievedAt, ExpiresAt: doc.EffectiveExpiry, Stale: doc.Metadata.Status == "stale" || r.now.Format(dateLayout) >= doc.EffectiveExpiry}
}
func (r *Repository) Freshness() []Result {
	results := []Result{}
	for _, doc := range r.documents {
		result := r.result(doc, 0)
		if result.Stale {
			results = append(results, result)
		}
	}
	return results
}
func (r *Repository) Stats() Stats {
	s := Stats{Documents: len(r.documents), Sources: len(r.sources)}
	for _, doc := range r.documents {
		switch doc.Metadata.Kind {
		case "knowledge":
			s.Knowledge++
		case "pattern":
			s.Patterns++
		case "module":
			s.Modules++
		}
		if r.result(doc, 0).Stale {
			s.Stale++
		} else {
			s.Active++
		}
	}
	return s
}
