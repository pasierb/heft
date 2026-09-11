package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCopyFile(t *testing.T, root, path, contents string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCopyWorktreeFiles(t *testing.T) {
	root, destination := t.TempDir(), t.TempDir()
	var warnings bytes.Buffer
	copyWorktreeFiles(root, destination, &warnings)
	if warnings.Len() != 0 {
		t.Fatal(warnings.String())
	}
	writeCopyFile(t, root, ".heftcopy", " # setup\n\n .env \r\n.local/\n.local/run\nspace name\n")
	writeCopyFile(t, root, ".env", "secret")
	writeCopyFile(t, root, ".local/.hidden", "hidden")
	writeCopyFile(t, root, ".local/keep", "source")
	writeCopyFile(t, root, ".local/run", "executable")
	writeCopyFile(t, root, "space name", "spaces")
	writeCopyFile(t, destination, ".local/keep", "destination")
	if err := os.Chmod(filepath.Join(root, ".local/run"), 0o751); err != nil {
		t.Fatal(err)
	}
	copyWorktreeFiles(root, destination, &warnings)
	if warnings.Len() != 0 {
		t.Fatal(warnings.String())
	}
	for path, want := range map[string]string{".env": "secret", ".local/.hidden": "hidden", ".local/keep": "destination", ".local/run": "executable", "space name": "spaces"} {
		assertFileContents(t, filepath.Join(destination, path), want)
	}
	for path, want := range map[string]os.FileMode{".env": 0o600, ".local/run": 0o751} {
		info, err := os.Stat(filepath.Join(destination, path))
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("permissions for %s: %v, %v", path, info, err)
		}
	}
}

func TestCopyWorktreeFilesWarnsAndContinues(t *testing.T) {
	for _, entry := range []string{"missing", "../outside", "/absolute", ".git/config", ".", "trees", "source-link/file", "dest-link/file", "nested", "blocked"} {
		t.Run(entry, func(t *testing.T) {
			root := t.TempDir()
			destination := filepath.Join(root, "trees", "feature")
			outside := t.TempDir()
			writeCopyFile(t, root, ".heftcopy", entry+"\n.env\n")
			writeCopyFile(t, root, ".env", "secret")
			writeCopyFile(t, root, "dest-link/file", "must not escape")
			writeCopyFile(t, root, "blocked/child", "blocked")
			writeCopyFile(t, root, "nested/.git/config", "metadata")
			writeCopyFile(t, root, "nested/good", "good")
			writeCopyFile(t, destination, "blocked", "keep")
			writeCopyFile(t, outside, "file", "outside")
			for link, target := range map[string]string{
				filepath.Join(root, "source-link"):      outside,
				filepath.Join(root, "nested/link"):      outside,
				filepath.Join(destination, "dest-link"): outside,
			} {
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
			}
			var warnings bytes.Buffer
			copyWorktreeFiles(root, destination, &warnings)
			if !strings.Contains(warnings.String(), "Warning: .heftcopy") {
				t.Fatalf("expected warning for %q, got %q", entry, warnings.String())
			}
			assertFileContents(t, filepath.Join(destination, ".env"), "secret")
			assertFileContents(t, filepath.Join(outside, "file"), "outside")
			assertFileContents(t, filepath.Join(destination, "blocked"), "keep")
			for _, path := range []string{"source-link", "nested/.git", "nested/link", "trees", ".git"} {
				if _, err := os.Lstat(filepath.Join(destination, path)); !os.IsNotExist(err) {
					t.Fatalf("unexpected copied path %q: %v", path, err)
				}
			}
			if entry == "nested" {
				assertFileContents(t, filepath.Join(destination, "nested/good"), "good")
			}
		})
	}
}

func TestWorkCopiesBeforeWorkspaceAndSkipsReuse(t *testing.T) {
	for _, tabs := range []bool{false, true} {
		t.Run(map[bool]string{false: "simple", true: "tabs"}[tabs], func(t *testing.T) {
			repo := workRepo(t)
			cfg := "worktrees_dir: trees\nbase_branch: main\n"
			if tabs {
				cfg += "tabs:\n  - name: shell\n"
			}
			writeCopyFile(t, repo, ".heft.yaml", cfg)
			writeCopyFile(t, repo, ".heftcopy", "missing\n.env\n")
			writeCopyFile(t, repo, ".env", "primary")
			linked := filepath.Join(t.TempDir(), "linked")
			gitRun(t, repo, "worktree", "add", "-qb", "linked", linked)
			writeCopyFile(t, linked, ".env", "linked")
			t.Chdir(linked)
			bin := t.TempDir()
			writeCopyFile(t, bin, "herdr", `#!/bin/sh
if [ "$1 $2" = "workspace create" ]; then
  [ "$(cat "$4/.env")" = primary ] || exit 1
  printf '%s\n' '{"result":{"workspace":{"workspace_id":"w1"},"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}'
fi
`)
			if err := os.Chmod(filepath.Join(bin, "herdr"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			_, warnings, err := execute(t, "work", "feature")
			if err != nil || !strings.Contains(warnings, "missing") {
				t.Fatalf("work: %v, warnings: %s", err, warnings)
			}
			writeCopyFile(t, repo, ".heftcopy", ".env\nlater\n")
			writeCopyFile(t, repo, ".env", "changed")
			writeCopyFile(t, repo, "later", "must not copy")
			if _, _, err := execute(t, "work", "feature"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(repo, "trees/feature/later")); !os.IsNotExist(err) {
				t.Fatalf("reused worktree received new file: %v", err)
			}
		})
	}
}
