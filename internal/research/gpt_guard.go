package research

import (
	"bytes"
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/shiroemons/ai-engineering-kit/internal/kb"
)

const gptMaxArtifactBytes = 2 << 20

var gptCommitSHA = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// Disable optional locks (including status index refresh), external diffs,
// textconv, fsmonitor hooks, and replace objects for these read-only checks.
func gptGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	prefix := []string{"--no-optional-locks", "--no-replace-objects", "--literal-pathspecs", "-c", "core.fsmonitor=false"}
	if len(args) > 0 && args[0] == "diff" {
		args = append([]string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--ignore-submodules=none"}, args[1:]...)
	}
	cmd := exec.CommandContext(ctx, "git", append(prefix, args...)...)
	cmd.Dir = root
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(diagnostic.String()))
	}
	return data, nil
}

func gptRoot(ctx context.Context, root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	top, err := gptGit(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(top)) != root {
		return "", errors.New("root must be the Git repository top-level directory")
	}
	return root, nil
}

func gptBaseline(ctx context.Context, root, base string) error {
	if !gptCommitSHA.MatchString(base) {
		return errors.New("base must be a full 40-character commit SHA")
	}
	kind, err := gptGit(ctx, root, "cat-file", "-t", base)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(kind)) != "commit" {
		return errors.New("base must identify a commit")
	}
	if _, err := gptGit(ctx, root, "merge-base", "--is-ancestor", base, "HEAD"); err != nil {
		return fmt.Errorf("base must be an ancestor of HEAD: %w", err)
	}
	return nil
}

func gptArtifactPath(path string) error {
	if filepath.Clean(path) != path || filepath.IsAbs(path) || strings.ContainsAny(path, "\\\r\n\t") {
		return fmt.Errorf("invalid artifact path: %q", path)
	}
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[2] == "" {
		return fmt.Errorf("change outside allowed artifact paths: %s", path)
	}
	valid := false
	switch parts[0] {
	case "knowledge":
		valid = domainID.MatchString(parts[1]) && strings.HasSuffix(parts[2], ".md") && parts[2] != ".md"
	case "sources":
		valid = parts[1] == "catalog" && strings.HasSuffix(parts[2], ".json") && parts[2] != ".json"
	case "evals":
		valid = parts[1] == "knowledge" && strings.HasSuffix(parts[2], ".json") && domainID.MatchString(strings.TrimSuffix(parts[2], ".json"))
	}
	if !valid {
		return fmt.Errorf("change outside allowed artifact paths: %s", path)
	}
	return nil
}

func gptRegularFile(root, path string) ([]byte, error) {
	full := root
	for part := range strings.SplitSeq(path, "/") {
		full = filepath.Join(full, part)
		info, err := os.Lstat(full)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("artifact must not use symlinks: %s", path)
		}
	}
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0111 != 0 || info.Size() > gptMaxArtifactBytes {
		return nil, fmt.Errorf("artifact must be a non-executable regular file no larger than 2 MiB: %s", path)
	}
	data, err := os.ReadFile(full)
	if len(data) > gptMaxArtifactBytes {
		return nil, fmt.Errorf("artifact exceeds 2 MiB: %s", path)
	}
	return data, err
}

