package research

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type worker struct {
	Domain             domain
	Model, Path, Topic string
	Progress           int
	Patch              []byte
	Files              map[string][]byte
	Err                error
}

// Run accepts only the model IDs verified by the outer runner's live pricing
// check. Workers never edit the integration checkout or publish Git changes.
func Run(ctx context.Context, root, state string, models []string, dry bool, out io.Writer) (resultErr error) {
	return RunWithProgress(ctx, root, state, models, dry, out, nil)
}

// RunWithProgress reports fixed research milestones through report. Percentages
// are estimates until validation and integration are confirmed by the runner.
func RunWithProgress(ctx context.Context, root, state string, models []string, dry bool, out io.Writer, report ProgressReporter) (resultErr error) {
	publish := func(progress Progress) error {
		if report == nil {
			return nil
		}
		progress.UpdatedAt = time.Now().UTC()
		return report(progress)
	}
	if err := publish(Progress{Percent: 1, Phase: "既存知識と調査履歴を確認中"}); err != nil {
		return err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	c, err := readConfig(root)
	if err != nil {
		return err
	}
	if state == "" {
		return errors.New("state-dir is required")
	}
	if err := os.MkdirAll(state, 0700); err != nil {
		return err
	}
	if data, err := os.ReadFile(filepath.Join(state, "cooldown-until")); err == nil {
		until, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid cooldown state: %w", err)
		}
		if time.Now().Unix() < until {
			return errRateLimit
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(models) == 0 {
		return errors.New("no verified free models")
	}
	seen := map[string]bool{}
	for _, model := range models {
		if !modelID.MatchString(model) || seen[model] {
			return errors.New("invalid or duplicate model")
		}
		seen[model] = true
	}
	if len(models) > c.Topics {
		models = models[:c.Topics]
	}
	base, err := git(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if err := requireClean(ctx, root); err != nil {
		return err
	}
	_, docs, err := indexed(root)
	if err != nil {
		return err
	}
	recent, err := git(ctx, root, "log", "-"+strconv.Itoa(c.Selection.RecentCommits), "--format=", "--name-only", "--", "knowledge/")
	if err != nil {
		return err
	}
	domains := plan(c.Domains, docs, strings.Fields(string(recent)))
	// Both scheduled and continuous batches share domain leases, while their
	// worktrees and run locks remain independent.
	leaseDir := filepath.Join(state, "domains")
	if err := os.MkdirAll(leaseDir, 0700); err != nil {
		return err
	}
	var assigned []domain
	for _, d := range domains {
		lease := filepath.Join(leaseDir, d.ID+".lock")
		if err := os.Mkdir(lease, 0700); errors.Is(err, os.ErrExist) {
			continue
		} else if err != nil {
			return err
		}
		defer func() {
			if err := os.Remove(lease); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("domain lease retained: %s: %w", lease, err))
			}
		}()
		assigned = append(assigned, d)
		if len(assigned) == len(models) {
			break
		}
	}
	if len(assigned) == 0 {
		return errors.New("all research domains are already assigned")
	}
	models = models[:len(assigned)]
	for _, d := range assigned {
		if err := publish(Progress{Percent: 3, Domain: d.ID, DomainName: d.Name, Topic: "テーマ候補と一次資料を確認中", Phase: "領域割り当て後、候補テーマの一次資料を確認中"}); err != nil {
			return err
		}
	}
	// Use the original repository's ignored workbench even from a worktree.
	common, err := git(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	storage := filepath.Join(filepath.Dir(strings.TrimSpace(string(common))), ".workbench/repositories/research")
	if err := os.MkdirAll(storage, 0700); err != nil {
		return err
	}
	batch, err := os.MkdirTemp(storage, "batch-")
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "batch worktrees: %s\n", batch); err != nil {
		return err
	}
	workers := make([]worker, len(models))
	for i, model := range models {
		w := &workers[i]
		w.Domain, w.Model, w.Path = assigned[i], model, filepath.Join(batch, fmt.Sprintf("worker-%d", i+1))
		if _, err := git(ctx, root, "worktree", "add", "--detach", w.Path, strings.TrimSpace(string(base))); err != nil {
			return err
		}
	}
	batchCtx, stopBatch := context.WithCancelCause(ctx)
	defer stopBatch(nil)
	monitorDone := make(chan struct{})
	defer close(monitorDone)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-monitorDone:
				return
			case <-batchCtx.Done():
				return
			case <-ticker.C:
				data, err := os.ReadFile(filepath.Join(state, "cooldown-until"))
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				if err != nil {
					stopBatch(err)
					return
				}
				until, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
				if err != nil {
					stopBatch(err)
					return
				}
				if time.Now().Unix() < until {
					stopBatch(errRateLimit)
					return
				}
			}
		}
	}()
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			w := &workers[i]
			workerCtx, cancel := context.WithTimeout(batchCtx, time.Duration(c.Parallel.TimeoutMinutes)*time.Minute)
			defer cancel()
			w.Progress = 3
			workerReport := func(progress Progress) error {
				if progress.Percent < w.Progress {
					return nil
				}
				w.Progress = progress.Percent
				return publish(progress)
			}
			w.Topic, w.Err = runOpenCode(workerCtx, w.Path, w.Model, prompt(w.Domain, dry), stopBatch, w.Domain, workerReport)
			if w.Err == nil {
				if dry {
					w.Err = requireClean(workerCtx, w.Path)
				} else {
					w.Err = workerReport(Progress{Percent: 92, Domain: w.Domain.ID, DomainName: w.Domain.Name, Topic: w.Topic, Phase: "成果物と検索 eval を検証中"})
					if w.Err == nil {
						w.Files, w.Patch, w.Err = artifacts(workerCtx, w.Path, w.Domain)
					}
				}
				if w.Err == nil {
					phase := "検索 eval と成果物の検証を通過"
					if dry {
						phase = "dry run のテーマ選定を確認"
					}
					w.Err = workerReport(Progress{Percent: 95, Domain: w.Domain.ID, DomainName: w.Domain.Name, Topic: w.Topic, Phase: phase})
				}
			}
		})
	}
	wg.Wait()
	if errors.Is(context.Cause(batchCtx), errRateLimit) {
		until := time.Now().Add(time.Duration(c.Parallel.CooldownMinutes) * time.Minute).Unix()
		file, err := os.CreateTemp(state, ".cooldown-*")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(file, until); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		if err := os.Rename(file.Name(), filepath.Join(state, "cooldown-until")); err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	accepted := map[string][]byte{}
	var topics []string
	partial := false
	for i := range workers {
		w := &workers[i]
		if w.Err == nil && !dry {
			w.Err = integrate(ctx, root, batch, base, accepted, w)
		}
		if w.Err != nil {
			partial = true
			if err := publish(Progress{Percent: w.Progress, Domain: w.Domain.ID, DomainName: w.Domain.Name, Topic: w.Topic, Phase: "検証に失敗、成果を保留", Error: w.Err.Error()}); err != nil {
				return errors.Join(w.Err, err)
			}
			if _, err := fmt.Fprintf(out, "worker failed: model=%s domain=%s reason=%v recovery=%s\n", w.Model, w.Domain.ID, w.Err, w.Path); err != nil {
				return err
			}
			continue
		}
		topics = append(topics, w.Topic)
		if _, err := fmt.Fprintf(out, "worker passed: model=%s domain=%s topic=%s\n", w.Model, w.Domain.ID, w.Topic); err != nil {
			return err
		}
		maps.Copy(accepted, w.Files)
		if err := publish(Progress{Percent: 96, Domain: w.Domain.ID, DomainName: w.Domain.Name, Topic: w.Topic, Phase: "候補成果の統合可否を確認"}); err != nil {
			return err
		}
	}
	if len(topics) == 0 {
		return errors.New("no worker produced valid research; worktrees preserved")
	}
	if !dry {
		if err := requireClean(ctx, root); err != nil {
			return err
		}
		now, err := git(ctx, root, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
		if !bytes.Equal(base, now) {
			return errors.New("integration HEAD changed during research")
		}
		// Each patch was checked against the already accepted changes. A final
		// combined patch applies atomically, without overwriting local edits.
		if err := applyAccepted(ctx, root, batch, base, accepted); err != nil {
			return err
		}
	}
	phase := "検証済み成果を統合"
	if dry {
		phase = "dry run の成果を確認"
	}
	if err := publish(Progress{Percent: 98, Domain: assigned[0].ID, DomainName: assigned[0].Name, Topic: strings.Join(topics, " / "), Phase: phase}); err != nil {
		return err
	}
	for i := range workers {
		w := &workers[i]
		if w.Err != nil {
			continue
		}
		if !dry {
			current, err := git(ctx, w.Path, "diff", "--binary", "HEAD", "--")
			if err != nil || !bytes.Equal(current, w.Patch) {
				return errors.New("worker changed after validation; worktree preserved")
			}
			untracked, err := git(ctx, w.Path, "ls-files", "--others", "--exclude-standard")
			if err != nil || len(untracked) != 0 {
				return errors.New("new worker files after validation; worktree preserved")
			}
		}
		// Only our own successful, verified, integrated worktrees are removed.
		args := []string{"worktree", "remove"}
		if !dry {
			args = append(args, "--force")
		}
		if _, err := git(ctx, root, append(args, w.Path)...); err != nil {
			return err
		}
	}
	if partial {
		if _, err := fmt.Fprintln(out, "BATCH: partial"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(out, "BATCH: complete"); err != nil {
			return err
		}
		if err := os.Remove(batch); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(out, "TOPIC:", strings.Join(topics, " / "))
	if err != nil {
		return err
	}
	phase = "調査成果を統合"
	if dry {
		phase = "dry run のテーマ選定を完了"
	}
	return publish(Progress{Percent: 100, Domain: assigned[0].ID, DomainName: assigned[0].Name, Topic: strings.Join(topics, " / "), Phase: phase})
}

func requireClean(ctx context.Context, root string) error {
	data, err := git(ctx, root, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(data) != 0 {
		return errors.New("working tree is dirty")
	}
	return nil
}

func prompt(d domain, dry bool) string {
	mode := "Research and update exactly one knowledge document."
	if dry {
		mode = "DRY RUN: select a topic only. Do not edit files. Do not perform research or write artifacts."
	}
	return fmt.Sprintf(`%s
Assigned domain: %s (%s). Stay within these technologies: %s. Other workers cover other domains; do not change domain. Use config/research.json for sources and selection within this domain. Write evals ONLY to evals/knowledge/%s.json, preserving existing cases. Follow knowledge-researcher validation and source instructions.

Progress protocol:
- While choosing a topic, do not emit a topic or progress marker. In a research run, first inspect coverage and verify the selected topic against primary sources. Only after that verification, emit TOPIC_SELECTED: <technology and topic> and PROGRESS: sources-verified on separate lines. This is an interim update; continue with artifact writing and validation.
- Emit each remaining milestone only after it is complete: PROGRESS: knowledge-written after the knowledge document is complete; PROGRESS: eval-written after writing search evals; PROGRESS: eval-search-verified after every new query returns the expected ID; PROGRESS: ready-to-validate before final validation.
- Emit TOPIC: <technology and topic> exactly once, only after the required sources are verified, the knowledge document and assigned eval are written, every new eval query returns the expected ID, and final validation passes. If a required step fails, report its concrete reason and do not emit TOPIC.
- In a DRY RUN, after selecting a topic, emit TOPIC_SELECTED: <technology and topic> and PROGRESS: topic-selected, then emit TOPIC: <technology and topic> and stop without research or edits.
`, mode, d.ID, d.Name, strings.Join(d.Technologies, ", "), d.ID)
}

func artifacts(ctx context.Context, root string, d domain) (map[string][]byte, []byte, error) {
	status, err := git(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, nil, err
	}
	files := map[string][]byte{}
	knowledge := ""
	for entry := range bytes.SplitSeq(status, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		if len(entry) < 4 || strings.ContainsAny(string(entry[:2]), "DRCUT!") {
			return nil, nil, errors.New("deletion, rename, conflict, or type change forbidden")
		}
		path := string(entry[3:])
		if filepath.Clean(path) != path || filepath.IsAbs(path) || strings.Contains(path, "\\") {
			return nil, nil, errors.New("invalid artifact path")
		}
		if strings.HasPrefix(path, "knowledge/") {
			if knowledge != "" {
				return nil, nil, errors.New("worker must change exactly one knowledge document")
			}
			knowledge = path
		} else if !strings.HasPrefix(path, "sources/catalog/") && path != "evals/knowledge/"+d.ID+".json" {
			return nil, nil, fmt.Errorf("change outside assigned artifact paths: %s", path)
		}
		full := filepath.Join(root, path)
		resolved, err := filepath.EvalSymlinks(full)
		if err != nil || resolved != full {
			return nil, nil, fmt.Errorf("artifact must not use symlinks: %s", path)
		}
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
			return nil, nil, fmt.Errorf("artifact is not a regular file below 2 MiB: %s", path)
		}
		files[path], err = os.ReadFile(full)
		if err != nil {
			return nil, nil, err
		}
	}
	if knowledge == "" || files["evals/knowledge/"+d.ID+".json"] == nil {
		return nil, nil, errors.New("knowledge or assigned eval missing")
	}
	r, docs, err := indexed(root)
	if err != nil {
		return nil, nil, err
	}
	found := false
	knowledgeID := ""
	for _, doc := range docs {
		if doc.Path != knowledge {
			continue
		}
		found = slices.Contains(doc.Metadata.Tags, "research-domain:"+d.ID) && slices.Contains(d.Technologies, doc.Metadata.Technology)
		knowledgeID = doc.Metadata.ID
		for _, tag := range doc.Metadata.Tags {
			if strings.HasPrefix(tag, "research-domain:") && tag != "research-domain:"+d.ID {
				found = false
				break
			}
		}
	}
	if !found {
		return nil, nil, errors.New("knowledge must match assigned domain and technology")
	}
	var suite evaluation
	if err := json.Unmarshal(files["evals/knowledge/"+d.ID+".json"], &suite); err != nil {
		return nil, nil, err
	}
	if !slices.ContainsFunc(suite.Cases, func(c evalCase) bool { return slices.Contains(c.ExpectedIDs, knowledgeID) }) {
		return nil, nil, errors.New("assigned eval must cover the changed knowledge ID")
	}
	if err := validateEvals(ctx, root, r); err != nil {
		return nil, nil, err
	}
	if _, err := git(ctx, root, "add", "--", "knowledge/", "sources/catalog/", "evals/knowledge/"); err != nil {
		return nil, nil, err
	}
	patch, err := git(ctx, root, "diff", "--binary", "HEAD", "--")
	return files, patch, err
}

func candidate(ctx context.Context, root, batch string, base []byte, files map[string][]byte) (string, error) {
	path, err := os.MkdirTemp(batch, "integration-")
	if err != nil {
		return "", err
	}
	if _, err := git(ctx, root, "worktree", "add", "--detach", path, strings.TrimSpace(string(base))); err != nil {
		return "", err
	}
	for name, data := range files {
		full := filepath.Join(path, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return path, err
		}
		if err := os.WriteFile(full, data, 0644); err != nil {
			return path, err
		}
	}
	return path, nil
}

func integrate(ctx context.Context, root, batch string, base []byte, accepted map[string][]byte, w *worker) (resultErr error) {
	combined := make(map[string][]byte, len(accepted)+len(w.Files))
	maps.Copy(combined, accepted)
	for name, data := range w.Files {
		if previous, ok := combined[name]; ok && !bytes.Equal(previous, data) {
			return fmt.Errorf("conflicting artifact: %s", name)
		}
		combined[name] = data
	}
	path, err := candidate(ctx, root, batch, base, combined)
	if path != "" {
		defer func() {
			_, err := git(context.WithoutCancel(ctx), root, "worktree", "remove", "--force", path)
			resultErr = errors.Join(resultErr, err)
		}()
	}
	if err != nil {
		return err
	}
	r, _, err := indexed(path)
	if err != nil {
		return err
	}
	return validateEvals(ctx, path, r)
}

func applyAccepted(ctx context.Context, root, batch string, base []byte, accepted map[string][]byte) (resultErr error) {
	path, err := candidate(ctx, root, batch, base, accepted)
	if path != "" {
		defer func() {
			_, err := git(context.WithoutCancel(ctx), root, "worktree", "remove", "--force", path)
			resultErr = errors.Join(resultErr, err)
		}()
	}
	if err != nil {
		return err
	}
	if _, err := git(ctx, path, "add", "--", "knowledge/", "sources/catalog/", "evals/knowledge/"); err != nil {
		return err
	}
	patch, err := git(ctx, path, "diff", "--cached", "--binary")
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "git", "apply", "--index", "--whitespace=error", "-")
	cmd.Dir, cmd.Stdin = root, bytes.NewReader(patch)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apply validated research: %w: %s", err, output)
	}
	return nil
}
