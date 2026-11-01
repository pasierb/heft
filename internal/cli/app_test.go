package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mockApp(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	app, log := filepath.Join(dir, "herdr-app"), filepath.Join(dir, "calls")
	t.Setenv("APP_LOG", log)
	t.Setenv("APP_COUNT", filepath.Join(dir, "count"))
	t.Setenv("APP_FAIL", "")
	script := `#!/bin/sh
printf '%s\n%s\n' "$2" "$3" >> "$APP_LOG"
if [ -n "$APP_FAIL" ]; then printf '%s' "$APP_FAIL"; exit 1; fi
if [ "$2" = tab.select ]; then printf '{}'; exit; fi
n=0
if [ -f "$APP_COUNT" ]; then read -r n < "$APP_COUNT"; fi
n=$((n + 1))
printf '%s' "$n" > "$APP_COUNT"
printf '{"space":"sp1","tab":"tb%s","terminal":"t%s"}' "$n" "$n"
`
	if err := os.WriteFile(app, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return app, log
}

func TestAppWorkCreatesConfiguredTabs(t *testing.T) {
	cmd, coreLog := mockCore(t, "")
	app, log := mockApp(t)
	command := "agent --flag 'two words' && echo done"
	err := appWork(cmd, app, "/repo", "/repo/tree with spaces", "custom label", []tab{
		{Name: "shell"}, {Name: "agent", Command: command, Agent: true}, {Name: "notes", Command: "nvim"},
	}, "do work", false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 8 {
		t.Fatalf("expected three creations and one selection: %s", data)
	}
	for i, action := range []string{"space.new", "tab.new", "tab.new", "tab.select"} {
		if lines[2*i] != action {
			t.Fatalf("action %d = %s", i, lines[2*i])
		}
	}
	var first, second, selected map[string]any
	for line, target := range map[int]*map[string]any{1: &first, 3: &second, 7: &selected} {
		if err := json.Unmarshal([]byte(lines[line]), target); err != nil {
			t.Fatal(err)
		}
	}
	if first["name"] != "custom label" || first["tab"] != "shell" || first["folder"] != "/repo/tree with spaces" || first["argv"] != nil {
		t.Fatalf("first tab args: %v", first)
	}
	argv := second["argv"].([]any)
	if second["space"] != "sp1" || second["name"] != "agent" || len(argv) != 3 || argv[0] != "sh" || argv[1] != "-c" || argv[2] != command || selected["id"] != "tb2" {
		t.Fatalf("agent args: %v; selected: %v", second, selected)
	}
	core, _ := os.ReadFile(coreLog)
	if strings.Count(string(core), "--json label ") != 3 || !strings.Contains(string(core), "heft.worktree=/repo/tree with spaces") || !strings.Contains(string(core), "send t2 do work") || strings.Contains(string(core), "run ") {
		t.Fatalf("unexpected core operations: %s", core)
	}
}

func TestAppWorkDefaultShellAndErrors(t *testing.T) {
	for _, failure := range []string{"", `{"error":"no window"}`, `{}`} {
		t.Run(failure, func(t *testing.T) {
			cmd, coreLog := mockCore(t, "")
			app, log := mockApp(t)
			t.Setenv("APP_FAIL", failure)
			err := appWork(cmd, app, "/repo", "/tree", "task", nil, "", true)
			if failure == "" {
				if err != nil {
					t.Fatal(err)
				}
				data, _ := os.ReadFile(log)
				if !strings.Contains(string(data), `"tab":"shell"`) || !strings.Contains(cmd.ErrOrStderr().(*bytes.Buffer).String(), "--no-focus") {
					t.Fatalf("missing default shell or notice: %s", data)
				}
			} else {
				if err == nil {
					t.Fatal("expected app failure")
				}
				if _, err := os.Stat(coreLog); !os.IsNotExist(err) {
					t.Fatal("core invoked after app failure")
				}
			}
		})
	}
}
