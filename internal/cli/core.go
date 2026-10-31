package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type coreTerminal struct {
	Address  string            `json:"address"`
	State    string            `json:"state"`
	Parent   string            `json:"parent"`
	Labels   map[string]string `json:"labels"`
	Reported bool              `json:"reported"`
	Status   *struct {
		State string `json:"state"`
	} `json:"status"`
}

func coreMode() bool { return strings.TrimSpace(os.Getenv("HC_TERMINAL_ID")) != "" }

func coreCall(cmd *cobra.Command, result any, args ...string) error {
	process := exec.CommandContext(cmd.Context(), "hc", append([]string{"--json"}, args...)...)
	process.Stderr = cmd.ErrOrStderr()
	out, err := process.Output()
	if err != nil {
		return fmt.Errorf("hc %s: %w", args[0], err)
	}
	if result != nil {
		if err := json.Unmarshal(out, result); err != nil {
			return fmt.Errorf("decode hc %s: %w", args[0], err)
		}
	}
	return nil
}

func coreWork(cmd *cobra.Command, root, path, label string, tabs []tab, prompt string) error {
	fmt.Fprintln(cmd.ErrOrStderr(), "Herdr core: visible names are automatic; focus is controlled by the app (including --label and --no-focus).")
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	space := "heft-" + hex.EncodeToString(id)
	if len(tabs) == 0 {
		tabs = []tab{{Name: "shell"}}
	}
	agentAddress := ""
	for i, tab := range tabs {
		args := []string{"run", "--json", "--bg", "--keep", "--no-parent", "--cwd", path}
		for _, pair := range []string{"herdr.space=" + space, fmt.Sprintf("herdr.tab=%s-%d", space, i), "heft.root=" + root, "heft.worktree=" + path, "heft.workspace=" + label, "heft.tab=" + tab.Name} {
			args = append(args, "--label", pair)
		}
		// Put the command after -- so its flags cannot become hc options.
		args = append(args, "--")
		if tab.Command != "" {
			args = append(args, tab.Command)
		}
		var terminal coreTerminal
		process := exec.CommandContext(cmd.Context(), "hc", args...)
		process.Stderr = cmd.ErrOrStderr()
		out, err := process.Output()
		if err != nil {
			return fmt.Errorf("launch core tab %q (previous terminals retained): %w", tab.Name, err)
		}
		if err = json.Unmarshal(out, &terminal); err != nil || terminal.Address == "" {
			return fmt.Errorf("launch core tab %q: invalid terminal response %s", tab.Name, out)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created core terminal %s for %s\n", terminal.Address, tab.Name)
		if tab.Agent && prompt != "" {
			agentAddress = terminal.Address
		}
	}
	if agentAddress != "" {
		return corePrompt(cmd, agentAddress, prompt)
	}
	return nil
}

func corePrompt(cmd *cobra.Command, address, prompt string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()
	probe := &cobra.Command{}
	probe.SetContext(ctx)
	probe.SetErr(cmd.ErrOrStderr())
	for {
		var t coreTerminal
		if err := coreCall(probe, &t, "get", address); err != nil {
			return err
		}
		if t.State != "live" {
			return fmt.Errorf("core agent %s exited", address)
		}
		if t.Status != nil {
			switch t.Status.State {
			case "idle", "done":
				return coreCall(probe, nil, "send", address, prompt)
			case "working":
			default:
				return fmt.Errorf("core agent %s status is %s", address, t.Status.State)
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for core agent %s: %w", address, ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func coreClose(cmd *cobra.Command, root, path string, preserve bool) error {
	var summary struct {
		Terminals *[]struct {
			Address string `json:"address"`
		} `json:"terminals"`
	}
	if err := coreCall(cmd, &summary, "ls"); err != nil {
		return err
	}
	if summary.Terminals == nil {
		return fmt.Errorf("core list response missing terminals")
	}
	var listed struct{ Terminals *[]coreTerminal }
	terminals := make([]coreTerminal, 0, len(*summary.Terminals))
	// ls is a compact CLI view: status is a string, unlike get's object.
	// Decode addresses only, then always fetch authoritative terminal details.
	for _, entry := range *summary.Terminals {
		if entry.Address == "" {
			return fmt.Errorf("core list response missing terminal address")
		}
		var terminal coreTerminal
		if err := coreCall(cmd, &terminal, "get", entry.Address); err != nil {
			return err
		}
		if terminal.Address != entry.Address || terminal.Labels == nil {
			return fmt.Errorf("invalid core terminal details for %s", entry.Address)
		}
		terminals = append(terminals, terminal)
	}
	listed.Terminals = &terminals
	selected := map[string]bool{}
	spaces := map[string]bool{}
	protected := map[string]bool{os.Getenv("HC_TERMINAL_ID"): true}
	for _, t := range *listed.Terminals {
		if t.Labels["heft.root"] == root && t.Labels["heft.worktree"] == path {
			selected[t.Address] = true
			if s := t.Labels["herdr.space"]; s != "" {
				spaces[s] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, t := range *listed.Terminals {
			if !selected[t.Address] && (spaces[t.Labels["herdr.space"]] || selected[t.Parent]) {
				selected[t.Address] = true
				changed = true
			}
			if protected[t.Address] && t.Parent != "" && !protected[t.Parent] {
				protected[t.Parent] = true
				changed = true
			}
		}
	}
	for _, t := range *listed.Terminals {
		if selected[t.Address] && preserve {
			if t.Status != nil {
				if t.Status.State != "idle" && t.Status.State != "done" {
					return fmt.Errorf("core agent %s status is %s", t.Address, t.Status.State)
				}
			} else if t.Reported {
				return fmt.Errorf("core agent %s status missing", t.Address)
			}
		}
	}
	var deferred []string
	for _, t := range *listed.Terminals {
		if !selected[t.Address] || (selected[t.Parent] && !protected[t.Parent]) {
			continue
		}
		if protected[t.Address] {
			deferred = append(deferred, t.Address)
			continue
		}
		if err := coreCall(cmd, nil, "kill", t.Address); err != nil {
			return err
		}
	}
	if len(deferred) > 0 {
		previous := cmd.PostRunE
		cmd.PostRunE = func(cmd *cobra.Command, args []string) error {
			if previous != nil {
				if err := previous(cmd, args); err != nil {
					return err
				}
			}
			for _, address := range deferred {
				if err := coreCall(cmd, nil, "kill", address); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return nil
}
