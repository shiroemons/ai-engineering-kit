package research

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type recoveryWorker struct {
	Domain           string `json:"domain"`
	Model            string `json:"model"`
	Path             string `json:"path"`
	Topic            string `json:"topic,omitempty"`
	Evidence         string `json:"evidence,omitempty"`
	Phase            string `json:"phase"`
	Progress         int    `json:"progress"`
	OpenCodePID      int    `json:"opencode_pid,omitempty"`
	OpenCodePIDStart string `json:"opencode_pid_start,omitempty"`
}

type batchRecovery struct {
	Version   int              `json:"version"`
	RunID     string           `json:"run_id"`
	PID       int              `json:"pid"`
	PIDStart  string           `json:"pid_start"`
	Root      string           `json:"root"`
	Base      string           `json:"base"`
	Batch     string           `json:"batch"`
	UpdatedAt time.Time        `json:"updated_at"`
	Prepared  bool             `json:"prepared,omitempty"`
	Accepted  []string         `json:"accepted,omitempty"`
	Topics    []string         `json:"topics,omitempty"`
	Partial   bool             `json:"partial,omitempty"`
	Complete  bool             `json:"complete,omitempty"`
	Workers   []recoveryWorker `json:"workers"`
}

type recoveryJournal struct {
	mu   sync.Mutex
	path string
	data batchRecovery
}

type domainLeaseOwner struct {
	RunID    string `json:"run_id"`
	PID      int    `json:"pid"`
	PIDStart string `json:"pid_start"`
}

func acquireDomainLease(path, runID string) error {
	pidStart, err := processStartIdentity(os.Getpid())
	if err != nil {
		return fmt.Errorf("identify domain lease owner: %w", err)
	}
	temporary, err := writeDomainLeaseTemp(path, domainLeaseOwner{RunID: runID, PID: os.Getpid(), PIDStart: pidStart})
	if err != nil {
		return err
	}
	if err := os.Link(temporary, path); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	if err := os.Remove(temporary); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	return nil
}

func reclaimDomainLease(path, runID string) error {
	reclaimed, err := reclaimDomainLeaseIfOwned(path, runID)
	if err != nil {
		return err
	}
	if !reclaimed {
		return fmt.Errorf("domain lease %s belongs to another run", path)
	}
	return nil
}

func reclaimDomainLeaseIfOwned(path, runID string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, fmt.Errorf("inspect domain lease %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("domain lease %s is a symlink", path)
	}
	legacyDirectory := info.IsDir()
	ownerPath := path
	if legacyDirectory {
		ownerPath = filepath.Join(path, "owner.json")
	}
	data, err := os.ReadFile(ownerPath)
	if legacyDirectory && errors.Is(err, os.ErrNotExist) {
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return false, fmt.Errorf("inspect empty legacy domain lease %s: %w", path, readErr)
		}
		if len(entries) == 0 {
			if err := os.Remove(path); err != nil {
				return false, fmt.Errorf("remove empty legacy domain lease %s: %w", path, err)
			}
			if err := acquireDomainLease(path, runID); err != nil {
				return false, fmt.Errorf("reclaim empty legacy domain lease %s: %w", path, err)
			}
			return true, nil
		}
	}
	if err != nil {
		return false, fmt.Errorf("read domain lease %s: %w", path, err)
	}
	var owner domainLeaseOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return false, fmt.Errorf("decode domain lease %s: %w", path, err)
	}
	if owner.RunID != runID {
		return false, nil
	}
	if owner.PID <= 1 || processIdentityAlive(owner.PID, owner.PIDStart) {
		return false, fmt.Errorf("domain lease %s still has a live or unidentified owner", path)
	}
	pidStart, err := processStartIdentity(os.Getpid())
	if err != nil {
		return false, fmt.Errorf("identify resumed domain lease owner: %w", err)
	}
	nextOwner := domainLeaseOwner{RunID: runID, PID: os.Getpid(), PIDStart: pidStart}
	if legacyDirectory {
		if err := os.Remove(ownerPath); err != nil {
			return false, fmt.Errorf("remove legacy domain lease owner %s: %w", ownerPath, err)
		}
		if err := os.Remove(path); err != nil {
			return false, fmt.Errorf("remove legacy domain lease directory %s: %w", path, err)
		}
		if err := acquireDomainLease(path, runID); err != nil {
			return false, fmt.Errorf("reclaim legacy domain lease %s: %w", path, err)
		}
		return true, nil
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("domain lease %s is not a regular file", path)
	}
	if err := writeDomainLeaseOwner(path, nextOwner); err != nil {
		return false, err
	}
	return true, nil
}

