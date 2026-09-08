package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

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
	root.AddCommand(newInitCommand(), newVersionCommand(version))

	return root
}

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize heft",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := exec.LookPath("herdr"); err != nil {
				return fmt.Errorf("herdr is not installed or not in PATH")
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "herdr is installed")
			return err
		},
	}
}

func newVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the heft version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "heft %s\n", version)
			return err
		},
	}
}
