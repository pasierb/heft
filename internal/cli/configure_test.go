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

	stdout, _, err := executeWithInput(t, "trees\ndevelop\n\n2\n", "configure")
	if err != nil {
		t.Fatalf("execute configure command: %v", err)
	}
	if want := "Worktrees directory [.worktrees]: Base branch [main]: Workspace prefix [" + filepath.Base(dir) + "]: Default harness:\n  1) Claude\n  2) Codex\n  3) Agy\n  4) Other\nSelect [1-4]: "; stdout != want {
		t.Fatalf("configure output = %q, want %q", stdout, want)
	}
	path := filepath.Join(dir, ".heft.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "worktrees_dir: trees\nbase_branch: develop\nworkspace_prefix: \"" + filepath.Base(dir) + "\"\ntabs:\n    - name: codex\n      command: codex\n      agent: true\n    - name: shell\n"; string(data) != want {
		t.Fatalf("config = %q, want %q", data, want)
	}
	assertFileContents(t, filepath.Join(dir, ".gitignore"), "/vendor/\n/trees/\n")

	if _, _, err := executeWithInput(t, "new trees\n\n custom \n", "configure"); err != nil {
		t.Fatalf("reconfigure: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "worktrees_dir: new trees\nbase_branch: develop\nworkspace_prefix: custom\ntabs:\n    - name: codex\n      command: codex\n      agent: true\n    - name: shell\n"; string(data) != want {
		t.Fatalf("reconfigured config = %q, want %q", data, want)
	}
	assertFileContents(t, filepath.Join(dir, ".gitignore"), "/vendor/\n/trees/\n/new trees/\n")
	if _, _, err := executeWithInput(t, "\n\n", "configure"); err != nil {
		t.Fatalf("configure without changes: %v", err)
	}
	assertFileContents(t, filepath.Join(dir, ".gitignore"), "/vendor/\n/trees/\n/new trees/\n")
}

func TestConfigureHarnessChoices(t *testing.T) {
	for name, test := range map[string]struct {
		input string
		want  string
	}{
		"claude": {"1\n", "claude\n      command: claude"},
		"agy":    {"3\n", "agy\n      command: agy"},
		"other":  {"nope\n4\n\n/usr/local/bin/my-agent --flag\n", "my-agent\n      command: /usr/local/bin/my-agent --flag"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := gitRepo(t)
			t.Chdir(dir)
			if _, _, err := executeWithInput(t, "\n\n\n"+test.input, "configure"); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, ".heft.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), "    - name: "+test.want+"\n      agent: true\n    - name: shell\n") {
				t.Fatalf("unexpected config %q", data)
			}
		})
	}
}

func TestConfigureHarnessRequiresInput(t *testing.T) {
	dir := gitRepo(t)
	t.Chdir(dir)
	if _, _, err := executeWithInput(t, "\n\n\n", "configure"); err == nil || !strings.Contains(err.Error(), "input ended before a valid selection") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".heft.yaml")); !os.IsNotExist(err) {
		t.Fatalf("config should not exist: %v", err)
	}
}

func TestConfigurePreservesTabs(t *testing.T) {
	dir := gitRepo(t)
	t.Chdir(dir)
	path := filepath.Join(dir, ".heft.yaml")
	if err := os.WriteFile(path, []byte("worktrees_dir: trees\nbase_branch: main\ntabs:\n  - name: codex\n    command: codex --dangerously-bypass-approvals-and-sandbox\n    agent: true\n  - name: shell\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := executeWithInput(t, "\n\n", "configure"); err != nil {
		t.Fatal(err)
	}
	assertFileContents(t, path, "worktrees_dir: trees\nbase_branch: main\nworkspace_prefix: \""+filepath.Base(dir)+"\"\ntabs:\n    - name: codex\n      command: codex --dangerously-bypass-approvals-and-sandbox\n      agent: true\n    - name: shell\n")
}

func TestConfigurePreservesUnknownFields(t *testing.T) {
	dir := gitRepo(t)
	t.Chdir(dir)
	path := filepath.Join(dir, ".heft.yaml")
	if err := os.WriteFile(path, []byte("worktrees_dir: trees\nbase_branch: main\nfuture: value\ntabs:\n  - name: codex\n    command: codex\n    agent: true\n    future_tab: value\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := executeWithInput(t, "\n\n\n", "configure"); err != nil {
		t.Fatal(err)
	}
	assertFileContents(t, path, "worktrees_dir: trees\nbase_branch: main\nworkspace_prefix: \""+filepath.Base(dir)+"\"\ntabs:\n    - name: codex\n      command: codex\n      agent: true\n      future_tab: value\nfuture: value\n")
}

func TestReadConfigAcceptsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".heft.yaml")
	contents := []byte("worktrees_dir: trees\nbase_branch: main\nfuture: value\ntabs:\n  - name: shell\n    future_tab: value\n")
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := readConfig(path); err != nil {
		t.Fatal(err)
	}
	assertFileContents(t, path, string(contents))
}

func TestConfigureRepairsMissingRequiredFields(t *testing.T) {
	dir := gitRepo(t)
	t.Chdir(dir)
	path := filepath.Join(dir, ".heft.yaml")
	if err := os.WriteFile(path, []byte("future: value\ntabs:\n  - name: codex\n    command: codex\n    agent: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := executeWithInput(t, "\n\n\n", "configure"); err != nil {
		t.Fatal(err)
	}
	assertFileContents(t, path, "worktrees_dir: .worktrees\nbase_branch: main\nworkspace_prefix: \""+filepath.Base(dir)+"\"\ntabs:\n    - name: codex\n      command: codex\n      agent: true\nfuture: value\n")
}

func TestReadConfigRejectsInvalidTabs(t *testing.T) {
	for name, contents := range map[string]string{
		"malformed":             "worktrees_dir: trees\nbase_branch: main\ntabs: nope\n",
		"empty name":            "worktrees_dir: trees\nbase_branch: main\ntabs:\n  - name: '  '\n",
		"missing base":          "worktrees_dir: trees\ntabs: []\n",
		"multiple docs":         "worktrees_dir: trees\nbase_branch: main\n---\nworktrees_dir: other\n",
		"agent without command": "worktrees_dir: trees\nbase_branch: main\ntabs:\n  - name: codex\n    agent: true\n",
		"multiple agents":       "worktrees_dir: trees\nbase_branch: main\ntabs:\n  - name: one\n    command: codex\n    agent: true\n  - name: two\n    command: claude\n    agent: true\n",
		"invalid profile tabs":  "worktrees_dir: trees\nbase_branch: main\nprofiles:\n  research:\n    tabs:\n      - name: ''\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".heft.yaml")
			if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := readConfig(path); err == nil {
				t.Fatal("expected invalid config")
			}
		})
	}
}

func TestConfigureRejectsInvalidConfig(t *testing.T) {
	dir := gitRepo(t)
	t.Chdir(dir)
	path := filepath.Join(dir, ".heft.yaml")
	original := []byte("worktrees_dir: [\n")
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