func releaseDomainLease(path, runID string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("domain lease owner is missing: %s", path)
	}
	if err != nil {
		return err
	}
	var owner domainLeaseOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return err
	}
	if owner.RunID != runID {
		return fmt.Errorf("domain lease %s belongs to another run", path)
	}
	return os.Remove(path)
}

func writeDomainLeaseOwner(path string, owner domainLeaseOwner) error {
	temporary, err := writeDomainLeaseTemp(path, owner)
	if err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return errors.Join(err, os.Remove(temporary))
	}
	return nil
}

func writeDomainLeaseTemp(path string, owner domainLeaseOwner) (string, error) {
	data, err := json.Marshal(owner)
	if err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".owner-"+owner.RunID+"-*.tmp")
	if err != nil {
		return "", err
	}
	name := temporary.Name()
	if err := temporary.Chmod(0600); err != nil {
		return "", errors.Join(err, temporary.Close(), os.Remove(name))
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		return "", errors.Join(err, temporary.Close(), os.Remove(name))
	}
	if err := temporary.Sync(); err != nil {
		return "", errors.Join(err, temporary.Close(), os.Remove(name))
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func newRecoveryJournal(runDir, runID, root, base, batch string, workers []recoveryWorker) (*recoveryJournal, error) {
	pidStart, err := processStartIdentity(os.Getpid())
	if err != nil {
		return nil, fmt.Errorf("identify recovery coordinator process: %w", err)
	}
	journal := &recoveryJournal{
		path: filepath.Join(runDir, "batch.json"),
		data: batchRecovery{
			Version: 2, RunID: runID, PID: os.Getpid(), PIDStart: pidStart, Root: root, Base: base, Batch: batch,
			UpdatedAt: time.Now().UTC(), Workers: workers,
		},
	}
	if err := journal.writeLocked(); err != nil {
		return nil, err
	}
	return journal, nil
}

func loadRecoveryJournal(runDir, runID string) (*recoveryJournal, error) {
	path := filepath.Join(runDir, "batch.json")
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect recovery journal: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("recovery journal must be a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read recovery journal: %w", err)
	}
	var snapshot batchRecovery
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("decode recovery journal: %w", err)
	}
	if (snapshot.Version != 1 && snapshot.Version != 2) || snapshot.RunID != runID || snapshot.Root == "" || snapshot.Base == "" || snapshot.Batch == "" || len(snapshot.Workers) == 0 {
		return nil, errors.New("recovery journal is incomplete or belongs to another run")
	}
	if snapshot.Version >= 2 && snapshot.PID > 1 && snapshot.PIDStart == "" {
		return nil, errors.New("recovery journal has no coordinator process identity")
	}
	return &recoveryJournal{path: path, data: snapshot}, nil
}

func (j *recoveryJournal) snapshot() batchRecovery {
	j.mu.Lock()
	defer j.mu.Unlock()
	copy := j.data
	copy.Workers = append([]recoveryWorker(nil), j.data.Workers...)
	return copy
}

func (j *recoveryJournal) updateWorker(index int, update func(*recoveryWorker)) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if index < 0 || index >= len(j.data.Workers) {
		return errors.New("recovery journal worker index is invalid")
	}
	update(&j.data.Workers[index])
	j.data.PID = os.Getpid()
	j.data.UpdatedAt = time.Now().UTC()
	return j.writeLocked()
}

func (j *recoveryJournal) update(update func(*batchRecovery)) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	update(&j.data)
	j.data.PID = os.Getpid()
	j.data.UpdatedAt = time.Now().UTC()
	return j.writeLocked()
}

func (j *recoveryJournal) clearCoordinatorPID() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.data.PID = 0
	j.data.PIDStart = ""
	j.data.UpdatedAt = time.Now().UTC()
	return j.writeLocked()
}

