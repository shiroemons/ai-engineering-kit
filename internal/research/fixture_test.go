package research

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyRepositoryFixture copies an unmodified seed, including its independent
// Git directory. Never share worktrees, hardlinks, or Git object alternates:
// cases intentionally modify files, modes, the index, and committed history.
func copyRepositoryFixture(t *testing.T, seed string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(root, os.DirFS(seed)); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCopiedRepositoryFixtureIsIsolated(t *testing.T) {
	config := filepath.Join(t.TempDir(), "gitconfig")
	writeFile(t, config, []byte("[maintenance]\n\tauto = true\n"), 0600)
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	seed, base := gptRepositoryFixture(t)
	first, second := copyRepositoryFixture(t, seed), copyRepositoryFixture(t, seed)

	paths := []string{
		"config/research.json", "knowledge/go/context.md", "sources/catalog/go-context-docs.json",
		".git/index", ".git/config", ".git/HEAD", ".git/refs/heads/main",
	}
	contents := make(map[string][]byte, len(paths))
	modes := make(map[string]os.FileMode, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(seed, path))
		if err != nil {
			t.Fatal(err)
		}
		contents[path] = data
		info, err := os.Stat(filepath.Join(seed, path))
		if err != nil {
			t.Fatal(err)
		}
		modes[path] = info.Mode()
		for _, root := range []string{first, second} {
			copyInfo, err := os.Stat(filepath.Join(root, path))
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(info, copyInfo) {
				t.Fatalf("copied fixture shares file %s with seed", path)
			}
		}
	}
	for _, root := range []string{seed, first, second} {
		top, err := git(t.Context(), root, "rev-parse", "--show-toplevel")
		if err != nil || strings.TrimSpace(string(top)) != root {
			t.Fatalf("fixture root = %q, want %q: %v", top, root, err)
		}
		common, err := git(t.Context(), root, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if err != nil || strings.TrimSpace(string(common)) != filepath.Join(root, ".git") {
			t.Fatalf("fixture shares Git directory: %q: %v", common, err)
		}
		maintenance, err := git(t.Context(), root, "config", "--local", "--bool", "--get", "maintenance.auto")
		if err != nil || strings.TrimSpace(string(maintenance)) != "false" {
			t.Fatalf("fixture may launch background maintenance: %q: %v", maintenance, err)
		}
		if _, err := os.Stat(filepath.Join(root, ".git/objects/info/alternates")); !os.IsNotExist(err) {
			t.Fatalf("fixture must not share Git object alternates: %v", err)
		}
	}

	writeFile(t, filepath.Join(first, "config/research.json"), []byte("{}\n"), 0644)
	if err := os.Chmod(filepath.Join(first, "knowledge/go/context.md"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(first, "sources/catalog/go-context-docs.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := git(t.Context(), first, "config", "--local", "fixture.isolation", "changed"); err != nil {
		t.Fatal(err)
	}
	gptTestCommit(t, first)
	if got := gptTestHEAD(t, first); got == base {
		t.Fatal("modified fixture did not advance its own history")
	}
	if err := os.RemoveAll(first); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{seed, second} {
		for _, path := range paths {
			data, err := os.ReadFile(filepath.Join(root, path))
			if err != nil || !bytes.Equal(data, contents[path]) {
				t.Fatalf("fixture mutation affected %s: %v", filepath.Join(root, path), err)
			}
			info, err := os.Stat(filepath.Join(root, path))
			if err != nil || info.Mode() != modes[path] {
				t.Fatalf("fixture mutation affected mode of %s: %v", filepath.Join(root, path), err)
			}
		}
		if got := gptTestHEAD(t, root); got != base {
			t.Fatalf("fixture mutation affected sibling history: %s, want %s", got, base)
		}
		if err := requireClean(t.Context(), root); err != nil {
			t.Fatalf("fixture mutation affected sibling working tree: %v", err)
		}
	}
}
