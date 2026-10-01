package research

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/shiroemons/ai-engineering-kit/internal/kb"
)

// GPTDomain is a coverage-ranked research assignment. The assistant chooses the
// concrete question and verifies primary sources; this helper calls no model.
type GPTDomain struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Technologies   []string `json:"technologies"`
	KnowledgeCount int      `json:"knowledge_count"`
	EvalPath       string   `json:"eval_path"`
}

// GPTPlanResult records the clean baseline and the bounded, ordered domains.
type GPTPlanResult struct {
	Base              string      `json:"base"`
	Topics            int         `json:"topics"`
	SelectionStrategy string      `json:"selection_strategy"`
	Domains           []GPTDomain `json:"domains"`
}

// GPTDocument identifies one validated document and its required search suite.
type GPTDocument struct {
	Path       string `json:"path"`
	ID         string `json:"id"`
	Domain     string `json:"domain"`
	Technology string `json:"technology"`
	EvalPath   string `json:"eval_path"`
}

// GPTValidationResult describes the final artifacts, including untracked files.
// Topics is the requested upper bound; Documents is the actual accepted set.
type GPTValidationResult struct {
	Base      string        `json:"base"`
	Topics    int           `json:"topics"`
	Documents []GPTDocument `json:"documents"`
	Files     []string      `json:"files"`
}

func gptTopics(topics int) error {
	if topics < 1 || topics > 3 {
		return errors.New("topics must be between 1 and 3")
	}
	return nil
}

// GPTPlan selects domains with the existing coverage-first planner, verifies
// the baseline knowledge and all search evals, and rebuilds ignored rag files.
// It requires a clean checkout and never invokes OpenCode or changes Git state.
func GPTPlan(ctx context.Context, root string, topics int) (*GPTPlanResult, error) {
	if err := gptTopics(topics); err != nil {
		return nil, err
	}
	root, err := gptRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	status, err := gptGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	if len(status) != 0 {
		return nil, errors.New("plan requires a clean working tree, including non-ignored untracked files")
	}
	base, err := gptGit(ctx, root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return nil, err
	}
	c, err := readConfig(root)
	if err != nil {
		return nil, err
	}
	if topics > len(c.Domains) {
		return nil, errors.New("topics exceeds configured research domains")
	}
	r, docs, err := indexed(root)
	if err != nil {
		return nil, err
	}
	if err := gptValidateEvals(ctx, root, r); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidBaselineEvals, err)
	}
	recent, err := gptGit(ctx, root, "log", "-"+strconv.Itoa(c.Selection.RecentCommits), "--format=", "--name-only", "-z", "--", "knowledge/")
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, doc := range docs {
		if doc.Metadata.Kind == "knowledge" {
			counts[documentDomain(doc, c.Domains)]++
		}
	}
	result := &GPTPlanResult{Base: strings.TrimSpace(string(base)), Topics: topics, SelectionStrategy: "coverage_first", Domains: make([]GPTDomain, 0, topics)}
	for _, d := range plan(c.Domains, docs, strings.Split(string(recent), "\x00"))[:topics] {
		result.Domains = append(result.Domains, GPTDomain{ID: d.ID, Name: d.Name, Technologies: slices.Clone(d.Technologies), KnowledgeCount: counts[d.ID], EvalPath: "evals/knowledge/" + d.ID + ".json"})
	}
	return result, nil
}

