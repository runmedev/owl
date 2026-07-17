package cmd

import (
	"github.com/spf13/cobra"

	"github.com/runmedev/owl/internal/version"
)

func NewRootCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:     "owl",
		Short:   "Typed environment variable store",
		Version: version.BaseVersionInfo(),
	}

	cmd.AddCommand(NewLocalCommands()...)
	cmd.AddCommand(newVersionCommand())

	return &cmd
}
