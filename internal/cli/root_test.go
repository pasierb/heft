package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func execute(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := New("v1.2.3")
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()

	return stdout.String(), stderr.String(), err
}

func TestInitChecksForHerdr(t *testing.T) {
	t.Run("installed", func(t *testing.T) {
		dir := t.TempDir()
		herdr := filepath.Join(dir, "herdr")
		if err := os.WriteFile(herdr, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)

		stdout, _, err := execute(t, "init")
		if err != nil {
			t.Fatalf("execute init command: %v", err)
		}
		if want := "herdr is installed\n"; stdout != want {
			t.Fatalf("init output = %q, want %q", stdout, want)
		}
	})

	t.Run("missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		_, _, err := execute(t, "init")
		if err == nil || err.Error() != "herdr is not installed or not in PATH" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

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

func TestVersionCommand(t *testing.T) {
	stdout, _, err := execute(t, "version")
	if err != nil {
		t.Fatalf("execute version command: %v", err)
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
