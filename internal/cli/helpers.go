package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func projectRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("find project root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGit(cmd *cobra.Command, root string, args ...string) error {
	git := exec.CommandContext(cmd.Context(), "git", append([]string{"-C", root}, args...)...)
	git.Stdin, git.Stdout, git.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
	return git.Run()
}

func validateBranch(cmd *cobra.Command, root, branch string) error {
	if err := exec.CommandContext(cmd.Context(), "git", "-C", root, "check-ref-format", "--branch", branch).Run(); err != nil {
		return fmt.Errorf("invalid branch %q", branch)
	}
	return nil
}

func worktreePath(root, dir, branch string) string {
	return filepath.Join(root, dir, strings.ReplaceAll(branch, "/", "_"))
}
