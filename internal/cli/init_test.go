package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInitChecksForHerdr(t *testing.T) {
	t.Run("installed", func(t *testing.T) {
		dir := gitRepo(t)
		t.Chdir(dir)
		installHerdr(t)

		stdout, _, err := execute(t, "init")
		if err != nil {
			t.Fatalf("execute init command: %v", err)
		}
		if want := "herdr is installed\nWorktrees directory [.worktrees]: \nBase branch [main]: \nWorkspace prefix [" + filepath.Base(dir) + "]: \n"; stdout != want {
			t.Fatalf("init output = %q, want %q", stdout, want)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".heft.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if want := "worktrees_dir: .worktrees\nbase_branch: main\nworkspace_prefix: \"" + filepath.Base(dir) + "\"\ntabs:\n    - name: shell\n"; string(data) != want {
			t.Fatalf("config = %q, want %q", data, want)
		}
		assertFileContents(t, filepath.Join(dir, ".gitignore"), "/.worktrees/\n")
	})

	t.Run("missing", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		t.Setenv("PATH", t.TempDir())

		_, _, err := execute(t, "init")
		if err == nil || err.Error() != "herdr is not installed or not in PATH" {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, ".heft.yaml")); !os.IsNotExist(err) {
			t.Fatalf("config should not exist: %v", err)
		}
	})
}
func TestInitPreservesExistingConfig(t *testing.T) {
	dir := gitRepo(t)
	t.Chdir(dir)
	installHerdr(t)
	path := filepath.Join(dir, ".heft.yaml")
	original := []byte("worktrees_dir: trees\nbase_branch: main\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := execute(t, "init")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "herdr is installed\n" {
		t.Fatalf("unexpected output %q", stdout)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, original) {
		t.Fatalf("config changed to %q", data)
	}
	assertFileContents(t, filepath.Join(dir, ".gitignore"), "/trees/\n")
}
