package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigure(t *testing.T) {
	dir := gitRepo(t)
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("/vendor/"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	stdout, _, err := executeWithInput(t, "trees\ndevelop\n", "configure")
	if err != nil {
		t.Fatalf("execute configure command: %v", err)
	}
	if want := "Worktrees directory [.worktrees]: Base branch [main]: "; stdout != want {
		t.Fatalf("configure output = %q, want %q", stdout, want)
	}
	path := filepath.Join(dir, ".heft.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "worktrees_dir: trees\nbase_branch: develop\n"; string(data) != want {
		t.Fatalf("config = %q, want %q", data, want)
	}
	assertFileContents(t, filepath.Join(dir, ".gitignore"), "/vendor/\n/trees/\n")

	if _, _, err := executeWithInput(t, "new trees\n\n", "configure"); err != nil {
		t.Fatalf("reconfigure: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "worktrees_dir: \"new trees\"\nbase_branch: develop\n"; string(data) != want {
		t.Fatalf("reconfigured config = %q, want %q", data, want)
	}
	assertFileContents(t, filepath.Join(dir, ".gitignore"), "/vendor/\n/trees/\n/new trees/\n")
	if _, _, err := executeWithInput(t, "\n\n", "configure"); err != nil {
		t.Fatalf("configure without changes: %v", err)
	}
	assertFileContents(t, filepath.Join(dir, ".gitignore"), "/vendor/\n/trees/\n/new trees/\n")
}

func TestConfigureRejectsInvalidConfig(t *testing.T) {
	dir := gitRepo(t)
	t.Chdir(dir)
	path := filepath.Join(dir, ".heft.yaml")
	original := []byte("worktrees_dir: trees\nunknown: value\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := execute(t, "configure"); err == nil {
		t.Fatal("expected invalid config error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, original) {
		t.Fatalf("config changed to %q", data)
	}
}

func TestConfigureOutsideGitRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, _, err := execute(t, "configure"); err == nil || !strings.Contains(err.Error(), "find project root") {
		t.Fatalf("unexpected error: %v", err)
	}
}
