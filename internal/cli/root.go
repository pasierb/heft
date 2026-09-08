package cli

import "github.com/spf13/cobra"

// New creates the root heft command.
func New(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "heft",
		Short:         "Seamless worktree management for herdr users",
		Args:          cobra.NoArgs,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.SetVersionTemplate("heft {{.Version}}\n")
	root.AddCommand(newInitCommand(), newConfigureCommand(), newWorkCommand(), newCleanupCommand(), newVersionCommand(version))

	return root
}
