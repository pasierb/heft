package cli

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"

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
			path := worktreePath(root, cfg.worktreesDir, branch)
			checks := []func(*cobra.Command, string) error{checkUncommittedChanges}
			for _, check := range checks {
				if err := check(cmd, path); err != nil {
					return err
				}
			}
			if err := runGit(cmd, root, "worktree", "remove", path); err != nil {
				return fmt.Errorf("remove worktree: %w", err)
			}
			return nil
		},
	}
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
