package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newWorkCommand() *cobra.Command {
	var prompt string
	var noFocus bool
	cmd := &cobra.Command{
		Use:   "work <branch>",
		Short: "Create a worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			cfg, err := readConfig(filepath.Join(root, ".heft.yaml"))
			if err != nil {
				return err
			}
			if prompt != "" && (len(cfg.Tabs) == 0 || cfg.Tabs[0].Command == "") {
				return fmt.Errorf("--prompt requires the first configured tab to start an agent")
			}
			if err := ensureWorktreesIgnored(root, cfg.WorktreesDir); err != nil {
				return err
			}

			branch := args[0]
			if err := validateBranch(cmd, root, branch); err != nil {
				return err
			}
			if err := runGit(cmd, root, "fetch", "origin", cfg.BaseBranch); err != nil {
				return fmt.Errorf("fetch base branch: %w", err)
			}
			path := worktreePath(root, cfg.WorktreesDir, branch)
			if err := runGit(cmd, root, "worktree", "add", "-b", branch, path, "FETCH_HEAD"); err != nil {
				return fmt.Errorf("create worktree: %w", err)
			}
			if len(cfg.Tabs) == 0 {
				args := []string{"workspace", "create", "--cwd", path, "--label", branch}
				if noFocus {
					args = append(args, "--no-focus")
				}
				return runHerdr(cmd, nil, "create herdr workspace", args...)
			}
			focus := "--focus"
			if noFocus {
				focus = "--no-focus"
			}
			created, err := createHerdr(cmd, "create herdr workspace", "workspace", "create", "--cwd", path, "--label", branch, focus)
			if err != nil {
				return err
			}
			if created.Result.Workspace.WorkspaceID == "" || created.Result.Tab.TabID == "" || created.Result.RootPane.PaneID == "" {
				return fmt.Errorf("create herdr workspace: response missing resource IDs")
			}
			if err := runHerdr(cmd, nil, "rename first herdr tab", "tab", "rename", created.Result.Tab.TabID, cfg.Tabs[0].Name); err != nil {
				return err
			}
			if cfg.Tabs[0].Command != "" {
				if err := runHerdr(cmd, nil, "run command in first herdr tab", "pane", "run", created.Result.RootPane.PaneID, cfg.Tabs[0].Command); err != nil {
					return err
				}
			}
			workspaceID := created.Result.Workspace.WorkspaceID
			agentPaneID := created.Result.RootPane.PaneID
			for _, tab := range cfg.Tabs[1:] {
				created, err := createHerdr(cmd, fmt.Sprintf("create herdr tab %q", tab.Name), "tab", "create", "--workspace", workspaceID, "--cwd", path, "--label", tab.Name, "--no-focus")
				if err != nil {
					return err
				}
				if created.Result.RootPane.PaneID == "" {
					return fmt.Errorf("create herdr tab %q: response missing root pane ID", tab.Name)
				}
				if tab.Command != "" {
					if err := runHerdr(cmd, nil, fmt.Sprintf("run command in herdr tab %q", tab.Name), "pane", "run", created.Result.RootPane.PaneID, tab.Command); err != nil {
						return err
					}
				}
			}
			if prompt != "" {
				return promptAgent(cmd, agentPaneID, prompt)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "", "prompt the agent in the first configured tab")
	cmd.Flags().BoolVar(&noFocus, "no-focus", false, "open the workspace without focusing it")
	return cmd
}

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List worktrees",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			return runGit(cmd, root, "worktree", "list")
		},
	}
}

type herdrCreation struct {
	Result struct {
		Workspace struct {
			WorkspaceID string `json:"workspace_id"`
		} `json:"workspace"`
		Tab struct {
			TabID string `json:"tab_id"`
		} `json:"tab"`
		RootPane struct {
			PaneID string `json:"pane_id"`
		} `json:"root_pane"`
	} `json:"result"`
}

func createHerdr(cmd *cobra.Command, action string, args ...string) (herdrCreation, error) {
	var created herdrCreation
	var out bytes.Buffer
	if err := runHerdr(cmd, &out, action, args...); err != nil {
		return created, err
	}
	if err := json.Unmarshal(out.Bytes(), &created); err != nil {
		return created, fmt.Errorf("%s: decode response: %w", action, err)
	}
	return created, nil
}

func runHerdr(cmd *cobra.Command, out *bytes.Buffer, action string, args ...string) error {
	herdr := exec.CommandContext(cmd.Context(), "herdr", args...)
	herdr.Stdin, herdr.Stderr = cmd.InOrStdin(), cmd.ErrOrStderr()
	if out == nil {
		herdr.Stdout = cmd.OutOrStdout()
	} else {
		herdr.Stdout = out
	}
	if err := herdr.Run(); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

func promptAgent(cmd *cobra.Command, paneID, prompt string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		probe := exec.CommandContext(cmd.Context(), "herdr", "agent", "get", paneID)
		if err := probe.Run(); err == nil {
			break
		} else if time.Now().After(deadline) {
			return fmt.Errorf("wait for agent in first herdr tab: %w", err)
		}
		select {
		case <-cmd.Context().Done():
			return fmt.Errorf("wait for agent in first herdr tab: %w", cmd.Context().Err())
		case <-time.After(100 * time.Millisecond):
		}
	}

	remaining := max(time.Until(deadline).Milliseconds(), 1)
	var out bytes.Buffer
	if err := runHerdr(cmd, &out, "wait for agent in first herdr tab", "agent", "wait", paneID, "--timeout", strconv.FormatInt(remaining, 10)); err != nil {
		return err
	}
	return runHerdr(cmd, &out, "prompt agent in first herdr tab", "agent", "prompt", paneID, prompt)
}