// ValidateGPT validates a bounded artifact-only batch against an ancestor
// commit. Both committed changes and working/staged/untracked changes are
// guarded; only the net final changes count toward the document cap. Git state
// is never modified. Existing source records are immutable and eval cases may
// only be appended, so provenance and earlier assertions cannot be weakened.
func ValidateGPT(ctx context.Context, root, base string, topics int) (*GPTValidationResult, error) {
	if err := gptTopics(topics); err != nil {
		return nil, err
	}
	root, err := gptRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	if err := gptBaseline(ctx, root, base); err != nil {
		return nil, err
	}
	files, err := gptChanges(ctx, root, base)
	if err != nil {
		return nil, err
	}
	knowledgeCount := 0
	for _, path := range files {
		if strings.HasPrefix(path, "knowledge/") {
			knowledgeCount++
		}
	}
	if knowledgeCount < 1 || knowledgeCount > topics {
		return nil, fmt.Errorf("research must change 1..%d knowledge documents; found %d", topics, knowledgeCount)
	}
	c, err := readConfig(root)
	if err != nil {
		return nil, err
	}
	domains := map[string]domain{}
	for _, d := range c.Domains {
		domains[d.ID] = d
	}
	r, docs, err := indexed(root)
	if err != nil {
		return nil, err
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	byPath := map[string]kb.Document{}
	for _, doc := range docs {
		byPath[doc.Path] = doc
	}
	result := &GPTValidationResult{Base: base, Topics: topics, Documents: make([]GPTDocument, 0, knowledgeCount), Files: files}
	usedDomains, sourceIDs := map[string]bool{}, map[string]bool{}
	for _, path := range files {
		if !strings.HasPrefix(path, "knowledge/") {
			continue
		}
		doc, found := byPath[path]
		if !found || doc.Metadata.Kind != "knowledge" {
			return nil, fmt.Errorf("changed knowledge file is not an indexed document: %s", path)
		}
		id, tags := "", 0
		for _, tag := range doc.Metadata.Tags {
			if value, ok := strings.CutPrefix(tag, "research-domain:"); ok {
				id = value
				tags++
			}
		}
		d, found := domains[id]
		if tags != 1 || !found || !slices.Contains(d.Technologies, doc.Metadata.Technology) {
			return nil, fmt.Errorf("knowledge requires exactly one configured research-domain tag and a matching technology: %s", path)
		}
		if strings.Split(path, "/")[1] != doc.Metadata.Technology {
			return nil, fmt.Errorf("knowledge directory must match metadata technology: %s", path)
		}
		if usedDomains[id] {
			return nil, fmt.Errorf("research must change exactly one knowledge document per domain: %s", id)
		}
		usedDomains[id] = true
		evalPath := "evals/knowledge/" + id + ".json"
		if !slices.Contains(doc.Metadata.Evals, evalPath) {
			return nil, fmt.Errorf("knowledge metadata must reference its domain eval %s: %s", evalPath, path)
		}
		suite, err := gptReadEval(root, evalPath)
		if err != nil {
			return nil, err
		}
		if !slices.ContainsFunc(suite.Cases, func(c evalCase) bool { return slices.Contains(c.ExpectedIDs, doc.Metadata.ID) }) {
			return nil, fmt.Errorf("domain eval %s must cover changed knowledge ID %s", evalPath, doc.Metadata.ID)
		}
		for _, ref := range doc.Metadata.Sources {
			sourceIDs[ref.ID] = true
		}
		result.Documents = append(result.Documents, GPTDocument{Path: path, ID: doc.Metadata.ID, Domain: id, Technology: doc.Metadata.Technology, EvalPath: evalPath})
	}
	for _, path := range files {
		switch {
		case strings.HasPrefix(path, "evals/knowledge/"):
			id := strings.TrimSuffix(filepath.Base(path), ".json")
			if !usedDomains[id] {
				return nil, fmt.Errorf("changed eval has no changed knowledge document in its domain: %s", path)
			}
			if err := gptPreserveEvals(ctx, root, base, path); err != nil {
				return nil, err
			}
		case strings.HasPrefix(path, "sources/catalog/"):
			if err := gptPreserveSource(ctx, root, base, path, sourceIDs); err != nil {
				return nil, err
			}
		}
	}
	if err := gptValidateEvals(ctx, root, r); err != nil {
		return nil, err
	}
	return result, nil
}
