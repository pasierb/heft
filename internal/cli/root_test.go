package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func execute(t *testing.T, args ...string) (string, string, error) {
	return executeWithInput(t, "", args...)
}

func executeWithInput(t *testing.T, input string, args ...string) (string, string, error) {
	t.Helper()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := New("v1.2.3")
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(args)
	err := cmd.Execute()

	return stdout.String(), stderr.String(), err
}

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return dir
}

func installHerdr(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	herdr := filepath.Join(dir, "herdr")
	if err := os.WriteFile(herdr, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestInitChecksForHerdr(t *testing.T) {
	t.Run("installed", func(t *testing.T) {
		dir := gitRepo(t)
		t.Chdir(dir)
		installHerdr(t)

		stdout, _, err := execute(t, "init")
		if err != nil {
			t.Fatalf("execute init command: %v", err)
		}
		if want := "herdr is installed\nWorktrees directory [.worktrees]: \nBase branch [main]: \n"; stdout != want {
			t.Fatalf("init output = %q, want %q", stdout, want)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".heft.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if want := "worktrees_dir: .worktrees\nbase_branch: main\n"; string(data) != want {
			t.Fatalf("config = %q, want %q", data, want)
		}
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

func TestConfigure(t *testing.T) {
	dir := gitRepo(t)
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
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

func TestInitPreservesExistingConfig(t *testing.T) {
	dir := gitRepo(t)
	t.Chdir(dir)
	installHerdr(t)
	path := filepath.Join(dir, ".heft.yaml")
	original := []byte("leave this alone")
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
}

func TestRootShowsHelp(t *testing.T) {
	stdout, _, err := execute(t)
	if err != nil {
		t.Fatalf("execute root command: %v", err)
	}
	if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "heft [flags]") {
		t.Fatalf("expected help output, got %q", stdout)
	}
}

func TestVersionFlag(t *testing.T) {
	stdout, _, err := execute(t, "--version")
	if err != nil {
		t.Fatalf("execute version flag: %v", err)
	}
	if want := "heft v1.2.3\n"; stdout != want {
		t.Fatalf("version output = %q, want %q", stdout, want)
	}
}

func TestVersionCommand(t *testing.T) {
	stdout, _, err := execute(t, "version")
	if err != nil {
		t.Fatalf("execute version command: %v", err)
	}
	if want := "heft v1.2.3\n"; stdout != want {
		t.Fatalf("version output = %q, want %q", stdout, want)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, _, err := execute(t, "unknown")
	if err == nil {
		t.Fatal("expected an error for an unknown command")
	}
	if !strings.Contains(err.Error(), `unknown command "unknown"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
