package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// New creates the root heft command.
func New(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "heft",
		Short: "Manage Git worktrees and Herdr workspaces",
		Long: `Heft manages task-focused Git worktrees and matching Herdr workspaces.

Run heft configure in a Git repository inside Herdr, then use heft work for each task.
Commands can run from any worktree or its subdirectories.
Configuration is stored in .heft.yaml at the primary checkout root;
worktree-local configuration is ignored.`,
		Example: `  heft configure
  heft work feature/login
  heft list
  heft cleanup feature/login`,
		Args:          cobra.NoArgs,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			switch cmd.Name() {
			case "configure", "work", "list", "cleanup", "prune":
				if _, err := exec.LookPath("herdr"); err != nil {
					return fmt.Errorf("Herdr is not installed or not in PATH; install Herdr and run heft inside it")
				}
				if strings.TrimSpace(os.Getenv("HERDR_WORKSPACE_ID")) == "" {
					return fmt.Errorf("run this command inside a Herdr terminal")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.SetVersionTemplate("heft {{.Version}}\n")
	root.AddCommand(newConfigureCommand(), newWorkCommand(), newListCommand(), newCleanupCommand(), newPruneCommand(), newVersionCommand(version))

	return root
}
