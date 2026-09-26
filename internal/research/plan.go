// Package research coordinates isolated OpenCode research and validates its artifacts.
package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/shiroemons/ai-engineering-kit/internal/kb"
)

type domain struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Technologies []string `json:"technologies"`
}

type config struct {
	Topics   int `json:"topics_per_run"`
	Parallel struct {
		PreferredModels []string `json:"preferred_models"`
		TimeoutMinutes  int      `json:"worker_timeout_minutes"`
		CooldownMinutes int      `json:"rate_limit_cooldown_minutes"`
	} `json:"parallel"`
	Domains   []domain `json:"domains"`
	Selection struct {
		RecentCommits int `json:"recent_commits"`
	} `json:"selection"`
}

type evalCase struct {
	Name        string   `json:"name"`
	Query       string   `json:"query"`
	ExpectedIDs []string `json:"expected_ids"`
}

type evaluation struct {
	Cases []evalCase `json:"cases"`
}

var domainID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var modelID = regexp.MustCompile(`^opencode/[a-zA-Z0-9._-]+$`)

func readConfig(root string) (config, error) {
	var c config
	data, err := os.ReadFile(filepath.Join(root, "config/research.json"))
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if c.Topics < 1 || c.Topics > 2 || len(c.Domains) < c.Topics ||
		c.Parallel.TimeoutMinutes < 1 || c.Parallel.TimeoutMinutes > 120 ||
		c.Parallel.CooldownMinutes < 1 || c.Parallel.CooldownMinutes > 1440 ||
		c.Selection.RecentCommits < 1 || c.Selection.RecentCommits > 1000 {
		return c, errors.New("invalid research parallelism, timeout, cooldown, or history limit")
	}
	seen := map[string]bool{}
	for _, d := range c.Domains {
		if !domainID.MatchString(d.ID) || seen[d.ID] || len(d.Technologies) == 0 {
			return c, fmt.Errorf("invalid or duplicate research domain: %q", d.ID)
		}
		seen[d.ID] = true
	}
	for _, m := range c.Parallel.PreferredModels {
		if !modelID.MatchString(m) {
			return c, fmt.Errorf("invalid preferred model: %q", m)
		}
	}
	return c, nil
}

func indexed(root string) (*kb.Repository, []kb.Document, error) {
	r, err := kb.Load(root, time.Now().UTC())
	if err != nil {
		return nil, nil, err
	}
	if err := r.Index(); err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "rag/documents/documents.json"))
	if err != nil {
		return nil, nil, err
	}
	var docs []kb.Document
	if err := json.Unmarshal(data, &docs); err != nil {
		return nil, nil, err
	}
	return r, docs, nil
}

func documentDomain(doc kb.Document, domains []domain) string {
	for _, tag := range doc.Metadata.Tags {
		if id, ok := strings.CutPrefix(tag, "research-domain:"); ok {
			return id
		}
	}
	for _, d := range domains {
		if slices.Contains(d.Technologies, doc.Metadata.Technology) {
			return d.ID
		}
	}
	return ""
}

func plan(domains []domain, docs []kb.Document, recent []string) []domain {
	counts, last := map[string]int{}, map[string]int{}
	for _, d := range domains {
		last[d.ID] = len(recent) + 1
	}
	for _, doc := range docs {
		if doc.Metadata.Kind != "knowledge" {
			continue
		}
		id := documentDomain(doc, domains)
		counts[id]++
		if i := slices.Index(recent, doc.Path); i >= 0 && i < last[id] {
			last[id] = i
		}
	}
	ordered := slices.Clone(domains)
	slices.SortStableFunc(ordered, func(a, b domain) int {
		if counts[a.ID] != counts[b.ID] {
			return counts[a.ID] - counts[b.ID]
		}
		return last[b.ID] - last[a.ID]
	})
	return ordered
}

// Every eval file is executed, including the domain-specific files workers write.
func validateEvals(ctx context.Context, root string, r *kb.Repository) error {
	paths, err := filepath.Glob(filepath.Join(root, "evals/knowledge/*.json"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("no knowledge evals")
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var suite evaluation
		if err := json.Unmarshal(data, &suite); err != nil {
			return err
		}
		if len(suite.Cases) == 0 {
			return fmt.Errorf("empty eval suite: %s", path)
		}
		for _, c := range suite.Cases {
			if c.Name == "" || len(c.ExpectedIDs) == 0 {
				return fmt.Errorf("incomplete eval: %s", path)
			}
			results, err := r.Search(ctx, c.Query, 20)
			if err != nil {
				return err
			}
			for _, id := range c.ExpectedIDs {
				if !slices.ContainsFunc(results, func(result kb.Result) bool { return result.ID == id }) {
					return fmt.Errorf("eval %s: expected %s not found", c.Name, id)
				}
			}
		}
	}
	return nil
}
