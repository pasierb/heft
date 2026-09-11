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

func TestRepositoryNameIsStableAcrossWorktrees(t *testing.T) {
	repo := workRepo(t)
	worktree := filepath.Join(t.TempDir(), "linked")
	gitRun(t, repo, "worktree", "add", "-qb", "linked", worktree)

	want := filepath.Base(repo)
	for _, root := range []string{repo, worktree} {
		got, err := repositoryName(root)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("repositoryName(%q) = %q, want %q", root, got, want)
		}
	}
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

func installHerdrForTabs(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	herdr := filepath.Join(dir, "herdr")
	contents := `#!/bin/sh
printf '%s\n' "$@" >> "$HERDR_TEST_LOG"
case "$*" in
  *"$HERDR_FAIL_MATCH"*) [ -z "$HERDR_FAIL_MATCH" ] || exit 1 ;;
esac
case "$1 $2" in
  "workspace create") printf '%s\n' '{"result":{"workspace":{"workspace_id":"w1"},"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}' ;;
  "tab create") printf '%s\n' '{"result":{"tab":{"tab_id":"t2"},"root_pane":{"pane_id":"p2"}}}' ;;
esac
`
	if err := os.WriteFile(herdr, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s = %q, want %q", filepath.Base(path), data, want)
	}
}

func TestProjectRootAcrossWorktrees(t *testing.T) {
	original := workRepo(t)
	repo := filepath.Join(t.TempDir(), "primary space\nline")
	if err := os.Rename(original, repo); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(t.TempDir(), "linked space\nline")
	gitRun(t, repo, "worktree", "add", "-qb", "linked", linked)
	for _, base := range []string{repo, linked} {
		nested := filepath.Join(base, "nested")
		if err := os.Mkdir(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, dir := range []string{base, nested} {
			t.Run(dir, func(t *testing.T) {
				t.Chdir(dir)
				got, err := projectRoot()
				if err != nil || got != repo {
					t.Fatalf("projectRoot() = %q, %v; want %q", got, err, repo)
				}
			})
		}
	}
}

func TestProjectRootRejectsBareRepository(t *testing.T) {
	dir := t.TempDir()
	gitRun(t, dir, "init", "--bare", "-q")
	t.Chdir(dir)
	if _, err := projectRoot(); err == nil {
		t.Fatal("expected an error for a repository without a primary checkout")
	}
}

func TestSubmoduleRootAndProfileAcrossWorktrees(t *testing.T) {
	source := workRepo(t)
	parent := gitRepo(t)
	gitRun(t, parent, "-c", "protocol.file.allow=always", "submodule", "add", source, "packages/module")
	repo := filepath.Join(parent, "packages/module")
	linked := filepath.Join(t.TempDir(), "linked")
	gitRun(t, repo, "worktree", "add", "-qb", "linked", linked)
	writeCopyFile(t, repo, ".heft.yaml", "worktrees_dir: trees\nbase_branch: main\nprofiles:\n  fable:\n    tabs:\n      - name: fable-shell\n")
	writeCopyFile(t, linked, ".heft.yaml", "invalid: [")
	installHerdrForTabs(t)
	log := filepath.Join(t.TempDir(), "herdr.log")
	t.Setenv("HERDR_TEST_LOG", log)
	for name, base := range map[string]string{"primary": repo, "linked": linked} {
		t.Run(name, func(t *testing.T) {
			nested := filepath.Join(base, "nested")
			if err := os.Mkdir(nested, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, dir := range []string{base, nested} {
				t.Chdir(dir)
				got, err := projectRoot()
				if err != nil || got != repo {
					t.Fatalf("projectRoot() = %q, %v; want %q", got, err, repo)
				}
			}
			if _, _, err := execute(t, "work", "feature-"+name, "--profile", "fable"); err != nil {
				t.Fatal(err)
			}
			if got := gitRun(t, filepath.Join(repo, "trees", "feature-"+name), "branch", "--show-current"); got != "feature-"+name {
				t.Fatalf("unexpected branch: %q", got)
			}
		})
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "tab\nrename\nt1\nfable-shell\n") != 2 {
		t.Fatalf("profile tabs were not opened: %s", calls)
	}
}
