package cli

import (
	"os"
	"path/filepath"
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

func cleanupRepo(t *testing.T, branch string) (string, string) {
	t.Helper()
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
