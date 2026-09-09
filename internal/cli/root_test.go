package cli

import (
	"strings"
	"testing"
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
		{"init", []string{".heft.yaml does not exist", "heft init"}},
		{"configure", []string{"Existing values are offered as defaults", "heft configure"}},
		{"work", []string{"New branches start from origin/<base_branch>", "heft work fizzy-40 --prompt"}},
		{"list", []string{"registered with the current Git repository", "heft list"}},
		{"cleanup", []string{"local\nbranch is preserved", "heft cleanup feature/login"}},
		{"prune", []string{"dirty worktrees, active agent workspaces, and local branches are preserved", "heft prune"}},
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
