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

	if err := os.WriteFile(filepath.Join(repo, ".heft.yaml"), []byte("worktrees_dir: trees\nbase_branch: main\nworkspace_prefix: '   '\ntabs: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execute(t, "work", "simple", "--no-focus"); err != nil {
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
		"workspace", "create", "--cwd", filepath.Join(repo, "trees", "feature_abc"), "--label", filepath.Base(repo) + " feature/abc",
		"workspace", "create", "--cwd", filepath.Join(repo, "trees", "simple"), "--label", filepath.Base(repo) + " simple", "--no-focus",
		"workspace", "create", "--cwd", filepath.Join(repo, "trees", "failed"), "--label", filepath.Base(repo) + " failed",
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

func TestWorkReusesExistingBranches(t *testing.T) {
	for _, remote := range []bool{false, true} {
		name := "local"
		location := "locally"
		if remote {
			name = "remote"
			location = "on origin"
		}
		t.Run(name, func(t *testing.T) {
			repo := workRepo(t)
			installHerdr(t)
			if err := os.WriteFile(filepath.Join(repo, ".heft.yaml"), []byte("worktrees_dir: .worktrees\nbase_branch: main\ntabs: []\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitRun(t, repo, "checkout", "-qb", "feature")
			if err := os.WriteFile(filepath.Join(repo, "file"), []byte(name+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			gitRun(t, repo, "commit", "-qam", name)
			wantCommit := gitRun(t, repo, "rev-parse", "HEAD")
			gitRun(t, repo, "checkout", "-q", "main")
			if remote {
				gitRun(t, repo, "push", "-qu", "origin", "feature")
				gitRun(t, repo, "branch", "-D", "feature")
			}
			t.Chdir(repo)

			stdout, _, err := execute(t, "work", "feature")
			if err != nil {
				t.Fatal(err)
			}
			worktree := filepath.Join(repo, ".worktrees", "feature")
			if got := gitRun(t, worktree, "rev-parse", "HEAD"); got != wantCommit {
				t.Fatalf("commit = %q, want %q", got, wantCommit)
			}
			if !strings.Contains(stdout, "already exists "+location) {
				t.Fatalf("unexpected output: %q", stdout)
			}
			if remote {
				if got := gitRun(t, worktree, "rev-parse", "--abbrev-ref", "@{upstream}"); got != "origin/feature" {
					t.Fatalf("upstream = %q, want origin/feature", got)
				}
			}
		})
	}
}

func TestListShowsWorktrees(t *testing.T) {
	repo := workRepo(t)
	worktree := filepath.Join(t.TempDir(), "feature")
	gitRun(t, repo, "worktree", "add", "-qb", "feature", worktree)
	t.Chdir(worktree)

	stdout, _, err := execute(t, "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, repo) || !strings.Contains(stdout, worktree) || !strings.Contains(stdout, "[feature]") {
		t.Fatalf("unexpected worktree list: %q", stdout)
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

func TestWorkCreatesConfiguredTabsInOrder(t *testing.T) {
	repo := workRepo(t)
	installHerdrForTabs(t)
	log := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", log)
	if err := os.WriteFile(filepath.Join(repo, ".heft.yaml"), []byte("worktrees_dir: trees\nbase_branch: main\nworkspace_prefix: custom\ntabs:\n  - name: codex\n    command: codex --model gpt-5\n  - name: shell\nprofiles:\n  other:\n    tabs:\n      - name: ignored\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	if _, _, err := execute(t, "work", "feature", "--label", "ticket 39"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "trees", "feature")
	want := strings.Join([]string{
		"workspace", "create", "--cwd", path, "--label", "ticket 39", "--focus",
		"tab", "rename", "t1", "codex",
		"pane", "run", "p1", "codex --model gpt-5",
		"tab", "create", "--workspace", "w1", "--cwd", path, "--label", "shell", "--no-focus",
	}, "\n") + "\n"
	assertFileContents(t, log, want)
}

func TestWorkCreatesProfileTabs(t *testing.T) {
	repo := workRepo(t)
	installHerdrForTabs(t)
	log := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", log)
	if err := os.WriteFile(filepath.Join(repo, ".heft.yaml"), []byte("worktrees_dir: trees\nbase_branch: main\ntabs:\n  - name: shell\nprofiles:\n  research:\n    tabs:\n      - name: codex\n        command: codex --model gpt-5\n      - name: notes\n        command: nvim\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	if _, _, err := execute(t, "work", "feature", "--profile", "research"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "trees", "feature")
	want := strings.Join([]string{
		"workspace", "create", "--cwd", path, "--label", filepath.Base(repo) + " feature", "--focus",
		"tab", "rename", "t1", "codex",
		"pane", "run", "p1", "codex --model gpt-5",
		"tab", "create", "--workspace", "w1", "--cwd", path, "--label", "notes", "--no-focus",
		"pane", "run", "p2", "nvim",
	}, "\n") + "\n"
	assertFileContents(t, log, want)
}

func TestWorkRejectsUnknownProfileBeforeSideEffects(t *testing.T) {
	repo := workRepo(t)
	t.Chdir(repo)

	_, _, err := execute(t, "work", "feature", "--profile", "missing")
	if err == nil || err.Error() != `profile "missing" is not configured` {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("gitignore should not exist: %v", err)
	}
}

func TestWorkCreatesConfiguredTabsWithoutFocus(t *testing.T) {
	repo := workRepo(t)
	installHerdrForTabs(t)
	log := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", log)
	if err := os.WriteFile(filepath.Join(repo, ".heft.yaml"), []byte("worktrees_dir: trees\nbase_branch: main\ntabs:\n  - name: shell\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	if _, _, err := execute(t, "work", "feature", "--no-focus"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(repo, "trees", "feature")
	want := strings.Join([]string{
		"workspace", "create", "--cwd", path, "--label", filepath.Base(repo) + " feature", "--no-focus",
		"tab", "rename", "t1", "shell",
	}, "\n") + "\n"
	assertFileContents(t, log, want)
}

func TestWorkPromptsConfiguredAgentTab(t *testing.T) {
	repo := workRepo(t)
	installHerdrForTabs(t)
	log := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", log)
	if err := os.WriteFile(filepath.Join(repo, ".heft.yaml"), []byte("worktrees_dir: trees\nbase_branch: main\ntabs:\n  - name: shell\nprofiles:\n  research:\n    tabs:\n      - name: shell\n      - name: codex\n        command: codex\n        agent: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	if _, _, err := execute(t, "work", "feature", "--profile", "research", "--prompt", "fix the failing test"); err != nil {
		t.Fatal(err)
	}
	lines, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := "agent\nget\np2\nagent\nwait\np2\n"
	got := string(lines)
	if !strings.Contains(got, wantSuffix+"--timeout\n") || !strings.HasSuffix(got, "agent\nprompt\np2\nfix the failing test\n") {
		t.Fatalf("unexpected herdr calls: %q", got)
	}
}

func TestWorkPromptRequiresAgentCommand(t *testing.T) {
	repo := workRepo(t)
	t.Chdir(repo)

	_, _, err := execute(t, "work", "feature", "--prompt", "do the work")
	if err == nil || err.Error() != "--prompt requires a configured agent tab" {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("gitignore should not exist: %v", err)
	}
}

func TestWorkConfiguredTabsStopOnHerdrFailure(t *testing.T) {
	for _, fail := range []string{"workspace create", "tab rename", "pane run", "tab create"} {
		t.Run(fail, func(t *testing.T) {
			repo := workRepo(t)
			installHerdrForTabs(t)
			t.Setenv("HERDR_TEST_LOG", filepath.Join(t.TempDir(), "herdr.log"))
			t.Setenv("HERDR_FAIL_MATCH", fail)
			if err := os.WriteFile(filepath.Join(repo, ".heft.yaml"), []byte("worktrees_dir: trees\nbase_branch: main\ntabs:\n  - name: codex\n    command: codex --model gpt-5\n  - name: shell\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Chdir(repo)
			if _, _, err := execute(t, "work", "feature"); err == nil {
				t.Fatal("expected Herdr error")
			}
		})
	}
}

func workRepo(t *testing.T) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "origin.git")
	if out, err := exec.Command("git", "init", "-q", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, out)
	}
	repo := gitRepo(t)
	gitRun(t, repo, "config", "user.email", "test@example.com")
	gitRun(t, repo, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repo, "file"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "file")
	gitRun(t, repo, "commit", "-qm", "initial")
	gitRun(t, repo, "branch", "-M", "main")
	gitRun(t, repo, "remote", "add", "origin", remote)
	gitRun(t, repo, "push", "-qu", "origin", "main")
	return repo
}
