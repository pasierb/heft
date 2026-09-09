package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print the heft version",
		Long:    "Print the installed heft version.",
		Example: "  heft version",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "heft %s\n", version)
			return err
		},
	}
}
