package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func mockCore(t *testing.T, list string) (*cobra.Command, string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$CORE_LOG"
case "$*" in
 *" ls"*) printf '%s' "$CORE_LIST" ;;
 *" get "*) if [ -f "$CORE_DETAILS/$3" ]; then cat "$CORE_DETAILS/$3"; exit; fi; printf '%s' '{"address":"t1","state":"live","status":{"state":"idle"}}' ;;
 *"run "*) printf '%s' '{"address":"t2","state":"live"}' ;;
 *) printf '%s' '{}' ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "hc"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("CORE_LOG", log)
	var details struct {
		Terminals []coreTerminal `json:"terminals"`
	}
	if list != "" {
		if err := json.Unmarshal([]byte(list), &details); err != nil {
			t.Fatal(err)
		}
	}
	compact := make([]map[string]any, 0, len(details.Terminals))
	for _, terminal := range details.Terminals {
		data, err := json.Marshal(terminal)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, terminal.Address), data, 0600); err != nil {
			t.Fatal(err)
		}
		status := ""
		if terminal.Status != nil {
			status = terminal.Status.State
		}
		compact = append(compact, map[string]any{"address": terminal.Address, "status": status})
	}
	encoded, err := json.Marshal(map[string]any{"terminals": compact})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORE_DETAILS", dir)
	t.Setenv("CORE_LIST", string(encoded))
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	return cmd, log
}

func TestCoreLaunchAndPrompt(t *testing.T) {
	cmd, log := mockCore(t, "")
	err := coreWork(cmd, "/repo", "/repo/tree", "task", []tab{{Name: "agent", Command: "agent --flag 'two words'", Agent: true}, {Name: "shell"}}, "do work")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	for _, want := range []string{"--no-parent", "heft.worktree=/repo/tree", "herdr.space=heft-", "-- agent --flag 'two words'", "send t2 do work"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("missing %q in %s", want, data)
		}
	}
}

func TestCoreCleanupProtectionAndDescendants(t *testing.T) {
	for _, state := range []string{"working", "blocked", "error", "unknown", "idle", "done"} {
		t.Run(state, func(t *testing.T) {
			cmd, log := mockCore(t, `{"terminals":[{"address":"t2","labels":{"heft.root":"/repo","heft.worktree":"/tree","herdr.space":"s"}},{"address":"t3","labels":{},"parent":"t2","status":{"state":"`+state+`"}}]}`)
			err := coreClose(cmd, "/repo", "/tree", true)
			if (err == nil) != (state == "idle" || state == "done") {
				t.Fatalf("unexpected result: %v", err)
			}
			data, _ := os.ReadFile(log)
			if err != nil && strings.Contains(string(data), "kill") {
				t.Fatal("killed active group")
			}
		})
	}
}

func TestCoreCallerDeferred(t *testing.T) {
	cmd, log := mockCore(t, `{"terminals":[{"address":"t1","labels":{"heft.root":"/repo","heft.worktree":"/tree"}}]}`)
	t.Setenv("HC_TERMINAL_ID", "t1")
	if err := coreClose(cmd, "/repo", "/tree", false); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	if strings.Contains(string(data), "kill") {
		t.Fatal("caller killed early")
	}
	if cmd.PostRunE == nil {
		t.Fatal("missing deferred close")
	}
	if err := cmd.PostRunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
}

// Run explicitly with HEFT_TEST_CORE=1; all core state belongs to this test.
func TestCoreRealIsolated(t *testing.T) {
	if os.Getenv("HEFT_TEST_CORE") != "1" {
		t.Skip("set HEFT_TEST_CORE=1 to exercise installed hc")
	}
	scratch := t.TempDir()
	t.Setenv("HC_SESSION", "heft-test-"+filepath.Base(scratch))
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR"} {
		dir := filepath.Join(scratch, key)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, dir)
	}
	t.Setenv("HC_TERMINAL_ID", "")
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := coreCall(cmd, nil, "server", "start"); err != nil {
		t.Fatal(err)
	}
	defer coreCall(cmd, nil, "server", "stop")
	if err := coreWork(cmd, scratch, scratch, "test", []tab{{Name: "shell", Command: "sh"}}, ""); err != nil {
		t.Fatal(err)
	}
	var list struct {
		Terminals []coreTerminal `json:"terminals"`
	}
	if err := coreCall(cmd, &list, "ls"); err != nil {
		t.Fatal(err)
	}
	if len(list.Terminals) == 1 {
		if err := coreCall(cmd, &list.Terminals[0], "get", list.Terminals[0].Address); err != nil {
			t.Fatal(err)
		}
	}
	if len(list.Terminals) != 1 || list.Terminals[0].Labels["herdr.space"] == "" {
		t.Fatalf("unexpected terminals: %+v", list.Terminals)
	}
	if err := coreClose(cmd, scratch, scratch, true); err != nil {
		t.Fatal(err)
	}
}
