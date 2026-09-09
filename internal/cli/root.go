package cli

import "github.com/spf13/cobra"

// New creates the root heft command.
func New(version string) *cobra.Command {
	root := &cobra.Command{
		Use:   "heft",
		Short: "Manage Git worktrees and Herdr workspaces",
		Long: `Heft manages task-focused Git worktrees and matching Herdr workspaces.

Run heft init once in a Git repository, then use heft work for each task.
Configuration is stored in .heft.yaml at the repository root.`,
		Example: `  heft init
  heft work feature/login
  heft list
  heft cleanup feature/login`,
		Args:          cobra.NoArgs,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.SetVersionTemplate("heft {{.Version}}\n")
	root.AddCommand(newInitCommand(), newConfigureCommand(), newWorkCommand(), newListCommand(), newCleanupCommand(), newPruneCommand(), newVersionCommand(version))

	return root
}
