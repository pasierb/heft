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
