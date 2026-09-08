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
