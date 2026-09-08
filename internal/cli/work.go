package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newWorkCommand() *cobra.Command {
	return &cobra.Command{
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
			if err := ensureWorktreesIgnored(root, cfg.worktreesDir); err != nil {
				return err
			}

			branch := args[0]
			if err := validateBranch(cmd, root, branch); err != nil {
				return err
			}
			if err := runGit(cmd, root, "fetch", "origin", cfg.baseBranch); err != nil {
				return fmt.Errorf("fetch base branch: %w", err)
			}
			path := worktreePath(root, cfg.worktreesDir, branch)
			if err := runGit(cmd, root, "worktree", "add", "-b", branch, path, "FETCH_HEAD"); err != nil {
				return fmt.Errorf("create worktree: %w", err)
			}
			herdr := exec.CommandContext(cmd.Context(), "herdr", "workspace", "create", "--cwd", path, "--label", branch)
			herdr.Stdin, herdr.Stdout, herdr.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
			if err := herdr.Run(); err != nil {
				return fmt.Errorf("create herdr workspace: %w", err)
			}
			return nil
		},
	}
}
