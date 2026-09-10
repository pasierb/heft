package cli

import (
	"fmt"
	"os"
	"os/exec"
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

func TestPrunePreservesActiveAgents(t *testing.T) {
	for _, status := range []string{"working", "blocked", "unknown"} {
		t.Run(status, func(t *testing.T) {
			_, worktree := cleanupRepo(t, status)
			herdrLog := filepath.Join(t.TempDir(), "herdr.log")
			t.Setenv("HERDR_TEST_LOG", herdrLog)
			t.Setenv("HERDR_TEST_WORKTREES", fmt.Sprintf(`{"result":{"worktrees":[{"path":%q,"open_workspace_id":"w1"}]}}`, worktree))
			t.Setenv("HERDR_TEST_AGENTS", fmt.Sprintf(`{"result":{"agents":[{"workspace_id":"w1","agent_status":%q}]}}`, status))

			_, stderr, err := execute(t, "prune")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(worktree); err != nil {
				t.Fatalf("worktree should remain: %v", err)
			}
			if !strings.Contains(stderr, "herdr agent status is "+status) {
				t.Fatalf("unexpected stderr: %q", stderr)
			}
			log, err := os.ReadFile(herdrLog)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(log), "workspace\nclose\n") {
				t.Fatalf("workspace should remain open: %q", log)
			}
		})
	}
}

