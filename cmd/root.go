package cmd

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:   "owl",
		Short: "Typed environment variable store",
	}

	cmd.AddCommand(NewLocalCommands()...)

	return &cmd
}