func gptDiffPaths(data []byte) ([]string, error) {
	fields := bytes.Split(data, []byte{0})
	var paths []string
	for i := 0; i < len(fields)-1; i += 2 {
		if i+1 >= len(fields)-1 || (string(fields[i]) != "A" && string(fields[i]) != "M") {
			return nil, errors.New("deletion, rename, copy, conflict, or type change forbidden")
		}
		path := string(fields[i+1])
		if err := gptArtifactPath(path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	if len(data) > 0 && data[len(data)-1] != 0 {
		return nil, errors.New("invalid Git diff output")
	}
	return paths, nil
}

func gptChanges(ctx context.Context, root, base string) ([]string, error) {
	// Inspect HEAD separately: a working-tree reversal must not conceal a
	// committed source-code/config change or a committed deletion/rename.
	committed, err := gptGit(ctx, root, "diff", "--name-status", "-z", "--find-renames", base, "HEAD", "--")
	if err != nil {
		return nil, err
	}
	committedPaths, err := gptDiffPaths(committed)
	if err != nil {
		return nil, err
	}
	status, err := gptGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return nil, err
	}
	untracked, statusPaths := []string{}, []string{}
	for entry := range bytes.SplitSeq(status, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		if len(entry) < 4 || entry[2] != ' ' || strings.ContainsAny(string(entry[:2]), "DRCUT!") {
			return nil, errors.New("deletion, rename, copy, conflict, or type change forbidden")
		}
		path := string(entry[3:])
		statusPaths = append(statusPaths, path)
		if err := gptArtifactPath(path); err != nil {
			return nil, err
		}
		if _, err := gptRegularFile(root, path); err != nil {
			return nil, err
		}
		if entry[0] != ' ' && entry[1] != ' ' && string(entry[:2]) != "??" {
			return nil, fmt.Errorf("mixed staged and unstaged edits are forbidden; deliberately stage or unstage before retrying: %s", path)
		}
		if string(entry[:2]) == "??" {
			untracked = append(untracked, path)
		}
	}
	final, err := gptGit(ctx, root, "diff", "--name-status", "-z", "--find-renames", base, "--")
	if err != nil {
		return nil, err
	}
	paths, err := gptDiffPaths(final)
	if err != nil {
		return nil, err
	}
	paths = append(paths, untracked...)
	slices.Sort(paths)
	paths = slices.Compact(paths)
	for _, path := range append(slices.Clone(committedPaths), statusPaths...) {
		if !slices.Contains(paths, path) {
			return nil, fmt.Errorf("committed or staged change is masked by the final working tree; reconcile the artifact before retrying: %s", path)
		}
	}
	allPaths := append(slices.Clone(paths), committedPaths...)
	allPaths = append(allPaths, statusPaths...)
	slices.Sort(allPaths)
	allPaths = slices.Compact(allPaths)
	if err := gptGitModes(ctx, root, base, allPaths); err != nil {
		return nil, err
	}
	for _, path := range paths {
		if _, err := gptRegularFile(root, path); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// Check baseline, committed, and staged modes independently. A non-executable
// working copy must not conceal executable or symlink objects in Git.
func gptGitModes(ctx context.Context, root, base string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	for _, args := range [][]string{{"ls-tree", "-z", base, "--"}, {"ls-tree", "-z", "HEAD", "--"}, {"ls-files", "--stage", "-z", "--"}} {
		data, err := gptGit(ctx, root, append(args, paths...)...)
		if err != nil {
			return err
		}
		for entry := range bytes.SplitSeq(data, []byte{0}) {
			if len(entry) == 0 {
				continue
			}
			metadata, path, ok := strings.Cut(string(entry), "\t")
			fields := strings.Fields(metadata)
			if !ok || len(fields) != 3 || fields[0] != "100644" {
				return fmt.Errorf("git artifact must be a non-executable regular file: %s", path)
			}
		}
	}
	return nil
}

func gptReadEval(root, path string) (evaluation, error) {
	var suite evaluation
	data, err := gptRegularFile(root, path)
	if err == nil {
		err = jsonv2.Unmarshal(data, &suite, jsonv2.RejectUnknownMembers(true))
	}
	if err != nil {
		return suite, fmt.Errorf("invalid knowledge eval %s: %w", path, err)
	}
	return suite, nil
}

func gptValidateEvals(ctx context.Context, root string, r *kb.Repository) error {
	paths, err := filepath.Glob(filepath.Join(root, "evals/knowledge/*.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		if _, err := gptReadEval(root, "evals/knowledge/"+filepath.Base(path)); err != nil {
			return err
		}
	}
	return validateEvals(ctx, root, r)
}

// ls-tree distinguishes an absent baseline file from an object-read failure;
// failures must never be interpreted as a newly added, unrestricted artifact.
func gptBaselineFile(ctx context.Context, root, base, path string) ([]byte, bool, error) {
	entry, err := gptGit(ctx, root, "ls-tree", "-z", base, "--", path)
	if err != nil {
		return nil, false, err
	}
	if len(entry) == 0 {
		return nil, false, nil
	}
	metadata, name, ok := strings.Cut(strings.TrimSuffix(string(entry), "\x00"), "\t")
	fields := strings.Fields(metadata)
	if !ok || name != path || len(fields) != 3 || fields[0] != "100644" || fields[1] != "blob" {
		return nil, false, fmt.Errorf("baseline artifact is not a regular file: %s", path)
	}
	data, err := gptGit(ctx, root, "cat-file", "blob", fields[2])
	if len(data) > gptMaxArtifactBytes {
		return nil, false, fmt.Errorf("baseline artifact exceeds 2 MiB: %s", path)
	}
	return data, true, err
}

func gptPreserveEvals(ctx context.Context, root, base, path string) error {
	before, found, err := gptBaselineFile(ctx, root, base, path)
	if err != nil || !found {
		return err
	}
	var previous evaluation
	if err := jsonv2.Unmarshal(before, &previous, jsonv2.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("invalid baseline eval %s: %w", path, err)
	}
	current, err := gptReadEval(root, path)
	if err != nil {
		return err
	}
	remaining := slices.Clone(current.Cases)
	for _, oldCase := range previous.Cases {
		i := slices.IndexFunc(remaining, func(candidate evalCase) bool { return reflect.DeepEqual(oldCase, candidate) })
		if i < 0 {
			return fmt.Errorf("existing eval cases must be preserved unchanged: %s (%s)", path, oldCase.Name)
		}
		remaining = slices.Delete(remaining, i, i+1)
	}
	return nil
}

func gptPreserveSource(ctx context.Context, root, base, path string, referenced map[string]bool) error {
	data, err := gptRegularFile(root, path)
	if err != nil {
		return err
	}
	var current kb.Source
	if err := jsonv2.Unmarshal(data, &current, jsonv2.RejectUnknownMembers(true)); err != nil {
		return err
	}
	if !referenced[current.ID] {
		return fmt.Errorf("changed source must be referenced by a changed knowledge document: %s", path)
	}
	before, found, err := gptBaselineFile(ctx, root, base, path)
	if err != nil || !found {
		return err
	}
	var previous kb.Source
	if err := jsonv2.Unmarshal(before, &previous, jsonv2.RejectUnknownMembers(true)); err != nil {
		return fmt.Errorf("invalid baseline source %s: %w", path, err)
	}
	if current != previous {
		return fmt.Errorf("existing source records must be preserved unchanged; add a new record and ID: %s", path)
	}
	return nil
}