func (j *recoveryJournal) writeLocked() error {
	data, err := json.MarshalIndent(j.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(j.path), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(j.path), ".batch-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	cleanup := func(cause error) error {
		return errors.Join(cause, temporary.Close(), os.Remove(name))
	}
	if err := temporary.Chmod(0600); err != nil {
		return cleanup(err)
	}
	if _, err := temporary.Write(append(data, '\n')); err != nil {
		return cleanup(err)
	}
	if err := temporary.Sync(); err != nil {
		return cleanup(err)
	}
	if err := temporary.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, j.path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

func validRecoveryID(id string) bool {
	if id == "" || filepath.Base(id) != id || strings.ContainsAny(id, `/\\`) {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}

func validateRecoveryBatchPath(storage, batch string) error {
	storage, err := filepath.EvalSymlinks(storage)
	if err != nil {
		return err
	}
	batch, err = filepath.Abs(batch)
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(batch)
	if err != nil {
		return fmt.Errorf("resolve saved batch worktree path: %w", err)
	}
	if resolved != batch || filepath.Dir(batch) != storage || !strings.HasPrefix(filepath.Base(batch), "batch-") {
		return errors.New("saved batch path is outside the research worktree directory")
	}
	info, err := os.Stat(batch)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("saved batch path is not a directory")
	}
	return nil
}

func ensureRecoveryWorkerWorktree(ctx context.Context, root, path, base string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if _, err := git(ctx, root, "worktree", "add", "--detach", path, base); err != nil {
			return fmt.Errorf("recreate missing worker worktree: %w", err)
		}
	} else if err != nil {
		return err
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("saved worker worktree path is not a real directory")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if resolved != path {
		return errors.New("saved worker worktree path resolves outside its recorded location")
	}
	top, err := git(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil || filepath.Clean(strings.TrimSpace(string(top))) != path {
		return errors.New("saved worker path is not the registered Git worktree for this run")
	}
	head, err := git(ctx, path, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(head)) != base {
		return errors.New("saved worker worktree no longer matches the run base commit")
	}
	return nil
}

func removeInterruptedIntegrationWorktrees(ctx context.Context, root, batch string) error {
	entries, err := os.ReadDir(batch)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "integration-") {
			continue
		}
		path := filepath.Join(batch, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("refusing to remove unexpected integration path: %s", path)
		}
		top, gitErr := git(ctx, path, "rev-parse", "--show-toplevel")
		if gitErr == nil && filepath.Clean(strings.TrimSpace(string(top))) == path {
			if _, err := git(ctx, root, "worktree", "remove", "--force", path); err != nil {
				return fmt.Errorf("remove interrupted integration worktree %s: %w", path, err)
			}
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove incomplete integration directory %s: %w", path, err)
		}
	}
	return nil
}

func verifyAcceptedApplied(ctx context.Context, root string, accepted map[string][]byte) error {
	expected := make([]string, 0, len(accepted))
	for path := range accepted {
		expected = append(expected, path)
	}
	slices.Sort(expected)
	staged, err := git(ctx, root, "diff", "--cached", "--name-only", "--no-renames", "--")
	if err != nil {
		return err
	}
	actual := strings.Fields(string(staged))
	slices.Sort(actual)
	if !slices.Equal(actual, expected) {
		return errors.New("staged paths do not match the saved accepted artifacts")
	}
	unstaged, err := git(ctx, root, "diff", "--name-only", "--no-renames", "--")
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(unstaged))) != 0 {
		return errors.New("integration worktree has unstaged changes")
	}
	untracked, err := git(ctx, root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(untracked))) != 0 {
		return errors.New("integration worktree has unexpected untracked files")
	}
	for path, want := range accepted {
		full := filepath.Join(root, path)
		resolved, err := filepath.EvalSymlinks(full)
		if err != nil || resolved != full {
			return fmt.Errorf("integrated artifact path is missing or uses a symlink: %s", path)
		}
		got, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		if !slices.Equal(got, want) {
			return fmt.Errorf("integrated artifact differs from the saved worker result: %s", path)
		}
	}
	return nil
}
