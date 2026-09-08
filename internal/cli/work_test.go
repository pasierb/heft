package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
	assertFileContents(t, filepath.Join(repo, ".gitignore"), "/.worktrees/\n")
	if _, err := os.Stat(filepath.Join(repo, ".worktrees", "feature")); !os.IsNotExist(err) {
		t.Fatalf("worktree should not exist: %v", err)
	}
	cmd := exec.Command("git", "-C", repo, "show-ref", "--verify", "--quiet", "refs/heads/feature")
	if err := cmd.Run(); err == nil {
		t.Fatal("branch should not exist")
	}
}
