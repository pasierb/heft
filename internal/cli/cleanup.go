package cli

import (
	"bytes"
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

			branch := args[0]
			if err := validateBranch(cmd, root, branch); err != nil {
				return err
			}
			path := worktreePath(root, cfg.WorktreesDir, branch)
			return removeWorktree(cmd, root, path)
		},
	}
}

func newPruneCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Remove all clean worktrees",
		Args:  cobra.NoArgs,
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
				if err := removeWorktree(cmd, root, path); err != nil {
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

func removeWorktree(cmd *cobra.Command, root, path string) error {
	if err := checkUncommittedChanges(cmd, path); err != nil {
		return err
	}
	if err := runGit(cmd, root, "worktree", "remove", path); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
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
