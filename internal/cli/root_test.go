package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootShowsHelp(t *testing.T) {
	stdout, _, err := execute(t)
	if err != nil {
		t.Fatalf("execute root command: %v", err)
	}
	if !strings.Contains(stdout, "Usage:") || !strings.Contains(stdout, "heft [flags]") {
		t.Fatalf("expected help output, got %q", stdout)
	}
}

func TestCommandHelpExplainsBehaviorAndShowsExample(t *testing.T) {
	tests := []struct {
		command string
		want    []string
	}{
		{"", []string{"matching Herdr workspaces", "heft work feature/login"}},
		{"init", []string{"create or update .heft.yaml", "heft init"}},
		{"configure", []string{"Existing values are offered as defaults", "heft configure"}},
		{"work", []string{"New branches start from origin/<base_branch>", "heft work fizzy-40 --prompt"}},
		{"list", []string{"registered with the current Git repository", "heft list"}},
		{"cleanup", []string{"local\nbranch is preserved", "heft cleanup feature/login", "--force", "origin is fetched"}},
		{"prune", []string{"dirty worktrees, active agent workspaces, and local branches are preserved", "heft prune", "--force", "origin is fetched once"}},
		{"version", []string{"installed heft version", "heft version"}},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			args := []string{"--help"}
			if tt.command != "" {
				args = append([]string{tt.command}, args...)
			}
			stdout, _, err := execute(t, args...)
			if err != nil {
				t.Fatalf("show help: %v", err)
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout, want) {
					t.Errorf("help does not contain %q:\n%s", want, stdout)
				}
			}
		})
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

func TestUnknownCommand(t *testing.T) {
	_, _, err := execute(t, "unknown")
	if err == nil {
		t.Fatal("expected an error for an unknown command")
	}
	if !strings.Contains(err.Error(), `unknown command "unknown"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHerdrGuard(t *testing.T) {
	for _, args := range [][]string{{"configure"}, {"init"}, {"work", "task"}, {"list"}, {"cleanup", "task"}, {"prune"}} {
		for _, scenario := range []struct {
			name, workspace, want string
			installed             bool
		}{
			{"missing executable", "w1", "Herdr is not installed", false},
			{"missing context", "", "inside a Herdr terminal", true},
			{"blank context", " \t", "inside a Herdr terminal", true},
			{"inside Herdr", "w1", "", true},
		} {
			t.Run(args[0]+"/"+scenario.name, func(t *testing.T) {
				dir := t.TempDir()
				t.Chdir(dir)
				t.Setenv("PATH", t.TempDir())
				if scenario.installed {
					installHerdr(t)
				}
				t.Setenv("HERDR_WORKSPACE_ID", scenario.workspace)
				root := New("test")
				command, _, err := root.Find(args)
				if err != nil {
					t.Fatal(err)
				}
				ran := false
				if scenario.want == "" {
					command.RunE = func(*cobra.Command, []string) error { ran = true; return nil }
				}
				root.SetArgs(args)
				err = root.Execute()
				if scenario.want == "" {
					if err != nil || !ran {
						t.Fatalf("command did not run: %v", err)
					}
				} else if err == nil || !strings.Contains(err.Error(), scenario.want) {
					t.Fatalf("error = %v, want %q", err, scenario.want)
				}
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 0 {
					t.Fatalf("unexpected changes: %v, %v", entries, err)
				}
			})
		}
	}
}

func TestInformationalCommandsOutsideHerdr(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HERDR_WORKSPACE_ID", "")
	for _, args := range [][]string{nil, {"--help"}, {"version"}, {"--version"}, {"completion", "bash"}, {"__complete", ""}, {"configure", "--help"}, {"init", "--help"}, {"work", "--help"}, {"list", "--help"}, {"cleanup", "--help"}, {"prune", "--help"}} {
		if _, _, err := execute(t, args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
}
