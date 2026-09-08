package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupRemovesCleanWorktreeAndPreservesBranch(t *testing.T) {
	repo, worktree := cleanupRepo(t, "feature/abc")

	if _, _, err := execute(t, "cleanup", "feature/abc"); err != nil {
		t.Fatalf("execute cleanup command: %v", err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("worktree should not exist: %v", err)
	}
	gitRun(t, repo, "show-ref", "--verify", "refs/heads/feature/abc")
}

func TestCleanupClosesHerdrWorkspace(t *testing.T) {
	repo, worktree := cleanupRepo(t, "feature")
	herdrLog := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", herdrLog)
	t.Setenv("HERDR_TEST_WORKTREES", fmt.Sprintf(`{"result":{"worktrees":[{"path":%q,"open_workspace_id":"w1"}]}}`, worktree))

	if _, _, err := execute(t, "cleanup", "feature"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("worktree should not exist: %v", err)
	}
	assertFileContents(t, herdrLog, strings.Join([]string{
		"worktree", "list", "--cwd", repo,
		"workspace", "close", "w1",
	}, "\n")+"\n")
}

func TestPruneClosesHerdrWorkspaces(t *testing.T) {
	repo, first := cleanupRepo(t, "first")
	second := worktreePath(repo, "trees", "second")
	gitRun(t, repo, "worktree", "add", "-q", "-b", "second", second)
	herdrLog := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", herdrLog)
	t.Setenv("HERDR_TEST_WORKTREES", fmt.Sprintf(`{"result":{"worktrees":[{"path":%q,"open_workspace_id":"w1"},{"path":%q,"open_workspace_id":"w2"}]}}`, first, second))

	if _, _, err := execute(t, "prune"); err != nil {
		t.Fatal(err)
	}
	log, err := os.ReadFile(herdrLog)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []string{"workspace\nclose\nw1\n", "workspace\nclose\nw2\n"} {
		if !strings.Contains(string(log), call) {
			t.Fatalf("missing Herdr call %q in %q", call, log)
		}
	}
}

func TestCleanupRejectsUncommittedChanges(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*testing.T, string)
	}{
		{"unstaged", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "file"), []byte("changed\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"staged", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "file"), []byte("changed\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitRun(t, dir, "add", "file")
		}},
		{"untracked", func(t *testing.T, dir string) {
			if err := os.WriteFile(filepath.Join(dir, "new"), []byte("new\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, worktree := cleanupRepo(t, "feature")
			test.change(t, worktree)

			if _, _, err := execute(t, "cleanup", "feature"); err == nil || err.Error() != "worktree has uncommitted changes" {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, err := os.Stat(worktree); err != nil {
				t.Fatalf("worktree should remain: %v", err)
			}
		})
	}
}

func TestCleanupRequiresOneValidBranch(t *testing.T) {
	for _, args := range [][]string{{"cleanup"}, {"cleanup", "one", "two"}} {
		if _, _, err := execute(t, args...); err == nil {
			t.Fatalf("expected argument error for %q", args)
		}
	}

	repo := gitRepo(t)
	t.Chdir(repo)
	if _, _, err := execute(t, "cleanup", "../escape"); err == nil || err.Error() != `invalid branch "../escape"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPruneRemovesCleanWorktreesAndSkipsDirtyOnes(t *testing.T) {
	repo, clean := cleanupRepo(t, "clean")
	dirty := worktreePath(repo, "trees", "dirty")
	later := worktreePath(repo, "trees", "later")
	gitRun(t, repo, "worktree", "add", "-q", "-b", "dirty", dirty)
	gitRun(t, repo, "worktree", "add", "-q", "-b", "later", later)
	if err := os.WriteFile(filepath.Join(dirty, "changed"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := execute(t, "prune")
	if err != nil {
		t.Fatalf("execute prune command: %v", err)
	}
	for _, path := range []string{clean, later} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("clean worktree should not exist: %s: %v", path, err)
		}
	}
	if _, err := os.Stat(dirty); err != nil {
		t.Fatalf("dirty worktree should remain: %v", err)
	}
	if !strings.Contains(stderr, "skip "+dirty+": worktree has uncommitted changes") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if _, err := os.Stat(repo); err != nil {
		t.Fatalf("primary worktree should remain: %v", err)
	}
	for _, branch := range []string{"clean", "dirty", "later"} {
		gitRun(t, repo, "show-ref", "--verify", "refs/heads/"+branch)
	}
}

func TestPruneRejectsArguments(t *testing.T) {
	if _, _, err := execute(t, "prune", "feature"); err == nil {
		t.Fatal("expected argument error")
	}
}

func cleanupRepo(t *testing.T, branch string) (string, string) {
	t.Helper()
	installHerdrForCleanup(t)
	repo := gitRepo(t)
	gitRun(t, repo, "config", "user.email", "test@example.com")
	gitRun(t, repo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "file"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "file")
	gitRun(t, repo, "commit", "-qm", "initial")
	if err := os.WriteFile(filepath.Join(repo, ".heft.yaml"), []byte("worktrees_dir: trees\nbase_branch: main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree := worktreePath(repo, "trees", branch)
	gitRun(t, repo, "worktree", "add", "-q", "-b", branch, worktree)
	t.Chdir(repo)
	return repo, worktree
}

func installHerdrForCleanup(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	herdr := filepath.Join(dir, "herdr")
	contents := `#!/bin/sh
[ -z "$HERDR_TEST_LOG" ] || printf '%s\n' "$@" >> "$HERDR_TEST_LOG"
if [ "$1 $2" = "worktree list" ]; then
  if [ -n "$HERDR_TEST_WORKTREES" ]; then
    printf '%s\n' "$HERDR_TEST_WORKTREES"
  else
    printf '%s\n' '{"result":{"worktrees":[]}}'
  fi
fi
`
	if err := os.WriteFile(herdr, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