func TestPruneRemovesWorktreesWithSettledAgents(t *testing.T) {
	for _, status := range []string{"idle", "done"} {
		t.Run(status, func(t *testing.T) {
			_, worktree := cleanupRepo(t, status)
			t.Setenv("HERDR_TEST_WORKTREES", fmt.Sprintf(`{"result":{"worktrees":[{"path":%q,"open_workspace_id":"w1"}]}}`, worktree))
			t.Setenv("HERDR_TEST_AGENTS", fmt.Sprintf(`{"result":{"agents":[{"workspace_id":"w1","agent_status":%q}]}}`, status))

			if _, _, err := execute(t, "prune"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(worktree); !os.IsNotExist(err) {
				t.Fatalf("worktree should not exist: %v", err)
			}
		})
	}
}

func TestPrunePreservesWorktreeWhenAgentStatusIsUnavailable(t *testing.T) {
	for _, test := range []struct {
		name, response, exit string
	}{
		{"malformed", "not json", ""},
		{"missing agents", `{"result":{}}`, ""},
		{"command failure", "", "1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, worktree := cleanupRepo(t, "feature")
			t.Setenv("HERDR_TEST_WORKTREES", fmt.Sprintf(`{"result":{"worktrees":[{"path":%q,"open_workspace_id":"w1"}]}}`, worktree))
			t.Setenv("HERDR_TEST_AGENTS", test.response)
			t.Setenv("HERDR_TEST_AGENTS_EXIT", test.exit)

			_, stderr, err := execute(t, "prune")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(worktree); err != nil {
				t.Fatalf("worktree should remain: %v", err)
			}
			if !strings.Contains(stderr, "skip "+worktree+": list herdr agents") {
				t.Fatalf("unexpected stderr: %q", stderr)
			}
		})
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
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitRun(t, repo, "init", "-q", "--bare", remote)
	gitRun(t, repo, "remote", "add", "origin", remote)
	gitRun(t, repo, "push", "-q", "origin", "HEAD:refs/heads/main")
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
elif [ "$1 $2" = "agent list" ]; then
  [ -z "$HERDR_TEST_AGENTS_EXIT" ] || exit "$HERDR_TEST_AGENTS_EXIT"
  if [ -n "$HERDR_TEST_AGENTS" ]; then
    printf '%s\n' "$HERDR_TEST_AGENTS"
  else
    printf '%s\n' '{"result":{"agents":[]}}'
  fi
fi
`
	if err := os.WriteFile(herdr, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestRemovalRemoteSafety(t *testing.T) {
	for _, command := range []string{"cleanup", "prune"} {
		for _, scenario := range []string{"unpushed", "pushed", "no upstream", "detached pushed", "detached unpushed", "deleted remote branch", "rewritten remote branch", "missing origin", "fetch failure", "forced unpushed", "forced offline", "forced unstaged", "forced staged", "forced untracked"} {
			t.Run(command+"/"+scenario, func(t *testing.T) {
				repo, worktree := cleanupRepo(t, "feature")
				log := filepath.Join(t.TempDir(), "herdr.log")
				t.Setenv("HERDR_TEST_LOG", log)
				t.Setenv("HERDR_TEST_WORKTREES", fmt.Sprintf(`{"result":{"worktrees":[{"path":%q,"open_workspace_id":"w1"}]}}`, worktree))
				if scenario != "no upstream" {
					gitRun(t, worktree, "commit", "--allow-empty", "-qm", "work")
				}
				if strings.Contains(scenario, "detached") {
					gitRun(t, worktree, "checkout", "--detach")
				}
				switch scenario {
				case "pushed", "detached pushed", "deleted remote branch", "rewritten remote branch":
					gitRun(t, worktree, "push", "-q", "origin", "HEAD:refs/heads/feature")
					remote := gitRun(t, repo, "remote", "get-url", "origin")
					if scenario == "deleted remote branch" {
						gitRun(t, remote, "update-ref", "-d", "refs/heads/feature")
					} else if scenario == "rewritten remote branch" {
						gitRun(t, remote, "update-ref", "refs/heads/feature", gitRun(t, repo, "rev-parse", "HEAD"))
					}
				case "missing origin":
					gitRun(t, repo, "remote", "remove", "origin")
				case "fetch failure", "forced offline":
					gitRun(t, repo, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
				case "forced unstaged", "forced staged", "forced untracked":
					name := "file"
					if scenario == "forced untracked" {
						name = "new"
					}
					if err := os.WriteFile(filepath.Join(worktree, name), []byte("changed\n"), 0o644); err != nil {
						t.Fatal(err)
					}
					if scenario == "forced staged" {
						gitRun(t, worktree, "add", "file")
					}
				}
				args := []string{command}
				if command == "cleanup" {
					args = append(args, "feature")
				}
				if strings.HasPrefix(scenario, "forced") {
					args = append(args, "--force")
				}
				allowed := scenario == "pushed" || scenario == "detached pushed" || scenario == "no upstream" || scenario == "forced unpushed" || scenario == "forced offline"
				_, stderr, err := execute(t, args...)
				fetchFailure := scenario == "missing origin" || scenario == "fetch failure"
				wantError := !allowed && (command == "cleanup" || fetchFailure)
				if (err != nil) != wantError {
					t.Fatalf("error = %v, want error %v; stderr: %s", err, wantError, stderr)
				}
				_, statErr := os.Stat(worktree)
				if allowed {
					if !os.IsNotExist(statErr) {
						t.Fatalf("worktree should be removed: %v", statErr)
					}
					gitRun(t, repo, "show-ref", "--verify", "refs/heads/feature")
				} else {
					if statErr != nil {
						t.Fatalf("worktree should remain: %v", statErr)
					}
					message := stderr
					if err != nil {
						message += err.Error()
					}
					want := "commits missing from origin"
					if fetchFailure {
						want = "fetch origin before removal"
					} else if strings.HasPrefix(scenario, "forced") {
						want = "uncommitted changes"
					}
					if !strings.Contains(message, want) {
						t.Fatalf("expected %q in %q", want, message)
					}
					calls, readErr := os.ReadFile(log)
					if readErr != nil && !os.IsNotExist(readErr) {
						t.Fatal(readErr)
					}
					if strings.Contains(string(calls), "workspace\nclose\n") {
						t.Fatalf("workspace should remain open: %s", calls)
					}
				}
			})
		}
	}
}

func TestPruneForcePreservesActiveAgent(t *testing.T) {
	_, worktree := cleanupRepo(t, "feature")
	gitRun(t, worktree, "commit", "--allow-empty", "-qm", "unpushed")
	log := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", log)
	t.Setenv("HERDR_TEST_WORKTREES", fmt.Sprintf(`{"result":{"worktrees":[{"path":%q,"open_workspace_id":"w1"}]}}`, worktree))
	t.Setenv("HERDR_TEST_AGENTS", `{"result":{"agents":[{"workspace_id":"w1","agent_status":"working"}]}}`)
	_, stderr, err := execute(t, "prune", "--force")
	if err != nil || !strings.Contains(stderr, "herdr agent status is working") {
		t.Fatalf("error: %v; stderr: %s", err, stderr)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(log)
	if err != nil || strings.Contains(string(calls), "workspace\nclose\n") {
		t.Fatalf("workspace should remain open: %s; %v", calls, err)
	}
}

func TestCleanupFetchPreservesLocalTags(t *testing.T) {
	repo, _ := cleanupRepo(t, "feature")
	gitRun(t, repo, "tag", "local-only")
	gitRun(t, repo, "config", "--add", "remote.origin.fetch", "+refs/tags/*:refs/tags/*")
	gitRun(t, repo, "config", "fetch.pruneTags", "true")
	gitRun(t, repo, "config", "remote.origin.pruneTags", "true")
	if _, _, err := execute(t, "cleanup", "feature"); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "show-ref", "--verify", "refs/tags/local-only")
}

func TestRemovalGitChecks(t *testing.T) {
	for _, scenario := range []string{"mixed prune", "cleanup revision failure", "prune revision failure", "forced prune"} {
		t.Run(scenario, func(t *testing.T) {
			repo, first := cleanupRepo(t, "first")
			second := worktreePath(repo, "trees", "second")
			gitRun(t, repo, "worktree", "add", "-q", "-b", "second", second)
			gitRun(t, first, "commit", "--allow-empty", "-qm", "unpushed")
			herdrLog := filepath.Join(t.TempDir(), "herdr.log")
			t.Setenv("HERDR_TEST_LOG", herdrLog)
			t.Setenv("HERDR_TEST_WORKTREES", fmt.Sprintf(`{"result":{"worktrees":[{"path":%q,"open_workspace_id":"w1"}]}}`, first))
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("HEFT_TEST_REAL_GIT", realGit)
			dir := t.TempDir()
			fetchLog := filepath.Join(dir, "fetch.log")
			t.Setenv("HEFT_TEST_FETCH_LOG", fetchLog)
			if strings.Contains(scenario, "revision failure") {
				t.Setenv("HEFT_TEST_REVISION_FAILURE", "1")
			}
			script := `#!/bin/sh
for arg do
  if [ "$arg" = fetch ]; then
    echo fetch >> "$HEFT_TEST_FETCH_LOG"
  fi
  if [ "$arg" = rev-list ] && [ "$HEFT_TEST_REVISION_FAILURE" = 1 ]; then
    exit 1
  fi
done
exec "$HEFT_TEST_REAL_GIT" "$@"
`
			if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			args := []string{"prune"}
			if scenario == "cleanup revision failure" {
				args = []string{"cleanup", "first"}
			} else if scenario == "forced prune" {
				args = append(args, "--force")
			}
			_, stderr, err := execute(t, args...)
			if (err != nil) != (scenario == "cleanup revision failure") {
				t.Fatalf("error: %v; stderr: %s", err, stderr)
			}
			if strings.Contains(scenario, "revision failure") && !strings.Contains(fmt.Sprint(err)+stderr, "check unpushed commits") {
				t.Fatalf("missing revision error: %v; %s", err, stderr)
			}
			fetches, readErr := os.ReadFile(fetchLog)
			if scenario == "forced prune" {
				if !os.IsNotExist(readErr) {
					t.Fatalf("forced prune must not fetch: %s; %v", fetches, readErr)
				}
			} else if readErr != nil || string(fetches) != "fetch\n" {
				t.Fatalf("expected one fetch: %s; %v", fetches, readErr)
			}
			for _, path := range []string{first, second} {
				_, err := os.Stat(path)
				removed := scenario == "forced prune" || (scenario == "mixed prune" && path == second)
				if removed && !os.IsNotExist(err) || !removed && err != nil {
					t.Fatalf("worktree %s: removed=%v, stat error=%v", path, removed, err)
				}
			}
			calls, readErr := os.ReadFile(herdrLog)
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			if scenario != "forced prune" && strings.Contains(string(calls), "workspace\nclose\n") {
				t.Fatalf("unsafe workspace closed: %s", calls)
			}
		})
	}
}
