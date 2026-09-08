package cli

import "testing"

func TestVersionCommand(t *testing.T) {
	stdout, _, err := execute(t, "version")
	if err != nil {
		t.Fatalf("execute version command: %v", err)
	}
	if want := "heft v1.2.3\n"; stdout != want {
		t.Fatalf("version output = %q, want %q", stdout, want)
	}
}
