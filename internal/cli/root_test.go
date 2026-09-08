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

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func installHerdr(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	herdr := filepath.Join(dir, "herdr")
	contents := "#!/bin/sh\n[ -z \"$HERDR_TEST_LOG\" ] || printf '%s\\n' \"$@\" >> \"$HERDR_TEST_LOG\"\nexit \"${HERDR_TEST_EXIT:-0}\"\n"
	if err := os.WriteFile(herdr, []byte(contents), 0o755); err != nil {
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

func TestWorkCreatesBranchFromFetchedBase(t *testing.T) {
	installHerdr(t)
	herdrLog := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", herdrLog)

	remote := filepath.Join(t.TempDir(), "origin.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	repo := gitRepo(t)
	gitRun(t, repo, "config", "user.email", "test@example.com")
	gitRun(t, repo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "file"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "file")
	gitRun(t, repo, "commit", "-qm", "first")
	gitRun(t, repo, "branch", "-M", "main")
	gitRun(t, repo, "remote", "add", "origin", remote)
	gitRun(t, repo, "push", "-qu", "origin", "main")

	if err := os.WriteFile(filepath.Join(repo, "file"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "commit", "-qam", "second")
	wantCommit := gitRun(t, repo, "rev-parse", "HEAD")
	gitRun(t, repo, "push", "-q", "origin", "main")
	gitRun(t, repo, "reset", "-q", "--hard", "HEAD~1")

	if err := os.WriteFile(filepath.Join(repo, ".heft.yaml"), []byte("worktrees_dir: trees\nbase_branch: main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(repo, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	if _, _, err := execute(t, "work", "feature/abc"); err != nil {
		t.Fatalf("execute work command: %v", err)
	}
	worktree := filepath.Join(repo, "trees", "feature_abc")
	if got := gitRun(t, worktree, "branch", "--show-current"); got != "feature/abc" {
		t.Fatalf("branch = %q, want feature/abc", got)
	}
	if got := gitRun(t, worktree, "rev-parse", "HEAD"); got != wantCommit {
		t.Fatalf("commit = %q, want %q", got, wantCommit)
	}

	if _, _, err := execute(t, "work", "simple"); err != nil {
		t.Fatalf("create simple worktree: %v", err)
	}
	if got := gitRun(t, filepath.Join(repo, "trees", "simple"), "branch", "--show-current"); got != "simple" {
		t.Fatalf("branch = %q, want simple", got)
	}
	t.Setenv("HERDR_TEST_EXIT", "1")
	if _, _, err := execute(t, "work", "failed"); err == nil || !strings.Contains(err.Error(), "create herdr workspace") {
		t.Fatalf("unexpected herdr error: %v", err)
	}
	if got := gitRun(t, filepath.Join(repo, "trees", "failed"), "branch", "--show-current"); got != "failed" {
		t.Fatalf("branch = %q, want failed", got)
	}
	if _, _, err := execute(t, "work", "feature_abc"); err == nil {
		t.Fatal("expected normalized directory collision")
	}

	data, err := os.ReadFile(herdrLog)
	if err != nil {
		t.Fatal(err)
	}
	wantHerdrCalls := strings.Join([]string{
		"workspace", "create", "--cwd", filepath.Join(repo, "trees", "feature_abc"), "--label", "feature/abc",
		"workspace", "create", "--cwd", filepath.Join(repo, "trees", "simple"), "--label", "simple",
		"workspace", "create", "--cwd", filepath.Join(repo, "trees", "failed"), "--label", "failed",
	}, "\n") + "\n"
	if string(data) != wantHerdrCalls {
		t.Fatalf("herdr calls = %q, want %q", data, wantHerdrCalls)
	}
}

func TestWorkRequiresOneBranch(t *testing.T) {
	for _, args := range [][]string{{"work"}, {"work", "one", "two"}} {
		if _, _, err := execute(t, args...); err == nil {
			t.Fatalf("expected argument error for %q", args)
		}
	}
}

func TestWorkRejectsInvalidBranchBeforeFetch(t *testing.T) {
	repo := gitRepo(t)
	t.Chdir(repo)

	_, _, err := execute(t, "work", "../escape")
	if err == nil || err.Error() != `invalid branch "../escape"` {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "escape")); !os.IsNotExist(err) {
		t.Fatalf("worktree should not exist: %v", err)
	}
}

func TestWorkStopsWhenFetchFails(t *testing.T) {
	repo := gitRepo(t)
	t.Chdir(repo)

	if _, _, err := execute(t, "work", "feature"); err == nil || !strings.Contains(err.Error(), "fetch base branch") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".worktrees", "feature")); !os.IsNotExist(err) {
		t.Fatalf("worktree should not exist: %v", err)
	}
	cmd := exec.Command("git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/feature")
	if err := cmd.Run(); err == nil {
		t.Fatal("branch should not exist")
	}
}
