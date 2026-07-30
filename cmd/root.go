package cmd

import (
	stderrors "errors"

	"github.com/spf13/cobra"

	"github.com/runmedev/owl/internal/version"
)

var errSilentExit = stderrors.New("silent exit")

func IsSilentExit(err error) bool {
	return stderrors.Is(err, errSilentExit)
}

func NewRootCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:           "owl",
		Short:         "Typed environment variable store",
		Version:       version.BaseVersionInfo(),
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.AddCommand(NewLocalCommands()...)
	cmd.AddCommand(NewDebugCommand())
	cmd.AddCommand(newVersionCommand())

	return &cmd
}
