package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize heft",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := exec.LookPath("herdr"); err != nil {
				return fmt.Errorf("herdr is not installed or not in PATH")
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "herdr is installed"); err != nil {
				return err
			}

			root, err := projectRoot()
			if err != nil {
				return err
			}
			configPath := filepath.Join(root, ".heft.yaml")
			if _, err := os.Stat(configPath); err == nil {
				cfg, err := readConfig(configPath)
				if err != nil {
					return err
				}
				return ensureWorktreesIgnored(root, cfg.worktreesDir)
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return configure(cmd, root)
		},
	}
}
