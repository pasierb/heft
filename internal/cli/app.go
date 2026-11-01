package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

type appResult struct {
	Space    string `json:"space"`
	Tab      string `json:"tab"`
	Terminal string `json:"terminal"`
	Error    string `json:"error"`
}

func appCall(cmd *cobra.Command, app, action string, args map[string]any) (appResult, error) {
	var result appResult
	data, err := json.Marshal(args)
	if err != nil {
		return result, err
	}
	process := exec.CommandContext(cmd.Context(), app, "do", action, string(data))
	process.Stderr = cmd.ErrOrStderr()
	out, runErr := process.Output()
	if err := json.Unmarshal(out, &result); err != nil {
		if runErr != nil {
			return result, fmt.Errorf("herdr-app %s: %w", action, runErr)
		}
		return result, fmt.Errorf("decode herdr-app %s: %w", action, err)
	}
	if result.Error != "" {
		return result, fmt.Errorf("herdr-app %s: %s", action, result.Error)
	}
	if runErr != nil {
		return result, fmt.Errorf("herdr-app %s: %w", action, runErr)
	}
	return result, nil
}

func appWork(cmd *cobra.Command, app, root, path, label string, tabs []tab, prompt string, noFocus bool) error {
	if noFocus {
		fmt.Fprintln(cmd.ErrOrStderr(), "Herdr app: --no-focus is not supported by the app's creation commands.")
	}
	if len(tabs) == 0 {
		tabs = []tab{{Name: "shell"}}
	}
	space, selected, agent := "", "", ""
	for i, tab := range tabs {
		action := "tab.new"
		args := map[string]any{"name": tab.Name, "space": space, "folder": path}
		if i == 0 {
			action = "space.new"
			args = map[string]any{"name": label, "tab": tab.Name, "folder": path}
		}
		if tab.Command != "" {
			// Pass the saved command to a shell intact, including quotes and operators.
			args["argv"] = []string{"sh", "-c", tab.Command}
		}
		created, err := appCall(cmd, app, action, args)
		if err != nil {
			return fmt.Errorf("create app tab %q (worktree and previous tabs retained): %w", tab.Name, err)
		}
		if created.Space == "" || created.Tab == "" || created.Terminal == "" {
			return fmt.Errorf("create app tab %q: response missing resource IDs (worktree and previous tabs retained)", tab.Name)
		}
		space = created.Space
		// Keep cleanup ownership on the core terminal; the app supplies layout labels.
		if err := coreCall(cmd, nil, "label", created.Terminal, "heft.root="+root, "heft.worktree="+path, "heft.workspace="+label, "heft.tab="+tab.Name); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Created app tab %s for %s\n", created.Tab, tab.Name)
		if selected == "" || (agent == "" && tab.Name == "shell") || tab.Agent {
			selected = created.Tab
		}
		if tab.Agent {
			agent = created.Terminal
		}
	}
	if _, err := appCall(cmd, app, "tab.select", map[string]any{"id": selected}); err != nil {
		return err
	}
	if prompt != "" && agent != "" {
		return corePrompt(cmd, agent, prompt)
	}
	return nil
}
