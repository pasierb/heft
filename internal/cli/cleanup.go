package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newCleanupCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup <branch>",
		Short: "Remove a worktree",
		Long: `Close the matching Herdr workspace and remove a task worktree.

The worktree must have no staged, unstaged, or untracked changes. The local
branch is preserved.`,
		Example: "  heft cleanup feature/login",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			cfg, err := readConfig(filepath.Join(root, ".heft.yaml"))
			if err != nil {
				return err
			}

			branch := args[0]
			if err := validateBranch(cmd, root, branch); err != nil {
				return err
			}
			path := worktreePath(root, cfg.WorktreesDir, branch)
			return removeWorktree(cmd, root, path, false)
		},
	}
}

func newPruneCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Remove all clean worktrees",
		Long: `Remove every clean linked worktree and close its Herdr workspace.

The primary worktree, dirty worktrees, active agent workspaces, and local branches are preserved.
Skipped worktrees are reported on stderr.`,
		Example: "  heft prune",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			git := exec.CommandContext(cmd.Context(), "git", "-C", root, "worktree", "list", "--porcelain")
			git.Stderr = cmd.ErrOrStderr()
			out, err := git.Output()
			if err != nil {
				return fmt.Errorf("list worktrees: %w", err)
			}
			worktrees := parseWorktrees(out)
			for _, path := range worktrees[1:] { // Git lists the primary worktree first.
				if path == root {
					continue
				}
				if err := removeWorktree(cmd, root, path, true); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: %v\n", path, err)
				}
			}
			return nil
		},
	}
}

func parseWorktrees(out []byte) []string {
	var worktrees []string
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		if path, ok := strings.CutPrefix(string(line), "worktree "); ok {
			worktrees = append(worktrees, path)
		}
	}
	return worktrees
}

func removeWorktree(cmd *cobra.Command, root, path string, preserveActiveAgent bool) error {
	if err := checkUncommittedChanges(cmd, path); err != nil {
		return err
	}
	workspaceID, err := herdrWorkspaceForWorktree(cmd, root, path)
	if err != nil {
		return err
	}
	if workspaceID != "" {
		if preserveActiveAgent {
			if err := checkHerdrAgentsSettled(cmd, workspaceID); err != nil {
				return err
			}
		}
		if err := runHerdr(cmd, nil, "close herdr workspace", "workspace", "close", workspaceID); err != nil {
			return err
		}
	}
	if err := runGit(cmd, root, "worktree", "remove", path); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}

func checkHerdrAgentsSettled(cmd *cobra.Command, workspaceID string) error {
	var out bytes.Buffer
	if err := runHerdr(cmd, &out, "list herdr agents", "agent", "list"); err != nil {
		return err
	}
	var listed struct {
		Result struct {
			Agents *[]struct {
				WorkspaceID string `json:"workspace_id"`
				AgentStatus string `json:"agent_status"`
			} `json:"agents"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil {
		return fmt.Errorf("list herdr agents: decode response: %w", err)
	}
	if listed.Result.Agents == nil {
		return errors.New("list herdr agents: response missing agents")
	}
	for _, agent := range *listed.Result.Agents {
		if agent.WorkspaceID == workspaceID && agent.AgentStatus != "idle" && agent.AgentStatus != "done" {
			status := agent.AgentStatus
			if status == "" {
				status = "missing"
			}
			return fmt.Errorf("herdr agent status is %s", status)
		}
	}
	return nil
}

func herdrWorkspaceForWorktree(cmd *cobra.Command, root, path string) (string, error) {
	var out bytes.Buffer
	if err := runHerdr(cmd, &out, "list herdr worktrees", "worktree", "list", "--cwd", root); err != nil {
		return "", err
	}
	var listed struct {
		Result struct {
			Worktrees []struct {
				Path            string `json:"path"`
				OpenWorkspaceID string `json:"open_workspace_id"`
			} `json:"worktrees"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil {
		return "", fmt.Errorf("list herdr worktrees: decode response: %w", err)
	}
	for _, worktree := range listed.Result.Worktrees {
		if worktree.Path == path {
			return worktree.OpenWorkspaceID, nil
		}
	}
	return "", nil
}

func checkUncommittedChanges(cmd *cobra.Command, path string) error {
	git := exec.CommandContext(cmd.Context(), "git", "-C", path, "status", "--porcelain")
	git.Stderr = cmd.ErrOrStderr()
	out, err := git.Output()
	if err != nil {
		return fmt.Errorf("check uncommitted changes: %w", err)
	}
	if len(out) > 0 {
		return errors.New("worktree has uncommitted changes")
	}
	return nil
}
