package main

import (
	"io"

	"github.com/spf13/cobra"
)

// newInternalCmd builds the hidden gc internal subcommand tree used by
// infrastructure that has no public operator surface.
func newInternalCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "internal",
		Short:  "Internal gc subcommands (not for direct human use)",
		Hidden: true,
	}
	cmd.AddCommand(newInternalProjectMCPCmd(stdout, stderr))
	cmd.AddCommand(newInternalBeadsCredentialCmd(stdout, stderr))
	return cmd
}
